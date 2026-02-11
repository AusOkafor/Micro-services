package webhook

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"context"
	"errors"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"microservice/internal/api"
	"microservice/internal/audit"
	"microservice/internal/events"
	"microservice/internal/milestone"
	"microservice/internal/portal"
	"microservice/internal/service"
	"microservice/internal/serviceproduct"
	"microservice/internal/shop"
	"microservice/pkg/config"
	"microservice/pkg/db"
)

type Handler struct {
	Cfg config.Config
	DB  *pgxpool.Pool

	Shops           *shop.Repository
	ServiceProducts *serviceproduct.Repository
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Prefer Shopify's topic header; fall back to route param.
	topic := strings.TrimSpace(r.Header.Get("X-Shopify-Topic"))
	if topic == "" {
		topic = chi.URLParam(r, "topic")
	}
	topic = NormalizeTopic(topic)

	shopDomain := strings.TrimSpace(r.Header.Get("X-Shopify-Shop-Domain"))
	hmacHeader := strings.TrimSpace(r.Header.Get("X-Shopify-Hmac-Sha256"))
	eventID := strings.TrimSpace(r.Header.Get("X-Shopify-Webhook-Id"))
	if eventID == "" {
		eventID = strings.TrimSpace(r.Header.Get("X-Shopify-Event-Id"))
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid body")
		return
	}

	if !VerifyShopifyWebhook(body, hmacHeader, h.Cfg.Shopify.WebhookSecret) {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid webhook signature")
		return
	}

	shopRec, err := h.Shops.FindByDomain(r.Context(), shopDomain)
	if err != nil {
		// Always return 200 to Shopify for unknown shops (avoid retries); log internally later.
		w.WriteHeader(http.StatusOK)
		return
	}

	payloadHash := sha256Hex(body)
	if eventID == "" {
		// Fallback idempotency key when webhook-id header isn't present.
		eventID = payloadHash
	}

	// Idempotency gate + handler execution in one tx.
	if err := db.WithTx(r.Context(), h.DB, func(tx pgx.Tx) error {
		if err := insertWebhookEvent(r.Context(), tx, shopRec.ID, topic, eventID, payloadHash); err != nil {
			if isUniqueViolation(err) {
				// Already processed.
				if h.Cfg.AppEnv != "prod" {
					log.Printf("webhook already processed shop=%s topic=%s event_id=%s", shopRec.Domain, topic, eventID)
				}
				return nil
			}
			return err
		}

		switch topic {
		case "orders_paid":
			return h.handleOrdersPaid(r.Context(), tx, shopRec, body)
		case "milestone_paid":
			return h.handleMilestonePaid(r.Context(), tx, shopRec, body)
		case "app_uninstalled":
			// Delete shop row; FK cascades remove related data.
			return h.Shops.DeleteByID(r.Context(), shopRec.ID)
		default:
			// Unknown topic: accept (no retries).
			return nil
		}
	}); err != nil && h.Cfg.AppEnv != "prod" {
		log.Printf("webhook tx error shop=%s topic=%s event_id=%s err=%v", shopRec.Domain, topic, eventID, err)
	}

	// Shopify expects a 200 quickly.
	w.WriteHeader(http.StatusOK)
}

