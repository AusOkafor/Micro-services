package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func main() {
	var (
		url       = flag.String("url", "", "webhook endpoint url (defaults to http://localhost<HTTP_ADDR>/v1/webhooks/shopify/orders_paid)")
		topic     = flag.String("topic", "orders/paid", "shopify topic header value")
		shop      = flag.String("shop", "example.myshopify.com", "X-Shopify-Shop-Domain")
		secret    = flag.String("secret", "", "SHOPIFY_WEBHOOK_SECRET")
		payload   = flag.String("payload", "", "path to json payload file")
		webhookID = flag.String("id", "", "optional webhook id header value")
	)
	flag.Parse()

	if *url == "" {
		httpAddr := os.Getenv("HTTP_ADDR")
		if httpAddr == "" {
			httpAddr = ":8080"
		}
		if len(httpAddr) > 0 && httpAddr[0] == ':' {
			*url = "http://localhost" + httpAddr + "/v1/webhooks/shopify/orders_paid"
		} else {
			*url = "http://localhost:8080/v1/webhooks/shopify/orders_paid"
		}
	}

	if *secret == "" {
		fmt.Fprintln(os.Stderr, "missing -secret")
		os.Exit(2)
	}
	if *payload == "" {
		fmt.Fprintln(os.Stderr, "missing -payload")
		os.Exit(2)
	}

	b, err := os.ReadFile(*payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read payload: %v\n", err)
		os.Exit(2)
	}

	sig := sign(b, *secret)

	req, err := http.NewRequest(http.MethodPost, *url, bytes.NewReader(b))
	if err != nil {
		fmt.Fprintf(os.Stderr, "new request: %v\n", err)
		os.Exit(2)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Shopify-Topic", *topic)
	req.Header.Set("X-Shopify-Shop-Domain", *shop)
	req.Header.Set("X-Shopify-Hmac-Sha256", sig)
	if *webhookID != "" {
		req.Header.Set("X-Shopify-Webhook-Id", *webhookID)
	}

	c := &http.Client{Timeout: 10 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "post: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("status=%d\n%s\n", resp.StatusCode, string(body))
}

func sign(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}


