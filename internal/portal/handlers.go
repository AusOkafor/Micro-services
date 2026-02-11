package portal

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"microservice/internal/api"
	"microservice/internal/approval"
	"microservice/internal/audit"
	"microservice/internal/events"
	"microservice/internal/milestone"
	"microservice/internal/service"
	"microservice/pkg/db"
	"microservice/pkg/config"
)

type Handlers struct {
	DB         *pgxpool.Pool
	Milestones *milestone.Repository
	Cfg        config.Config
}

func (h Handlers) View(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		api.WriteError(w, http.StatusBadRequest, "VALIDATION_FAILED", "missing token")
		return
	}

	now := time.Now()

	// Read-only view (no need for FOR UPDATE).
	const qSvc = `
SELECT s.id, s.display_id, s.shop_id, sh.shop_domain, s.shopify_order_id, s.shopify_product_id,
       COALESCE(s.client_email,''), COALESCE(s.client_name,''),
       s.total_amount::text, s.currency, s.status, s.service_config_snapshot, s.completed_via_override,
       s.created_at, s.updated_at
FROM portal_tokens t
JOIN services s ON s.id = t.service_id
JOIN shops sh ON sh.id = s.shop_id
WHERE t.token = $1 AND t.revoked_at IS NULL AND t.expires_at > $2
`
	var svc service.Service
	var shopDomain string
	if err := h.DB.QueryRow(r.Context(), qSvc, token, now).Scan(
		&svc.ID, &svc.DisplayID, &svc.ShopID, &shopDomain, &svc.ShopifyOrderID, &svc.ShopifyProductID,
		&svc.ClientEmail, &svc.ClientName,
		&svc.TotalAmount, &svc.Currency, &svc.Status, &svc.ServiceConfigSnapshot, &svc.CompletedViaOverride,
		&svc.CreatedAt, &svc.UpdatedAt,
	); err != nil {
		api.WriteError(w, http.StatusNotFound, "NOT_FOUND", "portal link not found")
		return
	}

	ms, err := h.Milestones.ListByService(r.Context(), svc.ID)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL", "internal error")
		return
	}

	var appr any
	const qAppr = `
SELECT approved, revision_requested, COALESCE(client_note,''), has_preview,
       CASE WHEN approved_at IS NULL THEN NULL ELSE approved_at::text END
FROM approvals
WHERE service_id = $1
`
	var approved bool
	var revisionRequested bool
	var note string
	var hasPreview bool
	var approvedAt *string
	if err := h.DB.QueryRow(r.Context(), qAppr, svc.ID).Scan(&approved, &revisionRequested, &note, &hasPreview, &approvedAt); err == nil {
		appr = map[string]any{
			"approved":          approved,
			"revisionRequested": revisionRequested,
			"clientNote":        note,
			"hasPreview":        hasPreview,
			"approvedAt":        approvedAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"service":    svc,
		"milestones": ms,
		"approval":   appr,
		"merchant": map[string]any{
			"name":         shopDomain,
			"supportEmail": h.Cfg.PortalSupportEmail,
			"logoUrl":      h.Cfg.PortalLogoURL,
		},
	})
}

type ClientActionRequest struct {
	Note string `json:"note"`
}

func (h Handlers) Approve(w http.ResponseWriter, r *http.Request) {
	h.clientAction(w, r, true)
}

func (h Handlers) RequestRevision(w http.ResponseWriter, r *http.Request) {
	h.clientAction(w, r, false)
}

func (h Handlers) Events(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		api.WriteError(w, http.StatusBadRequest, "VALIDATION_FAILED", "missing token")
		return
	}

	now := time.Now()
	const qTok = `
SELECT s.id
FROM portal_tokens t
JOIN services s ON s.id = t.service_id
WHERE t.token = $1 AND t.revoked_at IS NULL AND t.expires_at > $2
`
	var serviceID string
	if err := h.DB.QueryRow(r.Context(), qTok, token, now).Scan(&serviceID); err != nil {
		api.WriteError(w, http.StatusNotFound, "NOT_FOUND", "portal link not found")
		return
	}

	items, err := events.ListByService(r.Context(), h.DB, serviceID)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL", "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
}

func (h Handlers) clientAction(w http.ResponseWriter, r *http.Request, approve bool) {
	token := chi.URLParam(r, "token")
	if token == "" {
		api.WriteError(w, http.StatusBadRequest, "VALIDATION_FAILED", "missing token")
		return
	}

	var req ClientActionRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	now := time.Now()

	err := db.WithTx(r.Context(), h.DB, func(tx pgx.Tx) error {
		tr, err := GetActiveByTokenForUpdate(r.Context(), tx, token, now)
		if err != nil {
			api.WriteError(w, http.StatusNotFound, "NOT_FOUND", "portal link not found")
			return pgx.ErrTxCommitRollback
		}

		// Lock service row (portal is not shop-scoped).
		svc, err := service.GetForUpdateAny(r.Context(), tx, tr.ServiceID)
		if err != nil {
			api.WriteError(w, http.StatusNotFound, "NOT_FOUND", "service not found")
			return pgx.ErrTxCommitRollback
		}

		if svc.Status != service.StatusWaitingForApproval {
			api.WriteError(w, http.StatusConflict, "INVALID_STATE_TRANSITION", "service is not waiting for approval")
			return pgx.ErrTxCommitRollback
		}

		actor := "client"
		svcID := svc.ID

		if approve {
			if err := approval.Approve(r.Context(), tx, svc.ID, req.Note); err != nil {
				return err
			}
			// Unlock final milestone (locked -> unpaid).
			if err := milestone.UnlockFinal(r.Context(), tx, svc.ID); err != nil {
				return err
			}

			_ = audit.Insert(r.Context(), tx, svc.ShopID, &svcID, "APPROVED", actor, map[string]any{"note": req.Note})
			_ = events.Insert(r.Context(), tx, svc.ID, "APPROVED", "Client approved", actor, now, map[string]any{})
		} else {
			if err := approval.RequestRevision(r.Context(), tx, svc.ID, req.Note); err != nil {
				return err
			}
			// Bounce service back to InProgress.
			if err := service.UpdateStatus(r.Context(), tx, svc.ShopID, svc.ID, service.StatusInProgress, false); err != nil {
				return err
			}

			_ = audit.Insert(r.Context(), tx, svc.ShopID, &svcID, "REVISION_REQUESTED", actor, map[string]any{"note": req.Note})
			_ = events.Insert(r.Context(), tx, svc.ID, "REVISION_REQUESTED", "Client requested revision", actor, now, map[string]any{})
		}

		_ = tr // kept locked to prevent double-action races
		return nil
	})
	if err == pgx.ErrTxCommitRollback {
		return
	}
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL", "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}


