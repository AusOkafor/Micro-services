package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"microservice/internal/adminaction"
	"microservice/internal/api"
	"microservice/internal/approval"
	"microservice/internal/audit"
	"microservice/internal/events"
	"microservice/internal/milestone"
	"microservice/pkg/db"
)

type Handlers struct {
	DB        *pgxpool.Pool
	Services  *Repository
	Milestones *milestone.Repository
	Approvals *approval.Repository
}

func (h Handlers) List(w http.ResponseWriter, r *http.Request) {
	s := api.ShopFromContext(r.Context())
	if s == nil {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing shop identity")
		return
	}

	items, err := h.Services.ListByShop(r.Context(), s.ID)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL", "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
}

func (h Handlers) Get(w http.ResponseWriter, r *http.Request) {
	s := api.ShopFromContext(r.Context())
	if s == nil {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing shop identity")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		api.WriteError(w, http.StatusBadRequest, "VALIDATION_FAILED", "missing id")
		return
	}

	svc, err := h.Services.GetByID(r.Context(), s.ID, id)
	if err != nil {
		api.WriteError(w, http.StatusNotFound, "NOT_FOUND", "service not found")
		return
	}

	ms, err := h.Milestones.ListByService(r.Context(), svc.ID)
	if err != nil {
		// Log the actual error for debugging
		fmt.Printf("[service/handlers] ListByService failed for service %s: %v\n", svc.ID, err)
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL", fmt.Sprintf("failed to load milestones: %v", err))
		return
	}
	// If no milestones, return empty array (this is valid for newly created services)
	if ms == nil {
		ms = []milestone.Record{}
	}

	var appr *approval.Record
	appr, _ = h.Approvals.GetByService(r.Context(), svc.ID) // optional until requested

	var portalToken any
	{
		const q = `
SELECT token, expires_at
FROM portal_tokens
WHERE service_id = $1 AND revoked_at IS NULL AND expires_at > NOW()
ORDER BY created_at DESC
LIMIT 1
`
		var tok string
		var exp time.Time
		if err := h.DB.QueryRow(r.Context(), q, svc.ID).Scan(&tok, &exp); err == nil {
			portalToken = map[string]any{"token": tok, "expiresAt": exp}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"service":    svc,
		"milestones": ms,
		"approval":   appr,
		"portal":     portalToken,
	})
}

type PatchStatusRequest struct {
	Status string `json:"status"`
}

