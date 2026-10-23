package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"

	"github.com/tjb/ai-stats/internal/auth"
	"github.com/tjb/ai-stats/internal/crypto"
	"github.com/tjb/ai-stats/internal/db"
	"github.com/tjb/ai-stats/internal/handlers"
	apimiddleware "github.com/tjb/ai-stats/internal/middleware"
	"github.com/tjb/ai-stats/internal/scheduler"
)

func main() {
	// Load .env if present (dev convenience)
	_ = godotenv.Load()

	encKey, err := crypto.ParseKey(mustEnv("ENCRYPTION_KEY"))
	if err != nil {
		log.Fatalf("ENCRYPTION_KEY: %v", err)
	}

	pool, err := db.NewPool(mustEnv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("db pool: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(pool); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	jwtSecret := []byte(mustEnv("JWT_SECRET"))
	githubOAuth := auth.NewGitHubOAuth(
		mustEnv("GITHUB_CLIENT_ID"),
		mustEnv("GITHUB_CLIENT_SECRET"),
		mustEnv("APP_URL")+"/api/auth/callback",
	)

	queries := db.New(pool, encKey)
	authSvc := auth.NewService(queries, jwtSecret, githubOAuth)

	h := handlers.New(queries, authSvc, jwtSecret)

	// Background sync: re-runs "Sync now" for every org every SYNC_INTERVAL.
	syncCfg, err := scheduler.ConfigFromEnv()
	if err != nil {
		log.Fatalf("scheduler: %v", err)
	}
	syncScheduler := scheduler.New(syncCfg, scheduler.NewBackend(queries))
	syncScheduler.Start()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(apimiddleware.SecurityHeaders)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{mustEnv("APP_URL")},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Public
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})
	r.Get("/api/auth/github", h.Auth.GitHubLogin)
	r.Get("/api/auth/callback", h.Auth.GitHubCallback)
	r.Post("/api/auth/logout", h.Auth.Logout)
	r.Get("/api/reports/share/{token}", h.Reports.GetByShareToken)

	// Protected
	r.Group(func(r chi.Router) {
		r.Use(apimiddleware.RequireAuth(jwtSecret))
		r.Get("/api/me", h.Auth.Me)
		r.Get("/api/dashboard", h.Dashboard.Get)
		r.Get("/api/insights", h.Insights.Get)
		r.Get("/api/reports", h.Reports.List)
		r.Post("/api/reports/generate", h.Reports.Generate)
		r.Get("/api/settings", h.Settings.Get)
		r.Post("/api/settings/hourly-rate", h.Settings.UpdateHourlyRate)
		r.Post("/api/settings/disconnect", h.Settings.DisconnectOrg)
		r.Get("/api/github/orgs", h.Settings.ListGitHubOrgs)
		r.Post("/api/settings/connect-org", h.Settings.ConnectOrg)
		r.Delete("/api/settings/account", h.Settings.DeleteAccount)
		r.Post("/api/github/sync", h.GitHub.Sync)
		r.Post("/api/tools/sync", h.Tools.Sync)
		r.Post("/api/tools/sync/all", h.Tools.SyncAll)
		r.Post("/api/demo/activate", h.Demo.Activate)
		r.Get("/api/settings/ingest-keys", h.Settings.ListIngestKeys)
		r.Post("/api/settings/ingest-keys", h.Settings.CreateIngestKey)
		r.Delete("/api/settings/ingest-keys/{id}", h.Settings.DeleteIngestKey)
		r.Get("/api/onboarding", h.Onboarding.Get)
		r.Post("/api/onboarding/complete", h.Onboarding.Complete)
	})

	// Push ingest — API key auth, no JWT
	r.Post("/api/ingest/events", h.Ingest.PostEvents)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("server listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Stop background sync first so no run is still using the pool when it closes.
	if err := syncScheduler.Stop(ctx); err != nil {
		log.Print(err)
	}
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required env var %s is not set", key)
	}
	return v
}
