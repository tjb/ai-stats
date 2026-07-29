package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/tjb/ai-stats/internal/db"
	apimiddleware "github.com/tjb/ai-stats/internal/middleware"
)

type OnboardingHandler struct {
	queries *db.Queries
}

// Get handles GET /api/onboarding.
// Returns {"completed": true|false} based on whether onboarding_completed_at is set.
func (h *OnboardingHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := apimiddleware.UserIDFromCtx(r.Context())

	user, err := h.queries.GetUserByID(r.Context(), userID)
	if err != nil {
		jsonErr(w, "user not found", http.StatusNotFound)
		return
	}

	jsonOK(w, map[string]bool{"completed": user.OnboardingCompletedAt != nil})
}

// Complete handles POST /api/onboarding/complete.
// Body: {"tools": ["claude_code"], "github_org_login": "my-org"}
// Sets onboarding_completed_at = now(). Returns {"ok": true}.
func (h *OnboardingHandler) Complete(w http.ResponseWriter, r *http.Request) {
	userID := apimiddleware.UserIDFromCtx(r.Context())

	var body struct {
		Tools          []string `json:"tools"`
		GitHubOrgLogin string   `json:"github_org_login"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonErr(w, "invalid body", http.StatusBadRequest)
		return
	}

	log.Printf("onboarding complete: user=%s tools=%v github_org_login=%s",
		userID, body.Tools, body.GitHubOrgLogin)

	if err := h.queries.CompleteOnboarding(r.Context(), userID); err != nil {
		jsonErr(w, "db error", http.StatusInternalServerError)
		return
	}

	jsonOK(w, map[string]bool{"ok": true})
}
