package shopify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	HTTPClient  *http.Client
	ShopDomain  string
	AccessToken string
	APIVersion  string
}

func (c Client) doJSON(ctx context.Context, method, path string, reqBody any, respBody any) (int, error) {
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 20 * time.Second}
	}
	if c.APIVersion == "" {
		c.APIVersion = "2025-10"
	}
	if c.ShopDomain == "" || c.AccessToken == "" {
		return 0, fmt.Errorf("missing shop domain or access token")
	}

	var buf bytes.Buffer
	if reqBody != nil {
		if err := json.NewEncoder(&buf).Encode(reqBody); err != nil {
			return 0, err
		}
	}

	u := fmt.Sprintf("https://%s/admin/api/%s%s", c.ShopDomain, c.APIVersion, path)
	req, err := http.NewRequestWithContext(ctx, method, u, &buf)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Shopify-Access-Token", c.AccessToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	b, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return resp.StatusCode, readErr
	}

	// Surface Shopify error body for non-2xx, so callers can see missing scopes, etc.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if len(b) > 0 {
			return resp.StatusCode, fmt.Errorf("shopify api error: status=%d body=%s", resp.StatusCode, string(b))
		}
		return resp.StatusCode, fmt.Errorf("shopify api error: status=%d", resp.StatusCode)
	}

	if respBody != nil && len(b) > 0 {
		if err := json.Unmarshal(b, respBody); err != nil {
			// Include body for easier debugging (unexpected shape, partial responses, etc).
			return resp.StatusCode, fmt.Errorf("decode shopify response failed: %w body=%s", err, string(b))
		}
	}

	return resp.StatusCode, nil
}


