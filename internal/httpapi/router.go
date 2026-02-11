package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"microservice/internal/api"
	"microservice/internal/auth"
	"microservice/internal/approval"
	"microservice/internal/files"
	"microservice/internal/milestone"
	"microservice/internal/payment"
	"microservice/internal/portal"
	"microservice/internal/service"
	"microservice/internal/serviceproduct"
	"microservice/internal/shop"
	"microservice/internal/webhook"
	"microservice/pkg/config"
)

type Dependencies struct {
	Cfg config.Config
	DB  *pgxpool.Pool
}

func NewRouter(deps Dependencies) http.Handler {
	r := chi.NewRouter()

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	shopsRepo := shop.NewRepository(deps.DB)
	authHandlers := auth.Handlers{
		Cfg:   deps.Cfg,
		Shops: shopsRepo,
	}
	serviceProductRepo := serviceproduct.NewRepository(deps.DB)
	serviceProductHandlers := serviceproduct.Handlers{Repo: serviceProductRepo}
	filesRepo := files.NewRepository(deps.DB)
	merchantFilesHandlers := files.MerchantHandlers{DB: deps.DB, Repo: filesRepo}
	serviceRepo := service.NewRepository(deps.DB)
	milestoneRepo := milestone.NewRepository(deps.DB)
	approvalRepo := approval.NewRepository(deps.DB)
	serviceHandlers := service.Handlers{
		DB:         deps.DB,
		Services:   serviceRepo,
		Milestones: milestoneRepo,
		Approvals:  approvalRepo,
	}
	paymentHandlers := payment.Handlers{
		Cfg:        deps.Cfg,
		DB:         deps.DB,
		Milestones: milestoneRepo,
	}
	webhookHandler := webhook.Handler{
		Cfg:             deps.Cfg,
		DB:              deps.DB,
		Shops:           shopsRepo,
		ServiceProducts: serviceProductRepo,
	}

	// v1
	r.Route("/v1", func(r chi.Router) {
		// Auth/install placeholders (wired to Shopify later)
		r.Get("/auth/install", authHandlers.Install)
		r.Get("/auth/callback", authHandlers.Callback)

		// Merchant admin APIs (shop-scoped)
		r.Group(func(r chi.Router) {
			// Production: Shopify embedded session token auth
			// Dev: falls back to X-Shop-Domain if Authorization is missing.
			r.Use(api.ShopifySessionAuth(deps.Cfg, shopsRepo))

			// Service product config
			r.Get("/service-products", serviceProductHandlers.List)
			r.Put("/service-products/{shopify_product_id}", serviceProductHandlers.Put)

			// Services (still to implement)
			r.Get("/services", serviceHandlers.List)
			r.Get("/services/{id}", serviceHandlers.Get)
			r.Patch("/services/{id}/status", serviceHandlers.PatchStatus)
			r.Get("/services/{id}/events", serviceHandlers.Events)
			r.Post("/services/{id}/admin/override", serviceHandlers.AdminOverride)
			r.Post("/services/{id}/files", merchantFilesHandlers.Create)
			r.Get("/services/{id}/files", merchantFilesHandlers.List)

			// Milestones payments
			r.Post("/milestones/{id}/request-payment", paymentHandlers.RequestPayment)
		})

		// Portal
		r.Route("/portal", func(r chi.Router) {
			// Public, token-based endpoints used by a separate frontend domain.
			// Only allow CORS for explicitly configured origins.
			r.Use(api.CORSMiddleware(api.CORSOptions{
				AllowedOrigins: deps.Cfg.PortalAllowedOrigins,
				AllowedMethods: []string{"GET", "POST", "OPTIONS"},
				AllowedHeaders: []string{"Content-Type"},
				MaxAgeSeconds:  600,
			}))

			portalHandlers := portal.Handlers{DB: deps.DB, Milestones: milestoneRepo, Cfg: deps.Cfg}
			r.Get("/{token}", portalHandlers.View)
			r.Get("/{token}/events", portalHandlers.Events)
			r.Post("/{token}/approve", portalHandlers.Approve)
			r.Post("/{token}/request-revision", portalHandlers.RequestRevision)

			portalFilesHandlers := files.PortalHandlers{DB: deps.DB, Repo: filesRepo}
			r.Post("/{token}/files", portalFilesHandlers.Create)
			r.Get("/{token}/files", portalFilesHandlers.List)
		})

		// Webhooks
		r.Post("/webhooks/shopify/{topic}", webhookHandler.ServeHTTP)
	})

	return r
}

func notImplemented(w http.ResponseWriter, r *http.Request) {
	api.WriteError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "not implemented")
}


