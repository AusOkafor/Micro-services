package shopify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type OAuthExchanger struct {
	HTTPClient *http.Client
	APIKey     string
	APISecret  string
}

type accessTokenResponse struct {
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
}

func (o OAuthExchanger) ExchangeCodeForToken(ctx context.Context, shopDomain, code string) (string, error) {
	if o.HTTPClient == nil {
		o.HTTPClient = &http.Client{Timeout: 15 * time.Second}
	}

	body, _ := json.Marshal(map[string]string{
		"client_id":     o.APIKey,
		"client_secret": o.APISecret,
		"code":          code,
	})

	u := fmt.Sprintf("https://%s/admin/oauth/access_token", shopDomain)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("shopify token exchange failed: status=%d", resp.StatusCode)
	}

	var r accessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", err
	}
	if r.AccessToken == "" {
		return "", fmt.Errorf("shopify token exchange returned empty access_token")
	}
	return r.AccessToken, nil
}


