package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"microservice/internal/shop"
	"microservice/pkg/config"
	"microservice/pkg/shopify"
)

type Handlers struct {
	Cfg       config.Config
	Shops     *shop.Repository
	Exchanger shopify.OAuthExchanger
}

func (h Handlers) Install(w http.ResponseWriter, r *http.Request) {
	shopDomain := strings.TrimSpace(r.URL.Query().Get("shop"))
	if shopDomain == "" {
		http.Error(w, "missing shop", http.StatusBadRequest)
		return
	}

	state := randomHex(16)
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   false, // local dev; set true behind TLS in prod
	})

	u := url.URL{
		Scheme: "https",
		Host:   shopDomain,
		Path:   "/admin/oauth/authorize",
	}
	q := u.Query()
	q.Set("client_id", h.Cfg.Shopify.APIKey)
	q.Set("scope", h.Cfg.Shopify.Scopes)
	q.Set("redirect_uri", h.Cfg.Shopify.RedirectURL)
	q.Set("state", state)
	u.RawQuery = q.Encode()

	http.Redirect(w, r, u.String(), http.StatusFound)
}

func (h Handlers) Callback(w http.ResponseWriter, r *http.Request) {
	qs := r.URL.Query()
	shopDomain := strings.TrimSpace(qs.Get("shop"))
	code := strings.TrimSpace(qs.Get("code"))

	if shopDomain == "" || code == "" {
		http.Error(w, "missing shop or code", http.StatusBadRequest)
		return
	}

	c, err := r.Cookie("oauth_state")
	if err != nil || c.Value == "" || c.Value != qs.Get("state") {
		http.Error(w, "invalid oauth state", http.StatusBadRequest)
		return
	}

	if !VerifyOAuthHMAC(qs, h.Cfg.Shopify.APISecret) {
		http.Error(w, "invalid hmac", http.StatusUnauthorized)
		return
	}

	ex := h.Exchanger
	ex.APIKey = h.Cfg.Shopify.APIKey
	ex.APISecret = h.Cfg.Shopify.APISecret

	token, err := ex.ExchangeCodeForToken(r.Context(), shopDomain, code)
	if err != nil {
		http.Error(w, fmt.Sprintf("token exchange: %v", err), http.StatusBadGateway)
		return
	}

	if _, err := h.Shops.Upsert(r.Context(), shopDomain, token); err != nil {
		http.Error(w, fmt.Sprintf("save shop: %v", err), http.StatusInternalServerError)
		return
	}

	// Optional: register webhooks on install if we have a public base URL.
	if strings.TrimSpace(h.Cfg.PublicBaseURL) != "" {
		base := strings.TrimRight(strings.TrimSpace(h.Cfg.PublicBaseURL), "/")
		c := shopify.Client{
			ShopDomain:  shopDomain,
			AccessToken: token,
			APIVersion:  h.Cfg.Shopify.APIVersion,
		}

		// Deposit/service purchase and milestone payment completion are handled via orders/paid.
		if err := c.CreateWebhook(r.Context(), "orders/paid", base+"/v1/webhooks/shopify/orders_paid"); err != nil {
			log.Printf("webhook register orders/paid failed shop=%s err=%v", shopDomain, err)
		}
		if err := c.CreateWebhook(r.Context(), "app/uninstalled", base+"/v1/webhooks/shopify/app_uninstalled"); err != nil {
			log.Printf("webhook register app/uninstalled failed shop=%s err=%v", shopDomain, err)
		}
	}

	_, _ = w.Write([]byte("installed"))
}

func randomHex(nBytes int) string {
	b := make([]byte, nBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}


