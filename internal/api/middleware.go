package api

import (
	"net/http"
	"strings"

	"microservice/internal/shop"
)

// MerchantAuth is a minimal shop-scoped auth middleware for early development.
//
// Contract:
// - Caller must provide the shop domain via `X-Shop-Domain` header or `?shop=` query param.
// - Middleware loads the shop record from DB and attaches it to context.
//
// Note: For production embedded apps, this must be replaced by a signed session/JWT verification.
func MerchantAuth(shopsRepo *shop.Repository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			shopDomain := strings.TrimSpace(r.Header.Get("X-Shop-Domain"))
			if shopDomain == "" {
				shopDomain = strings.TrimSpace(r.URL.Query().Get("shop"))
			}
			if shopDomain == "" {
				WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing shop identity")
				return
			}

			s, err := shopsRepo.FindByDomain(r.Context(), shopDomain)
			if err != nil {
				// Dev bootstrap: if caller provides an access token, register the shop.
				accessToken := strings.TrimSpace(r.Header.Get("X-Shopify-Access-Token"))
				if accessToken == "" {
					WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unknown shop")
					return
				}
				s, err = shopsRepo.Upsert(r.Context(), shopDomain, accessToken)
				if err != nil {
					WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to register shop")
					return
				}
			}

			// If caller provides an access token, refresh it (dev-friendly; keeps DB in sync).
			accessToken := strings.TrimSpace(r.Header.Get("X-Shopify-Access-Token"))
			if accessToken != "" && s.AccessToken != accessToken {
				if updated, err := shopsRepo.Upsert(r.Context(), shopDomain, accessToken); err == nil {
					s = updated
				}
			}

			next.ServeHTTP(w, r.WithContext(WithShop(r.Context(), s)))
		})
	}
}


