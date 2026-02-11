package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
)

// VerifyOAuthHMAC verifies Shopify's OAuth callback HMAC.
// Shopify computes the HMAC over the querystring (excluding hmac and signature) in lexicographical order.
func VerifyOAuthHMAC(values url.Values, apiSecret string) bool {
	given := values.Get("hmac")
	if given == "" || apiSecret == "" {
		return false
	}

	var keys []string
	for k := range values {
		if k == "hmac" || k == "signature" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		for _, v := range values[k] {
			parts = append(parts, k+"="+strings.ReplaceAll(v, "&", "%26"))
		}
	}
	msg := strings.Join(parts, "&")

	mac := hmac.New(sha256.New, []byte(apiSecret))
	_, _ = mac.Write([]byte(msg))
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(given))
}


