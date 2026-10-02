package router

import (
	"embed"
	"log/slog"
	"net/http"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/alert"
	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/floodwatch-ongkharak/backend/internal/flood"
	"github.com/floodwatch-ongkharak/backend/internal/health"
	"github.com/floodwatch-ongkharak/backend/internal/http/handler"
	"github.com/floodwatch-ongkharak/backend/internal/http/middleware"
	"github.com/floodwatch-ongkharak/backend/internal/report"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"
)

//go:embed openapi.json
var staticFiles embed.FS

type Dependencies struct {
	Logger             *slog.Logger
	Readiness          *health.Service
	CORSAllowedOrigins []string
	SwaggerEnabled     bool
	AuthService        *auth.Service
	FloodService       *flood.Service
	ReportService      *report.Service
	AlertService       *alert.Service
	AdminService       handler.AdminService
	RedisClient        redis.UniversalClient
	CookieOptions      handler.CookieOptions
}

func New(dependencies Dependencies) http.Handler {
	router := chi.NewRouter()
	router.Use(chimiddleware.RequestID)
	router.Use(chimiddleware.Recoverer)
	router.Use(requestLogger(dependencies.Logger))
	router.Use(cors(dependencies.CORSAllowedOrigins))

	router.Get("/health", handler.NewHealthHandler().ServeHTTP)
	router.Get("/ready", handler.NewReadinessHandler(dependencies.Readiness).ServeHTTP)
	router.Route("/api/v1", func(api chi.Router) {
		if dependencies.RedisClient != nil {
			api.Get("/events/stream", handler.NewEventStreamHandler(dependencies.RedisClient).ServeHTTP)
		}
		if dependencies.AlertService != nil {
			alertHandler := handler.NewAlertHandler(dependencies.AlertService)
			ruleLimiter := middleware.NewRateLimiter()
			api.Get("/alerts", alertHandler.List)
			if dependencies.AuthService != nil {
				api.With(middleware.RequireAuthentication(dependencies.AuthService), middleware.RequireRole(auth.RoleAdmin), ruleLimiter.LimitAuthenticated("alert-rule-create", 10, time.Hour)).Post("/admin/alert-rules", alertHandler.CreateRule)
				api.With(middleware.RequireAuthentication(dependencies.AuthService), middleware.RequireRole(auth.RoleAdmin), ruleLimiter.LimitAuthenticated("alert-rule-update", 30, time.Hour)).Put("/admin/alert-rules/{id}", alertHandler.UpdateRule)
			}
		}
		if dependencies.AuthService != nil {
			authHandler := handler.NewAuthHandler(dependencies.AuthService, dependencies.CookieOptions)
			limiter := middleware.NewRateLimiter()
			api.With(limiter.Limit("register", 5, time.Minute)).Post("/auth/register", authHandler.Register)
			api.With(limiter.Limit("login", 10, time.Minute)).Post("/auth/login", authHandler.Login)
			api.With(limiter.Limit("refresh", 30, time.Minute)).Post("/auth/refresh", authHandler.Refresh)
			api.Post("/auth/logout", authHandler.Logout)
			api.With(middleware.RequireAuthentication(dependencies.AuthService)).Get("/auth/me", authHandler.Me)
			api.With(middleware.RequireAuthentication(dependencies.AuthService)).Get("/user/ping", func(response http.ResponseWriter, _ *http.Request) {
				response.WriteHeader(http.StatusOK)
			})
			api.With(middleware.RequireAuthentication(dependencies.AuthService), middleware.RequireRole(auth.RoleAdmin)).Get("/admin/ping", func(response http.ResponseWriter, _ *http.Request) {
				response.WriteHeader(http.StatusOK)
			})
		}
		if dependencies.FloodService != nil {
			floodHandler := handler.NewFloodHandler(dependencies.FloodService)
			publicRead := middleware.NewRateLimiter().Limit("flood-read", 120, time.Minute)
			api.With(publicRead).Get("/data-sources", floodHandler.Sources)
			api.With(publicRead).Get("/water-stations", floodHandler.Stations)
			api.With(publicRead).Get("/water-stations/{id}", floodHandler.Station)
			api.With(publicRead).Get("/water-stations/{id}/readings", floodHandler.Readings)
			api.With(publicRead).Get("/flood-events", floodHandler.Events)
			api.With(publicRead).Get("/flood-events/{id}", floodHandler.Event)
			api.With(publicRead).Get("/risk-areas", floodHandler.RiskAreas)
			api.With(publicRead).Get("/roads", floodHandler.Roads)
			api.With(publicRead).Get("/road-conditions", floodHandler.RoadConditions)
			if dependencies.AuthService != nil {
				admin := []func(http.Handler) http.Handler{middleware.RequireAuthentication(dependencies.AuthService), middleware.RequireRole(auth.RoleAdmin)}
				api.With(admin...).Post("/data-sources", floodHandler.CreateSource)
				api.With(admin...).Post("/risk-areas", floodHandler.CreateRiskArea)
				api.With(admin...).Post("/road-conditions", floodHandler.CreateRoadCondition)
			}
		}
		if dependencies.ReportService != nil && dependencies.AuthService != nil {
			reportHandler := handler.NewReportHandler(dependencies.ReportService)
			writeLimiter := middleware.NewRateLimiter()
			authenticated := middleware.RequireAuthentication(dependencies.AuthService)
			admin := []func(http.Handler) http.Handler{authenticated, middleware.RequireRole(auth.RoleAdmin)}
			api.Get("/reports", reportHandler.List)
			api.Get("/reports/{id}", reportHandler.Get)
			api.With(middleware.OptionalAuthentication(dependencies.AuthService)).Get("/reports/{id}/media/{mediaId}", reportHandler.Media)
			api.With(authenticated).Get("/reports/mine", reportHandler.ListMine)
			api.With(authenticated, writeLimiter.LimitAuthenticated("report-create", 10, time.Hour)).Post("/reports", reportHandler.Create)
			api.With(authenticated, writeLimiter.LimitAuthenticated("report-edit", 30, time.Hour)).Patch("/reports/{id}", reportHandler.Update)
			api.With(authenticated, writeLimiter.LimitAuthenticated("report-upload", 30, time.Hour)).Post("/reports/{id}/media", reportHandler.UploadMedia)
			api.With(admin...).Get("/admin/reports", reportHandler.ListAdmin)
			api.With(admin...).Get("/admin/reports/{id}", reportHandler.GetAdmin)
			api.With(authenticated, middleware.RequireRole(auth.RoleAdmin), writeLimiter.LimitAuthenticated("report-review", 30, time.Hour)).Post("/admin/reports/{id}/verify", reportHandler.Verify)
			api.With(authenticated, middleware.RequireRole(auth.RoleAdmin), writeLimiter.LimitAuthenticated("report-review", 30, time.Hour)).Post("/admin/reports/{id}/reject", reportHandler.Reject)
		}
		if dependencies.AdminService != nil && dependencies.AuthService != nil {
			adminHandler := handler.NewAdminHandler(dependencies.AdminService, dependencies.AdminService)
			adminLimiter := middleware.NewRateLimiter()
			authenticated := middleware.RequireAuthentication(dependencies.AuthService)
			adminOnly := []func(http.Handler) http.Handler{authenticated, middleware.RequireRole(auth.RoleAdmin)}
			
			// User management
			api.With(adminOnly...).Get("/admin/users", adminHandler.ListUsers)
			api.With(adminOnly...).Get("/admin/users/{id}", adminHandler.GetUser)
			api.With(adminOnly...).Patch("/admin/users/{id}/role", adminHandler.UpdateUserRole)
			api.With(adminOnly..., adminLimiter.LimitAuthenticated("admin-user-disable", 20, time.Hour)).Post("/admin/users/{id}/disable", adminHandler.DisableUser)
			api.With(adminOnly..., adminLimiter.LimitAuthenticated("admin-user-enable", 20, time.Hour)).Post("/admin/users/{id}/enable", adminHandler.EnableUser)
			
			// System settings
			api.With(adminOnly...).Get("/admin/settings", adminHandler.ListSettings)
			api.With(adminOnly...).Get("/admin/settings/{key}", adminHandler.GetSetting)
			api.With(adminOnly..., adminLimiter.LimitAuthenticated("admin-setting-update", 50, time.Hour)).Put("/admin/settings/{key}", adminHandler.UpdateSetting)
			
			// Audit logs
			api.With(adminOnly...).Get("/admin/audit-logs", adminHandler.ListAuditLogs)
			api.With(adminOnly...).Get("/admin/audit-logs/{id}", adminHandler.GetAuditLog)
		}
	})
	if dependencies.SwaggerEnabled {
		router.Get("/openapi.json", openAPIHandler)
		router.Get("/docs", func(response http.ResponseWriter, request *http.Request) {
			http.Redirect(response, request, "/docs/", http.StatusTemporaryRedirect)
		})
		router.Get("/docs/", swaggerUIHandler)
	}
	return router
}