func (h Handler) handleOrdersPaid(ctx context.Context, tx pgx.Tx, shopRec *shop.Shop, body []byte) error {
	var payload orderPaidPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}
	if payload.ID == 0 || payload.TotalPrice == "" || len(payload.LineItems) == 0 {
		return nil
	}

	// Milestone payment orders: if the order note contains a milestone_id, mark that milestone paid.
	if milestoneID := ParseKeyFromNote(payload.Note, "milestone_id"); milestoneID != "" {
		return h.applyMilestonePaymentFromOrder(ctx, tx, shopRec, milestoneID, payload.ID)
	}

	// Find first line item that has a configured service product.
	var chosenProductID string
	var cfgRaw json.RawMessage
	for _, li := range payload.LineItems {
		if li.ProductID == 0 {
			continue
		}
		productID := int64ToString(li.ProductID)
		rec, err := getServiceProductConfig(ctx, tx, shopRec.ID, productID)
		if err == nil && len(rec) > 0 {
			chosenProductID = productID
			cfgRaw = rec
			break
		}
	}
	if chosenProductID == "" {
		if h.Cfg.AppEnv != "prod" {
			log.Printf("orders_paid: no service_product_config found shop=%s order_id=%d", shopRec.Domain, payload.ID)
		}
		return nil
	}

	cfg, err := serviceproduct.ParseAndValidate(cfgRaw)
	if err != nil {
		if h.Cfg.AppEnv != "prod" {
			log.Printf("orders_paid: invalid service_product_config shop=%s product_id=%s err=%v", shopRec.Domain, chosenProductID, err)
		}
		return nil
	}

	total, err := decimal.NewFromString(payload.TotalPrice)
	if err != nil {
		return nil
	}

	amounts, err := milestone.CalculateAmounts(total, cfg.Templates, milestone.DefaultCurrencyScale)
	if err != nil {
		if h.Cfg.AppEnv != "prod" {
			log.Printf("orders_paid: milestone calc failed shop=%s order_id=%d err=%v", shopRec.Domain, payload.ID, err)
		}
		return nil
	}

	// Create service (idempotent by UNIQUE(shop_id, shopify_order_id)).
	serviceID, created, err := insertService(ctx, tx, shopRec.ID, payload.ID, chosenProductID, payload.Email, payload.CustomerName(), total, payload.Currency, cfgRaw)
	if err != nil {
		if isUniqueViolation(err) {
			return nil
		}
		return err
	}

	now := time.Now()
	actor := "webhook"

	if created {
		if err := audit.Insert(ctx, tx, shopRec.ID, &serviceID, "SERVICE_CREATED", actor, map[string]any{"shopifyOrderId": payload.ID, "productId": chosenProductID}); err != nil {
			return err
		}
		if err := events.Insert(ctx, tx, serviceID, "SERVICE_CREATED", "Service created", actor, now, map[string]any{"shopifyOrderId": payload.ID}); err != nil {
			return err
		}

		// Create a portal token for the client (shareable link). Keep short-ish expiry for safety.
		if _, err := portal.InsertToken(ctx, tx, serviceID, now.Add(30*24*time.Hour)); err != nil {
			return err
		}
		if err := events.Insert(ctx, tx, serviceID, "PORTAL_TOKEN_CREATED", "Client portal link created", actor, now, map[string]any{}); err != nil {
			return err
		}
	}

	// Create milestones if none exist yet (idempotent by UNIQUE(service_id, sequence)).
	for i, m := range amounts {
		status := "unpaid"
		var paidAt *time.Time
		if i == 0 {
			status = "paid"
			paidAt = &now
		} else if m.IsFinal {
			status = "locked"
		}

		if err := insertMilestone(ctx, tx, serviceID, i, m.Amount, status, paidAt); err != nil {
			if isUniqueViolation(err) {
				continue
			}
			return err
		}

		if i == 0 {
			if err := audit.Insert(ctx, tx, shopRec.ID, &serviceID, "DEPOSIT_PAID", actor, map[string]any{"sequence": 0, "amount": m.Amount.StringFixed(2)}); err != nil {
				return err
			}
			if err := events.Insert(ctx, tx, serviceID, "MILESTONE_PAID", "Deposit paid", actor, now, map[string]any{"sequence": 0}); err != nil {
				return err
			}
		}
	}

	// Ensure service status is Booked (only if newly created).
	if created {
		if err := service.UpdateStatus(ctx, tx, shopRec.ID, serviceID, service.StatusBooked, false); err != nil {
			return err
		}
	}

	return nil
}

func (h Handler) applyMilestonePaymentFromOrder(ctx context.Context, tx pgx.Tx, shopRec *shop.Shop, milestoneID string, orderID int64) error {
	// Shop-scope + row-lock the milestone.
	m, err := milestone.GetForUpdateScoped(ctx, tx, shopRec.ID, milestoneID)
	if err != nil {
		return nil
	}
	if m.Status == "paid" {
		return nil
	}
	if m.Status == "locked" {
		// Final milestone cannot be paid before approval; ignore.
		return nil
	}

	now := time.Now()
	if err := milestone.MarkPaid(ctx, tx, m.ID, now); err != nil {
		return err
	}

	actor := "webhook"
	serviceID := m.ServiceID
	if err := audit.Insert(ctx, tx, shopRec.ID, &serviceID, "MILESTONE_PAID", actor, map[string]any{"milestoneId": m.ID, "orderId": int64ToString(orderID)}); err != nil {
		return err
	}
	if err := events.Insert(ctx, tx, serviceID, "MILESTONE_PAID", "Milestone paid", actor, now, map[string]any{"milestoneId": m.ID}); err != nil {
		return err
	}

	// If this is the final milestone, complete only if approved.
	const qFinalSeq = `SELECT sequence FROM milestones WHERE service_id = $1 ORDER BY sequence DESC LIMIT 1`
	var finalSeq int
	if err := tx.QueryRow(ctx, qFinalSeq, serviceID).Scan(&finalSeq); err == nil && m.Sequence == finalSeq {
		const qAppr = `SELECT approved FROM approvals WHERE service_id = $1`
		var approved bool
		if err := tx.QueryRow(ctx, qAppr, serviceID).Scan(&approved); err == nil && approved {
			if err := service.UpdateStatus(ctx, tx, shopRec.ID, serviceID, service.StatusCompleted, false); err != nil {
				return err
			}
			if err := events.Insert(ctx, tx, serviceID, "STATUS_CHANGED", "Service completed", actor, now, map[string]any{"to": service.StatusCompleted}); err != nil {
				return err
			}
		}
	}

	return nil
}

