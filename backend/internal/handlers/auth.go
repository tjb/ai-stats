package handlers

import (
	"log"
	"net/http"
	"os"

	"github.com/tjb/ai-stats/internal/auth"
	"github.com/tjb/ai-stats/internal/db"
	apimiddleware "github.com/tjb/ai-stats/internal/middleware"
)

type AuthHandler struct {
	queries *db.Queries
	authSvc *auth.Service
}

// GitHubLogin starts the OAuth flow.
func (h *AuthHandler) GitHubLogin(w http.ResponseWriter, r *http.Request) {
	h.authSvc.BeginOAuth(w, r)
}

// GitHubCallback handles the GitHub OAuth callback.
func (h *AuthHandler) GitHubCallback(w http.ResponseWriter, r *http.Request) {
	_, err := h.authSvc.CompleteOAuth(r.Context(), w, r)
	if err != nil {
		log.Printf("oauth callback error: %v", err)
		http.Error(w, "authentication failed", http.StatusBadRequest)
		return
	}
	appURL := os.Getenv("APP_URL")
	if appURL == "" {
		appURL = "http://localhost:3000"
	}
	http.Redirect(w, r, appURL+"/dashboard", http.StatusFound)
}

// Logout clears the session cookie.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	h.authSvc.ClearSession(w, r)
	jsonOK(w, map[string]bool{"ok": true})
}

// Me returns the current authenticated user.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID := apimiddleware.UserIDFromCtx(r.Context())
	user, err := h.queries.GetUserByID(r.Context(), userID)
	if err != nil {
		jsonErr(w, "user not found", http.StatusNotFound)
		return
	}
	jsonOK(w, map[string]interface{}{
		"id":          user.ID,
		"email":       user.Email,
		"name":        user.Name,
		"image":       user.Image,
		"hourly_rate": user.HourlyRate,
	})
}
