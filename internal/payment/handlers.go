package payment

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"microservice/internal/api"
	"microservice/internal/audit"
	"microservice/internal/events"
	"microservice/internal/milestone"
	"microservice/pkg/config"
	"microservice/pkg/db"
	"microservice/pkg/shopify"
)

type Handlers struct {
	Cfg        config.Config
	DB         *pgxpool.Pool
	Milestones *milestone.Repository
}

func (h Handlers) RequestPayment(w http.ResponseWriter, r *http.Request) {
	shopCtx := api.ShopFromContext(r.Context())
	if shopCtx == nil {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing shop identity")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		api.WriteError(w, http.StatusBadRequest, "VALIDATION_FAILED", "missing id")
		return
	}

	var resp any
	err := db.WithTx(r.Context(), h.DB, func(tx pgx.Tx) error {
		m, err := milestone.GetForUpdateScoped(r.Context(), tx, shopCtx.ID, id)
		if err != nil {
			return err
		}

		if m.Status == "paid" {
			api.WriteError(w, http.StatusConflict, "MILESTONE_ALREADY_PAID", "milestone already paid")
			return pgx.ErrTxCommitRollback
		}
		if m.Status == "locked" {
			api.WriteError(w, http.StatusConflict, "MILESTONE_LOCKED", "milestone is locked")
			return pgx.ErrTxCommitRollback
		}

		// Idempotency: if already has a draft order, return existing link.
		if m.DraftOrderID != "" && m.CheckoutURL != "" {
			resp = map[string]any{"draftOrderId": m.DraftOrderID, "checkoutUrl": m.CheckoutURL}
			return nil
		}

		// Block final milestone until approval (strict).
		const qFinalSeq = `SELECT sequence FROM milestones WHERE service_id = $1 ORDER BY sequence DESC LIMIT 1`
		var finalSeq int
		if err := tx.QueryRow(r.Context(), qFinalSeq, m.ServiceID).Scan(&finalSeq); err == nil && m.Sequence == finalSeq {
			const qAppr = `SELECT approved FROM approvals WHERE service_id = $1`
			var approved bool
			if err := tx.QueryRow(r.Context(), qAppr, m.ServiceID).Scan(&approved); err != nil || !approved {
				api.WriteError(w, http.StatusConflict, "FINAL_MILESTONE_LOCKED", "final payment requires approval")
				return pgx.ErrTxCommitRollback
			}
		}

		client := shopify.Client{
			ShopDomain:  shopCtx.Domain,
			AccessToken: shopCtx.AccessToken,
			APIVersion:  h.Cfg.Shopify.APIVersion,
		}
		// Dev convenience: allow using a Shopify "Develop app" Admin API token (shpat_...) to bypass
		// Protected Customer Data restrictions that apply to public apps.
		if h.Cfg.AppEnv != "prod" && strings.TrimSpace(h.Cfg.Shopify.DevAdminAccessToken) != "" {
			client.AccessToken = strings.TrimSpace(h.Cfg.Shopify.DevAdminAccessToken)
		}

		title := fmt.Sprintf("Milestone payment (service %s, seq %d)", m.ServiceID, m.Sequence)
		// Note is used later to resolve paid orders back to a milestone via the orders/paid webhook.
		note := fmt.Sprintf("service_workflow: milestone_id=%s service_id=%s", m.ID, m.ServiceID)
		currency := m.Currency
		if currency == "" {
			currency = "USD"
		}
		draftOrderID, checkoutURL, err := client.CreateDraftOrder(r.Context(), title, m.Amount, currency, note)
		if err != nil {
			return err
		}

		if err := milestone.SetDraftOrder(r.Context(), tx, m.ID, draftOrderID, checkoutURL); err != nil {
			return err
		}

		now := time.Now()
		actor := "merchant"
		_ = audit.Insert(r.Context(), tx, shopCtx.ID, &m.ServiceID, "MILESTONE_PAYMENT_REQUESTED", actor, map[string]any{"milestoneId": m.ID, "draftOrderId": draftOrderID})
		_ = events.Insert(r.Context(), tx, m.ServiceID, "MILESTONE_PAYMENT_REQUESTED", "Milestone payment requested", actor, now, map[string]any{"milestoneId": m.ID, "draftOrderId": draftOrderID})

		resp = map[string]any{"draftOrderId": draftOrderID, "checkoutUrl": checkoutURL}
		return nil
	})

	if err != nil {
		if err == pgx.ErrTxCommitRollback {
			return
		}
		if h.Cfg.AppEnv != "prod" {
			api.WriteError(w, http.StatusInternalServerError, "INTERNAL", fmt.Sprintf("request payment failed: %v", err))
			return
		}
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL", "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