func openAPIHandler(response http.ResponseWriter, _ *http.Request) {
	specification, err := staticFiles.ReadFile("openapi.json")
	if err != nil {
		http.Error(response, "OpenAPI contract is unavailable", http.StatusInternalServerError)
		return
	}
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = response.Write(specification)
}

func swaggerUIHandler(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = response.Write([]byte(swaggerUI))
}

func cors(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[origin] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			origin := request.Header.Get("Origin")
			if _, ok := allowed[origin]; ok {
				response.Header().Set("Access-Control-Allow-Origin", origin)
				response.Header().Set("Vary", "Origin")
				response.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, OPTIONS")
				response.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				response.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			if request.Method == http.MethodOptions {
				response.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(response, request)
		})
	}
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			started := time.Now()
			next.ServeHTTP(response, request)
			logger.Info("HTTP request complete",
				slog.String("method", request.Method),
				slog.String("path", request.URL.Path),
				slog.String("request_id", chimiddleware.GetReqID(request.Context())),
				slog.Duration("duration", time.Since(started)),
			)
		})
	}
}

const swaggerUI = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Flood Watch Ongkharak API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.29.0/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5.29.0/swagger-ui-bundle.js"></script>
  <script>window.ui = SwaggerUIBundle({url: '/openapi.json', dom_id: '#swagger-ui', persistAuthorization: true});</script>
</body>
</html>`