func (h Handler) handleMilestonePaid(ctx context.Context, tx pgx.Tx, shopRec *shop.Shop, body []byte) error {
	var payload milestonePaidPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}
	if payload.DraftOrderID == "" {
		return nil
	}

	// Resolve milestone by draft_order_id, shop-scoped.
	const q = `
SELECT m.id, m.service_id
FROM milestones m
JOIN services s ON s.id = m.service_id
WHERE s.shop_id = $1 AND m.draft_order_id = $2
LIMIT 1
`
	var milestoneID, serviceID string
	if err := tx.QueryRow(ctx, q, shopRec.ID, payload.DraftOrderID).Scan(&milestoneID, &serviceID); err != nil {
		return nil
	}

	m, err := milestone.GetForUpdate(ctx, tx, milestoneID)
	if err != nil {
		return nil
	}
	if m.Status == "paid" {
		return nil
	}

	now := time.Now()
	if err := milestone.MarkPaid(ctx, tx, m.ID, now); err != nil {
		return err
	}

	actor := "webhook"
	if err := audit.Insert(ctx, tx, shopRec.ID, &serviceID, "MILESTONE_PAID", actor, map[string]any{"sequence": m.Sequence, "draftOrderId": payload.DraftOrderID}); err != nil {
		return err
	}
	if err := events.Insert(ctx, tx, serviceID, "MILESTONE_PAID", "Milestone paid", actor, now, map[string]any{"sequence": m.Sequence}); err != nil {
		return err
	}

	// If this is the final milestone, attempt completion if approved; otherwise just record paid.
	const qFinalSeq = `SELECT sequence FROM milestones WHERE service_id = $1 ORDER BY sequence DESC LIMIT 1`
	var finalSeq int
	if err := tx.QueryRow(ctx, qFinalSeq, serviceID).Scan(&finalSeq); err == nil && m.Sequence == finalSeq {
		// Check approval
		const qAppr = `SELECT approved FROM approvals WHERE service_id = $1`
		var approved bool
		if err := tx.QueryRow(ctx, qAppr, serviceID).Scan(&approved); err == nil && approved {
			if err := service.UpdateStatus(ctx, tx, shopRec.ID, serviceID, service.StatusCompleted, false); err != nil {
				return err
			}
			if err := events.Insert(ctx, tx, serviceID, "STATUS_CHANGED", "Service completed", actor, now, map[string]any{"to": service.StatusCompleted}); err != nil {
				return err
			}
		}
	}

	return nil
}

func insertWebhookEvent(ctx context.Context, tx pgx.Tx, shopID, topic, eventID, payloadHash string) error {
	const q = `
INSERT INTO webhook_events (shop_id, topic, event_id, payload_hash, processed_at)
VALUES ($1, $2, $3, $4, NOW())
`
	_, err := tx.Exec(ctx, q, shopID, topic, eventID, payloadHash)
	return err
}

func getServiceProductConfig(ctx context.Context, tx pgx.Tx, shopID, productID string) (json.RawMessage, error) {
	const q = `
SELECT config
FROM service_product_configs
WHERE shop_id = $1 AND shopify_product_id = $2
`
	var cfg json.RawMessage
	err := tx.QueryRow(ctx, q, shopID, productID).Scan(&cfg)
	return cfg, err
}

func insertService(ctx context.Context, tx pgx.Tx, shopID string, shopifyOrderID int64, shopifyProductID string, email string, name string, total decimal.Decimal, currency string, snapshot json.RawMessage) (string, bool, error) {
	const q = `
INSERT INTO services (shop_id, shopify_order_id, shopify_product_id, client_email, client_name, total_amount, currency, status, service_config_snapshot)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id
`
	var id string
	err := tx.QueryRow(ctx, q, shopID, int64ToString(shopifyOrderID), shopifyProductID, email, name, total.StringFixed(2), currencyOrDefault(currency), string(service.StatusBooked), snapshot).Scan(&id)
	if err != nil {
		return "", false, err
	}
	return id, true, nil
}

func insertMilestone(ctx context.Context, tx pgx.Tx, serviceID string, seq int, amount decimal.Decimal, status string, paidAt *time.Time) error {
	const q = `
INSERT INTO milestones (service_id, sequence, amount, status, paid_at)
VALUES ($1, $2, $3, $4, $5)
`
	_, err := tx.Exec(ctx, q, serviceID, seq, amount.StringFixed(2), status, paidAt)
	return err
}

func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func currencyOrDefault(c string) string {
	c = strings.TrimSpace(c)
	if c == "" {
		return "USD"
	}
	return c
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if ok := errors.As(err, &pgErr); ok {
		return pgErr.Code == "23505"
	}
	return false
}

type orderPaidPayload struct {
	ID         int64  `json:"id"`
	Email      string `json:"email"`
	TotalPrice string `json:"total_price"`
	Currency   string `json:"currency"`
	Note       string `json:"note"`
	Customer   struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	} `json:"customer"`
	LineItems []struct {
		ProductID int64 `json:"product_id"`
	} `json:"line_items"`
}

func (o orderPaidPayload) CustomerName() string {
	name := strings.TrimSpace(o.Customer.FirstName + " " + o.Customer.LastName)
	return strings.TrimSpace(name)
}

type milestonePaidPayload struct {
	DraftOrderID string `json:"draft_order_id"`
}

func int64ToString(v int64) string {
	// IDs can exceed 32-bit; represent safely as base-10 string.
	return strconv.FormatInt(v, 10)
}
