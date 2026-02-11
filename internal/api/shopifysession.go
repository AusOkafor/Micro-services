package api

import (
	"net/http"
	"strings"
	"time"

	"microservice/internal/shop"
	"microservice/pkg/config"
	"microservice/pkg/shopify"
)

// ShopifySessionAuth validates Shopify embedded session tokens.
//
// Expected header:
// - Authorization: Bearer <JWT>
//
// In dev, if Authorization is missing, it can fall back to X-Shop-Domain to keep local testing simple.
func ShopifySessionAuth(cfg config.Config, shopsRepo *shop.Repository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authz := strings.TrimSpace(r.Header.Get("Authorization"))
			if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
				token := strings.TrimSpace(authz[7:])
				vs, err := shopify.VerifySessionToken(token, cfg.Shopify.APIKey, cfg.Shopify.APISecret, time.Now())
				if err != nil {
					// Dev rescue: during local dev it's easy to forget exporting SHOPIFY_API_KEY/SECRET
					// into the Go API process, and the embedded app can refresh/rotate session tokens.
					// Fall back to MerchantAuth (X-Shop-Domain) so the UI doesn't brick.
					if cfg.AppEnv != "prod" {
						shopDomain := strings.TrimSpace(r.Header.Get("X-Shop-Domain"))
						if shopDomain != "" {
							MerchantAuth(shopsRepo)(next).ServeHTTP(w, r)
							return
						}
					}

					WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid session token")
					return
				}

				// Ensure the shop exists in our DB. If it doesn't, bootstrap it using the offline access token
				// provided by the embedded Remix app (server-side).
				s, err := shopsRepo.FindByDomain(r.Context(), vs.ShopDomain)
				if err != nil {
					accessToken := strings.TrimSpace(r.Header.Get("X-Shopify-Access-Token"))
					if accessToken == "" {
						WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unknown shop")
						return
					}
					s, err = shopsRepo.Upsert(r.Context(), vs.ShopDomain, accessToken)
					if err != nil {
						WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to register shop")
						return
					}
				} else {
					// If embedded app sends an offline token, refresh it (keeps DB in sync).
					accessToken := strings.TrimSpace(r.Header.Get("X-Shopify-Access-Token"))
					if accessToken != "" && s.AccessToken != accessToken {
						if updated, err := shopsRepo.Upsert(r.Context(), vs.ShopDomain, accessToken); err == nil {
							s = updated
						}
					}
				}

				next.ServeHTTP(w, r.WithContext(WithShop(r.Context(), s)))
				return
			}

			// Dev fallback
			if cfg.AppEnv != "prod" {
				MerchantAuth(shopsRepo)(next).ServeHTTP(w, r)
				return
			}

			WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing session token")
		})
	}
}


