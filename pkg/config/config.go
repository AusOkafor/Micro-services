package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv         string
	HTTPAddr       string
	MigrationsPath string

	// Supabase/hosted Postgres convenience:
	// - DATABASE_URL: runtime connection (often PgBouncer/pooler)
	// - DIRECT_URL: direct connection for migrations
	DatabaseURL string
	DirectURL   string

	// PublicBaseURL is the externally reachable URL for this backend (required for webhook registration).
	// Example: https://your-ngrok-subdomain.ngrok-free.app
	PublicBaseURL string

	DB DBConfig

	Shopify ShopifyConfig

	// PortalAllowedOrigins is a comma-separated allowlist of origins allowed to call
	// the public portal endpoints (token-based). Example:
	//   https://portal.yourapp.com,http://localhost:5173
	PortalAllowedOrigins []string

	// PortalSupportEmail is shown in the client portal footer (optional).
	PortalSupportEmail string

	// PortalLogoURL is shown in the client portal header (optional).
	PortalLogoURL string
}

type DBConfig struct {
	Host     string
	Port     string
	Name     string
	User     string
	Password string
	SSLMode  string
}

type ShopifyConfig struct {
	APIKey      string
	APISecret   string
	Scopes      string
	RedirectURL string

	WebhookSecret string

	APIVersion string

	// DevAdminAccessToken is a convenience for local dev when using a Shopify "Develop app" (custom app)
	// Admin API access token (usually starts with "shpat_"). If set, some handlers may prefer this token
	// over the stored shop access token for Shopify Admin API calls.
	//
	// Never set this in production.
	DevAdminAccessToken string
}

func Load() Config {
	// Convenience for local dev: load variables from .env if present.
	// In production, rely on real environment variables.
	_ = godotenv.Load()

	// Cloud Run sets PORT. Prefer it when HTTP_ADDR isn't explicitly set.
	httpAddr := os.Getenv("HTTP_ADDR")
	if httpAddr == "" {
		if port := os.Getenv("PORT"); port != "" {
			httpAddr = ":" + port
		} else {
			httpAddr = ":8081"
		}
	}

	return Config{
		AppEnv:         env("APP_ENV", "dev"),
		HTTPAddr:       httpAddr,
		MigrationsPath: os.Getenv("MIGRATIONS_PATH"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		DirectURL:      os.Getenv("DIRECT_URL"),
		PublicBaseURL:  os.Getenv("PUBLIC_BASE_URL"),
		DB: DBConfig{
			Host:     env("DB_HOST", "localhost"),
			Port:     env("DB_PORT", "5432"),
			Name:     env("DB_NAME", "microservice"),
			User:     env("DB_USER", "microservice"),
			Password: env("DB_PASSWORD", "microservice"),
			SSLMode:  env("DB_SSLMODE", "disable"),
		},
		Shopify: ShopifyConfig{
			APIKey:        os.Getenv("SHOPIFY_API_KEY"),
			APISecret:     os.Getenv("SHOPIFY_API_SECRET"),
			Scopes:        os.Getenv("SHOPIFY_SCOPES"),
			RedirectURL:   os.Getenv("SHOPIFY_REDIRECT_URL"),
			WebhookSecret: os.Getenv("SHOPIFY_WEBHOOK_SECRET"),
			APIVersion:    env("SHOPIFY_API_VERSION", "2025-10"),
			DevAdminAccessToken: os.Getenv("SHOPIFY_DEV_ADMIN_ACCESS_TOKEN"),
		},

		PortalAllowedOrigins: envList("PORTAL_ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:4173"),
		PortalSupportEmail:   os.Getenv("PORTAL_SUPPORT_EMAIL"),
		PortalLogoURL:        os.Getenv("PORTAL_LOGO_URL"),
	}
}

func env(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func envList(key, fallbackCSV string) []string {
	v := os.Getenv(key)
	if v == "" {
		v = fallbackCSV
	}
	var out []string
	start := 0
	for i := 0; i <= len(v); i++ {
		if i == len(v) || v[i] == ',' {
			s := v[start:i]
			start = i + 1
			// trim spaces
			for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n' || s[0] == '\r') {
				s = s[1:]
			}
			for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
				s = s[:len(s)-1]
			}
			if s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}


