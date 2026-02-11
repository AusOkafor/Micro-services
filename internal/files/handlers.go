package files

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"microservice/internal/api"
)

type MerchantHandlers struct {
	DB   *pgxpool.Pool
	Repo *Repository
}

type PortalHandlers struct {
	DB   *pgxpool.Pool
	Repo *Repository
}

type CreateRequest struct {
	FileURL string `json:"fileUrl"`
	Kind    string `json:"kind"` // preview | deliverable | other
}

func normalizeKind(k string) string {
	k = strings.TrimSpace(strings.ToLower(k))
	switch k {
	case "preview", "deliverable", "other":
		return k
	default:
		return "preview"
	}
}

func (h MerchantHandlers) Create(w http.ResponseWriter, r *http.Request) {
	s := api.ShopFromContext(r.Context())
	if s == nil {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing shop identity")
		return
	}

	serviceID := chi.URLParam(r, "id")
	if serviceID == "" {
		api.WriteError(w, http.StatusBadRequest, "VALIDATION_FAILED", "missing id")
		return
	}

	// Ensure service belongs to shop.
	const qSvc = `SELECT 1 FROM services WHERE id=$1 AND shop_id=$2`
	var one int
	if err := h.DB.QueryRow(r.Context(), qSvc, serviceID, s.ID).Scan(&one); err != nil {
		api.WriteError(w, http.StatusNotFound, "NOT_FOUND", "service not found")
		return
	}

	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid json")
		return
	}
	if strings.TrimSpace(req.FileURL) == "" {
		api.WriteError(w, http.StatusBadRequest, "VALIDATION_FAILED", "fileUrl is required")
		return
	}

	rec, err := h.Repo.Insert(r.Context(), serviceID, "merchant", strings.TrimSpace(req.FileURL), normalizeKind(req.Kind))
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL", "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rec)
}

func (h MerchantHandlers) List(w http.ResponseWriter, r *http.Request) {
	s := api.ShopFromContext(r.Context())
	if s == nil {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing shop identity")
		return
	}

	serviceID := chi.URLParam(r, "id")
	if serviceID == "" {
		api.WriteError(w, http.StatusBadRequest, "VALIDATION_FAILED", "missing id")
		return
	}

	// Ensure service belongs to shop.
	const qSvc = `SELECT 1 FROM services WHERE id=$1 AND shop_id=$2`
	var one int
	if err := h.DB.QueryRow(r.Context(), qSvc, serviceID, s.ID).Scan(&one); err != nil {
		api.WriteError(w, http.StatusNotFound, "NOT_FOUND", "service not found")
		return
	}

	items, err := h.Repo.ListByService(r.Context(), serviceID)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL", "internal error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
}

func (h PortalHandlers) Create(w http.ResponseWriter, r *http.Request) {
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

	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid json")
		return
	}
	if strings.TrimSpace(req.FileURL) == "" {
		api.WriteError(w, http.StatusBadRequest, "VALIDATION_FAILED", "fileUrl is required")
		return
	}

	rec, err := h.Repo.Insert(r.Context(), serviceID, "client", strings.TrimSpace(req.FileURL), normalizeKind(req.Kind))
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL", "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rec)
}

func (h PortalHandlers) List(w http.ResponseWriter, r *http.Request) {
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

	items, err := h.Repo.ListByService(r.Context(), serviceID)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL", "internal error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
}


