package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/tjb/ai-stats/internal/auth"
	"github.com/tjb/ai-stats/internal/db"
)

// Handlers is the top-level registry of all handler groups.
type Handlers struct {
	Auth       *AuthHandler
	Dashboard  *DashboardHandler
	Insights   *InsightsHandler
	Reports    *ReportsHandler
	Settings   *SettingsHandler
	GitHub     *GitHubHandler
	Tools      *ToolsHandler
	Demo       *DemoHandler
	Ingest     *IngestHandler
	Onboarding *OnboardingHandler
}

func New(queries *db.Queries, authSvc *auth.Service, jwtSecret []byte) *Handlers {
	ghHandler := &GitHubHandler{queries: queries}
	return &Handlers{
		Auth:       &AuthHandler{queries: queries, authSvc: authSvc},
		Dashboard:  &DashboardHandler{queries: queries},
		Insights:   &InsightsHandler{queries: queries},
		Reports:    &ReportsHandler{queries: queries},
		Settings:   &SettingsHandler{queries: queries, authSvc: authSvc},
		GitHub:     ghHandler,
		Tools:      &ToolsHandler{GitHubHandler: ghHandler},
		Demo:       &DemoHandler{queries: queries},
		Ingest:     &IngestHandler{queries: queries},
		Onboarding: &OnboardingHandler{queries: queries},
	}
}

// jsonOK writes a JSON 200 response.
func jsonOK(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// jsonErr writes a JSON error response.
func jsonErr(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
