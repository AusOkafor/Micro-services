package shopify

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type SessionTokenClaims struct {
	jwt.RegisteredClaims

	// Shopify uses custom claims; we only rely on a few.
	Dest string `json:"dest,omitempty"` // e.g. https://{shop}
}

type VerifiedSession struct {
	ShopDomain string
	ExpiresAt  time.Time
}

// VerifySessionToken verifies an embedded app session token (JWT, HS256) using the app API secret.
// It returns the shop domain derived from dest/issuer after validation.
func VerifySessionToken(tokenString string, apiKey string, apiSecret string, now time.Time) (*VerifiedSession, error) {
	if tokenString == "" {
		return nil, fmt.Errorf("missing token")
	}
	if apiSecret == "" {
		return nil, fmt.Errorf("missing api secret")
	}

	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithTimeFunc(func() time.Time { return now }),
	)
	claims := &SessionTokenClaims{}
	tok, err := parser.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		return []byte(apiSecret), nil
	})
	if err != nil {
		return nil, err
	}
	if !tok.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	// Time validation
	if claims.ExpiresAt == nil || claims.ExpiresAt.Time.Before(now) {
		return nil, fmt.Errorf("token expired")
	}
	if claims.NotBefore != nil && claims.NotBefore.Time.After(now) {
		return nil, fmt.Errorf("token not active yet")
	}

	// Audience validation (should include apiKey)
	if apiKey != "" {
		if !audContains([]string(claims.RegisteredClaims.Audience), apiKey) {
			return nil, fmt.Errorf("audience mismatch")
		}
	}

	shopDomain := extractShopFromClaims(claims)
	if shopDomain == "" {
		return nil, fmt.Errorf("missing shop in token")
	}

	return &VerifiedSession{
		ShopDomain: shopDomain,
		ExpiresAt:  claims.ExpiresAt.Time,
	}, nil
}

func audContains(aud []string, want string) bool {
	for _, a := range aud {
		if a == want {
			return true
		}
	}
	return false
}

func extractShopFromClaims(c *SessionTokenClaims) string {
	// Prefer dest: "https://{shop}"
	if c.Dest != "" {
		s := strings.TrimSpace(c.Dest)
		s = strings.TrimPrefix(s, "https://")
		s = strings.TrimPrefix(s, "http://")
		s = strings.TrimSuffix(s, "/")
		if s != "" {
			return s
		}
	}
	// Fallback: issuer might contain shop url-ish value
	if c.Issuer != "" {
		s := strings.TrimSpace(c.Issuer)
		s = strings.TrimPrefix(s, "https://")
		s = strings.TrimPrefix(s, "http://")
		s = strings.TrimSuffix(s, "/")
		return s
	}
	return ""
}


