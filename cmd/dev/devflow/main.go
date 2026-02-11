package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"microservice/internal/serviceproduct"
	"microservice/internal/shop"
	"microservice/pkg/config"
	"microservice/pkg/db"
)

func main() {
	var (
		webhookURL = flag.String("webhook-url", "", "local webhook url (defaults to http://localhost<HTTP_ADDR>/v1/webhooks/shopify/orders_paid)")
		shopDomain = flag.String("shop", "", "shop domain (e.g. your-store.myshopify.com)")
		token      = flag.String("access-token", "", "shop access token (optional; if omitted, uses existing DB token for the shop)")
		productID  = flag.String("product-id", "", "shopify product id used for service config and orders_paid line item")
		total      = flag.String("total", "100.00", "service total amount")
		orderID    = flag.Int64("order-id", time.Now().Unix(), "fake Shopify order id for the webhook payload")
		secret     = flag.String("webhook-secret", "", "SHOPIFY_WEBHOOK_SECRET used by server")
	)
	flag.Parse()

	if *shopDomain == "" || *productID == "" {
		fmt.Fprintln(os.Stderr, "missing -shop or -product-id")
		os.Exit(2)
	}

	cfg := config.Load()

	if *webhookURL == "" {
		*webhookURL = defaultWebhookURL(cfg.HTTPAddr)
	}

	// Prefer explicit flag, otherwise take from config/env (.env is loaded by config.Load()).
	if *secret == "" {
		*secret = cfg.Shopify.WebhookSecret
	}
	if *secret == "" {
		*secret = os.Getenv("SHOPIFY_WEBHOOK_SECRET")
	}
	if *secret == "" {
		fmt.Fprintln(os.Stderr, "missing -webhook-secret (or SHOPIFY_WEBHOOK_SECRET in env/.env)")
		os.Exit(2)
	}

	ctx := context.Background()

	pool, err := db.Open(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "db open: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	if cfg.MigrationsPath != "" {
		if err := db.Migrate(cfg.MigrationsPath, cfg.DB); err != nil {
			fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
			os.Exit(1)
		}
	}

	shopsRepo := shop.NewRepository(pool)
	var sh *shop.Shop
	if strings.TrimSpace(*token) != "" {
		var err error
		sh, err = shopsRepo.Upsert(ctx, *shopDomain, *token)
		if err != nil {
			fmt.Fprintf(os.Stderr, "upsert shop: %v\n", err)
			os.Exit(1)
		}
	} else {
		var err error
		sh, err = shopsRepo.FindByDomain(ctx, *shopDomain)
		if err != nil {
			fmt.Fprintf(os.Stderr, "shop not found; provide -access-token to seed it: %v\n", err)
			os.Exit(1)
		}
	}

	// Seed a default 50/50 template: deposit + final.
	cfgJSON, _ := json.Marshal(serviceproduct.Config{
		Version:   1,
		Currency:  "USD",
		Templates: nil,
	})
	// NOTE: We intentionally avoid importing internal/milestone types in this dev tool.
	// Store a raw JSON config matching the same schema.
	// Use numeric values (not strings) to avoid JSON decoding ambiguity.
	cfgJSON = []byte(`{"version":1,"currency":"USD","templates":[{"type":"percentage","value":50,"isFinal":false},{"type":"percentage","value":50,"isFinal":true}]}`)

	spRepo := serviceproduct.NewRepository(pool)
	_, err = spRepo.Upsert(ctx, sh.ID, *productID, cfgJSON)
	if err != nil {
		fmt.Fprintf(os.Stderr, "upsert service product config: %v\n", err)
		os.Exit(1)
	}

	payload := map[string]any{
		"id":          *orderID,
		"email":       "client@example.com",
		"total_price": *total,
		"currency":    "USD",
		"note":        "",
		"customer": map[string]any{
			"first_name": "John",
			"last_name":  "Doe",
		},
		"line_items": []map[string]any{
			{"product_id": mustInt64(*productID)},
		},
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest(http.MethodPost, *webhookURL, bytes.NewReader(body))
	if err != nil {
		fmt.Fprintf(os.Stderr, "new request: %v\n", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Shopify-Topic", "orders/paid")
	req.Header.Set("X-Shopify-Shop-Domain", *shopDomain)
	req.Header.Set("X-Shopify-Hmac-Sha256", sign(body, *secret))
	req.Header.Set("X-Shopify-Webhook-Id", fmt.Sprintf("devflow-%d", time.Now().UnixNano()))

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "post webhook: %v\n", err)
		fmt.Fprintf(os.Stderr, "tip: is the API running, and is HTTP_ADDR set correctly? webhook_url=%s\n", *webhookURL)
		os.Exit(1)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fmt.Fprintf(os.Stderr, "webhook status=%d body=%s\n", resp.StatusCode, string(b))
		os.Exit(1)
	}

	shopifyOrderID := strconv.FormatInt(*orderID, 10)
	var serviceID string
	if err := pool.QueryRow(ctx, `SELECT id FROM services WHERE shop_id=$1 AND shopify_order_id=$2`, sh.ID, shopifyOrderID).Scan(&serviceID); err != nil {
		fmt.Fprintf(os.Stderr, "find service: %v\n", err)
		os.Exit(1)
	}

	var portalToken string
	_ = pool.QueryRow(ctx, `
SELECT token
FROM portal_tokens
WHERE service_id=$1 AND revoked_at IS NULL AND expires_at > NOW()
ORDER BY created_at DESC
LIMIT 1
`, serviceID).Scan(&portalToken)

	type msRow struct {
		ID       string
		Sequence int
		Status   string
		Amount   string
	}
	rows, err := pool.Query(ctx, `SELECT id, sequence, status, amount::text FROM milestones WHERE service_id=$1 ORDER BY sequence ASC`, serviceID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "list milestones: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()
	var milestones []msRow
	for rows.Next() {
		var r msRow
		if err := rows.Scan(&r.ID, &r.Sequence, &r.Status, &r.Amount); err != nil {
			fmt.Fprintf(os.Stderr, "scan milestone: %v\n", err)
			os.Exit(1)
		}
		milestones = append(milestones, r)
	}

	fmt.Printf("Seed complete.\n")
	fmt.Printf("shop_id=%s shop_domain=%s\n", sh.ID, sh.Domain)
	fmt.Printf("service_id=%s (shopify_order_id=%s)\n", serviceID, shopifyOrderID)
	fmt.Printf("portal_token=%s\n", portalToken)
	fmt.Printf("milestones:\n")
	for _, m := range milestones {
		fmt.Printf("  - id=%s seq=%d status=%s amount=%s\n", m.ID, m.Sequence, m.Status, m.Amount)
	}

	fmt.Printf("\nNext steps:\n")
	fmt.Printf("- Merchant: set service status to WaitingForApproval, then client approves via portal.\n")
	fmt.Printf("- Client approve:\n")
	fmt.Printf("  POST http://localhost:8080/v1/portal/%s/approve\n", portalToken)
	fmt.Printf("- After approval, request final payment link via merchant endpoint:\n")
	fmt.Printf("  POST http://localhost:8080/v1/milestones/{finalMilestoneId}/request-payment (with X-Shop-Domain header)\n")
}

func defaultWebhookURL(httpAddr string) string {
	// httpAddr is typically ":8080" or "0.0.0.0:8080".
	addr := strings.TrimSpace(httpAddr)
	if addr == "" {
		addr = ":8081"
	}
	if strings.HasPrefix(addr, ":") {
		return "http://localhost" + addr + "/v1/webhooks/shopify/orders_paid"
	}
	// Strip host if present (bind address), keep port.
	if strings.HasPrefix(addr, "0.0.0.0:") {
		return "http://localhost" + strings.TrimPrefix(addr, "0.0.0.0") + "/v1/webhooks/shopify/orders_paid"
	}
	if strings.HasPrefix(addr, "127.0.0.1:") {
		return "http://" + addr + "/v1/webhooks/shopify/orders_paid"
	}
	return "http://localhost:8080/v1/webhooks/shopify/orders_paid"
}

func mustInt64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

func sign(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