func (h Handlers) PatchStatus(w http.ResponseWriter, r *http.Request) {
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

	var req PatchStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid json")
		return
	}

	next, err := ParseStatus(req.Status)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid status")
		return
	}

	err = db.WithTx(r.Context(), h.DB, func(tx pgx.Tx) error {
		svc, err := GetForUpdate(r.Context(), tx, shopCtx.ID, id)
		if err != nil {
			return err
		}

		// Completion law enforcement.
		if next == StatusCompleted && !svc.CompletedViaOverride {
			// Require final milestone paid.
			// We check by selecting the last milestone and ensuring it's paid.
			const qFinal = `
SELECT status
FROM milestones
WHERE service_id = $1
ORDER BY sequence DESC
LIMIT 1
`
			var st string
			if err := tx.QueryRow(r.Context(), qFinal, svc.ID).Scan(&st); err != nil {
				return err
			}
			if st != "paid" {
				api.WriteError(w, http.StatusConflict, "FINAL_MILESTONE_LOCKED", "final milestone is not paid")
				return pgx.ErrTxCommitRollback
			}
		}

		if !CanTransition(svc.Status, next) {
			api.WriteError(w, http.StatusConflict, "INVALID_STATE_TRANSITION", "invalid state transition")
			return pgx.ErrTxCommitRollback
		}

		// Side effect: requesting approval creates/updates approval row and records has_preview snapshot.
		if next == StatusWaitingForApproval {
			const qHasPreview = `SELECT EXISTS (SELECT 1 FROM files WHERE service_id = $1 AND kind = 'preview')`
			var hasPreview bool
			if err := tx.QueryRow(r.Context(), qHasPreview, svc.ID).Scan(&hasPreview); err != nil {
				return err
			}
			if err := approval.UpsertRequested(r.Context(), tx, svc.ID, hasPreview); err != nil {
				return err
			}
		}

		if err := UpdateStatus(r.Context(), tx, shopCtx.ID, svc.ID, next, svc.CompletedViaOverride); err != nil {
			return err
		}

		actor := "merchant"
		svcID := svc.ID
		_ = audit.Insert(r.Context(), tx, shopCtx.ID, &svcID, "STATUS_CHANGED", actor, map[string]any{"from": svc.Status, "to": next})
		_ = events.Insert(r.Context(), tx, svc.ID, "STATUS_CHANGED", "Status changed", actor, time.Now(), map[string]any{"from": svc.Status, "to": next})

		return nil
	})

	if err != nil {
		// If we used pgx.ErrTxCommitRollback to early-return after writing response, ignore.
		if err == pgx.ErrTxCommitRollback {
			return
		}
		api.WriteError(w, http.StatusNotFound, "NOT_FOUND", "service not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type AdminOverrideRequest struct {
	ActionType  string `json:"actionType"`
	Reason      string `json:"reason"`
	MilestoneID string `json:"milestoneId,omitempty"`
}

func (h Handlers) AdminOverride(w http.ResponseWriter, r *http.Request) {
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

	var req AdminOverrideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid json")
		return
	}
	if req.Reason == "" {
		api.WriteError(w, http.StatusBadRequest, "OVERRIDE_REASON_REQUIRED", "reason is required")
		return
	}

	action := adminaction.ActionType(req.ActionType)
	switch action {
	case adminaction.ActionMarkMilestonePaid, adminaction.ActionCompleteServiceWithoutFinalPay, adminaction.ActionReopenService:
	default:
		api.WriteError(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid actionType")
		return
	}

	err := db.WithTx(r.Context(), h.DB, func(tx pgx.Tx) error {
		svc, err := GetForUpdate(r.Context(), tx, shopCtx.ID, id)
		if err != nil {
			return err
		}

		now := time.Now()
		actor := "merchant"
		svcID := svc.ID

		switch action {
		case adminaction.ActionMarkMilestonePaid:
			if req.MilestoneID == "" {
				api.WriteError(w, http.StatusBadRequest, "VALIDATION_FAILED", "milestoneId is required for MARK_MILESTONE_PAID")
				return pgx.ErrTxCommitRollback
			}
			m, err := milestone.GetForUpdate(r.Context(), tx, req.MilestoneID)
			if err != nil || m.ServiceID != svc.ID {
				api.WriteError(w, http.StatusNotFound, "NOT_FOUND", "milestone not found")
				return pgx.ErrTxCommitRollback
			}
			if m.Status == "paid" {
				api.WriteError(w, http.StatusConflict, "MILESTONE_ALREADY_PAID", "milestone already paid")
				return pgx.ErrTxCommitRollback
			}
			if err := milestone.MarkPaid(r.Context(), tx, m.ID, now); err != nil {
				return err
			}

		case adminaction.ActionCompleteServiceWithoutFinalPay:
			if err := UpdateStatus(r.Context(), tx, shopCtx.ID, svc.ID, StatusCompleted, true); err != nil {
				return err
			}

		case adminaction.ActionReopenService:
			// Reopen means go back to InProgress and clear override flag.
			if err := UpdateStatus(r.Context(), tx, shopCtx.ID, svc.ID, StatusInProgress, false); err != nil {
				return err
			}
		}

		_ = adminaction.Insert(r.Context(), tx, svc.ID, action, req.Reason, actor, map[string]any{"milestoneId": req.MilestoneID})
		_ = audit.Insert(r.Context(), tx, shopCtx.ID, &svcID, "ADMIN_OVERRIDE", actor, map[string]any{"actionType": action, "reason": req.Reason, "milestoneId": req.MilestoneID})
		_ = events.Insert(r.Context(), tx, svc.ID, "ADMIN_OVERRIDE", "Admin override applied", actor, now, map[string]any{"actionType": action, "reason": req.Reason, "milestoneId": req.MilestoneID})

		return nil
	})

	if err != nil {
		if err == pgx.ErrTxCommitRollback {
			return
		}
		api.WriteError(w, http.StatusNotFound, "NOT_FOUND", "service not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) Events(w http.ResponseWriter, r *http.Request) {
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

	// Ensure service belongs to the shop.
	if _, err := h.Services.GetByID(r.Context(), shopCtx.ID, id); err != nil {
		api.WriteError(w, http.StatusNotFound, "NOT_FOUND", "service not found")
		return
	}

	evs, err := events.ListByService(r.Context(), h.DB, id)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL", "internal error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": evs})
}


