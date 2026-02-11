package serviceproduct

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"microservice/internal/api"
)

type Handlers struct {
	Repo *Repository
}

func (h Handlers) List(w http.ResponseWriter, r *http.Request) {
	s := api.ShopFromContext(r.Context())
	if s == nil {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing shop identity")
		return
	}

	recs, err := h.Repo.List(r.Context(), s.ID)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL", "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": recs})
}

type PutRequest struct {
	Config json.RawMessage `json:"config"`
}

func (h Handlers) Put(w http.ResponseWriter, r *http.Request) {
	s := api.ShopFromContext(r.Context())
	if s == nil {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing shop identity")
		return
	}

	productID := chi.URLParam(r, "shopify_product_id")
	if productID == "" {
		api.WriteError(w, http.StatusBadRequest, "VALIDATION_FAILED", "missing shopify_product_id")
		return
	}

	var req PutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid json")
		return
	}

	// Validate config contract now (structural + percent sum if percent-only).
	if _, err := ParseAndValidate(req.Config); err != nil {
		api.WriteError(w, http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
		return
	}

	rec, err := h.Repo.Upsert(r.Context(), s.ID, productID, req.Config)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL", "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rec)
}


