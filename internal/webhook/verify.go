package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
)

// VerifyShopifyWebhook verifies the webhook signature using the shared secret.
// Signature header is base64(HMAC_SHA256(body)).
func VerifyShopifyWebhook(body []byte, hmacHeader string, secret string) bool {
	if hmacHeader == "" || secret == "" {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(hmacHeader))
}


