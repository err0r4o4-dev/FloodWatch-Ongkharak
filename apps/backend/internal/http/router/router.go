package router

import (
	"embed"
	"log/slog"
	"net/http"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/floodwatch-ongkharak/backend/internal/health"
	"github.com/floodwatch-ongkharak/backend/internal/http/handler"
	"github.com/floodwatch-ongkharak/backend/internal/http/middleware"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

//go:embed openapi.json
var staticFiles embed.FS

type Dependencies struct {
	Logger             *slog.Logger
	Readiness          *health.Service
	CORSAllowedOrigins []string
	SwaggerEnabled     bool
	AuthService        *auth.Service
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
	if dependencies.AuthService != nil {
		authHandler := handler.NewAuthHandler(dependencies.AuthService, dependencies.CookieOptions)
		limiter := middleware.NewRateLimiter()
		router.Route("/api/v1", func(api chi.Router) {
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
		})
	}
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
				response.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
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
