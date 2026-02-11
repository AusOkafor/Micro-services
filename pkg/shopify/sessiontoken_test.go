package shopify

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestVerifySessionToken_AudienceAndDest(t *testing.T) {
	apiKey := "test_api_key"
	secret := "test_secret"

	now := time.Unix(1700000000, 0)

	claims := SessionTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  []string{apiKey},
			ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now.Add(-1 * time.Minute)),
		},
		Dest: "https://my-shop.myshopify.com",
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	got, err := VerifySessionToken(s, apiKey, secret, now)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got.ShopDomain != "my-shop.myshopify.com" {
		t.Fatalf("shop domain mismatch: %q", got.ShopDomain)
	}
}


