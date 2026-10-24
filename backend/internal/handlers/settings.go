package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tjb/ai-stats/internal/auth"
	"github.com/tjb/ai-stats/internal/db"
	ghpkg "github.com/tjb/ai-stats/internal/github"
	"github.com/tjb/ai-stats/internal/ingest"
	apimiddleware "github.com/tjb/ai-stats/internal/middleware"
)

type SettingsHandler struct {
	queries *db.Queries
	authSvc *auth.Service
}

type settingsResponse struct {
	User settingsUser `json:"user"`
	Org  *settingsOrg `json:"org"`
}

type settingsUser struct {
	ID         string  `json:"id"`
	Email      string  `json:"email"`
	Name       *string `json:"name"`
	Image      *string `json:"image"`
	HourlyRate int     `json:"hourly_rate"`
}

type settingsOrg struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	GitHubOrgLogin    string  `json:"github_org_login"`
	AvatarURL         *string `json:"avatar_url"`
	GithubMemberCount *int    `json:"github_member_count"`
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := apimiddleware.UserIDFromCtx(r.Context())

	user, err := h.queries.GetUserByID(r.Context(), userID)
	if err != nil {
		jsonErr(w, "user not found", http.StatusNotFound)
		return
	}

	resp := settingsResponse{
		User: settingsUser{
			ID:         user.ID,
			Email:      user.Email,
			Name:       user.Name,
			Image:      user.Image,
			HourlyRate: user.HourlyRate,
		},
	}

	org, err := h.queries.GetOrgByOwner(r.Context(), userID)
	if err == nil {
		resp.Org = &settingsOrg{
			ID:                org.ID,
			Name:              org.Name,
			GitHubOrgLogin:    org.GitHubOrgLogin,
			AvatarURL:         org.AvatarURL,
			GithubMemberCount: org.GithubMemberCount,
		}
	}

	jsonOK(w, resp)
}

func (h *SettingsHandler) UpdateHourlyRate(w http.ResponseWriter, r *http.Request) {
	userID := apimiddleware.UserIDFromCtx(r.Context())

	var body struct {
		HourlyRate int `json:"hourly_rate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonErr(w, "invalid body", http.StatusBadRequest)
		return
	}
	if body.HourlyRate < 1 || body.HourlyRate > 10000 {
		jsonErr(w, "hourly_rate must be between 1 and 10000", http.StatusBadRequest)
		return
	}

	if err := h.queries.UpdateUserHourlyRate(r.Context(), userID, body.HourlyRate); err != nil {
		jsonErr(w, "db error", http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

func (h *SettingsHandler) DisconnectOrg(w http.ResponseWriter, r *http.Request) {
	userID := apimiddleware.UserIDFromCtx(r.Context())

	org, err := h.queries.GetOrgByOwner(r.Context(), userID)
	if err != nil {
		jsonErr(w, "no org to disconnect", http.StatusNotFound)
		return
	}
	if err := h.queries.DisconnectOrg(r.Context(), org.ID); err != nil {
		jsonErr(w, "db error", http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

func (h *SettingsHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	userID := apimiddleware.UserIDFromCtx(r.Context())

	if err := h.queries.DeleteUser(r.Context(), userID); err != nil {
		jsonErr(w, "db error", http.StatusInternalServerError)
		return
	}
	h.authSvc.ClearSession(w, r)
	jsonOK(w, map[string]bool{"ok": true})
}

// ListGitHubOrgs returns the GitHub orgs the authenticated user belongs to.
// GET /api/github/orgs
func (h *SettingsHandler) ListGitHubOrgs(w http.ResponseWriter, r *http.Request) {
	userID := apimiddleware.UserIDFromCtx(r.Context())

	token, err := h.queries.GetUserAccessToken(r.Context(), userID)
	if err != nil || token == "" {
		jsonErr(w, "no github token — please re-authenticate", http.StatusUnauthorized)
		return
	}

	orgs, err := ghpkg.ListUserOrgs(r.Context(), token)
	if err != nil {
		jsonErr(w, fmt.Sprintf("github error: %v", err), http.StatusBadGateway)
		return
	}
	jsonOK(w, orgs)
}

// ConnectOrg creates an org record for the given GitHub org login.
// POST /api/settings/connect-org  body: {"org_login": "my-org"}
func (h *SettingsHandler) ConnectOrg(w http.ResponseWriter, r *http.Request) {
	userID := apimiddleware.UserIDFromCtx(r.Context())

	// Only one org per user
	if _, err := h.queries.GetOrgByOwner(r.Context(), userID); err == nil {
		jsonErr(w, "org already connected — disconnect first", http.StatusConflict)
		return
	}

	var body struct {
		OrgLogin string `json:"org_login"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.OrgLogin == "" {
		jsonErr(w, "org_login is required", http.StatusBadRequest)
		return
	}

	token, err := h.queries.GetUserAccessToken(r.Context(), userID)
	if err != nil || token == "" {
		jsonErr(w, "no github token — please re-authenticate", http.StatusUnauthorized)
		return
	}

	ghOrg, err := ghpkg.GetOrg(r.Context(), body.OrgLogin, token)
	if err != nil {
		jsonErr(w, fmt.Sprintf("github error: %v", err), http.StatusBadGateway)
		return
	}

	name := ghOrg.Name
	if name == "" {
		name = ghOrg.Login
	}
	avatarURL := &ghOrg.AvatarURL

	org, err := h.queries.CreateOrg(r.Context(),
		fmt.Sprintf("%d", ghOrg.ID),
		ghOrg.Login,
		name,
		avatarURL,
		token,
		userID,
	)
	if err != nil {
		jsonErr(w, "db error", http.StatusInternalServerError)
		return
	}

	// Best-effort: fetch member count and persist it. Errors are non-fatal.
	if memberCount, mcErr := ghpkg.GetOrgMemberCount(r.Context(), org.GitHubOrgLogin, token); mcErr == nil {
		if updateErr := h.queries.UpdateOrgMemberCount(r.Context(), org.ID, memberCount); updateErr != nil {
			_ = updateErr // logged implicitly; non-fatal
		}
	}

	jsonOK(w, map[string]string{"org_id": org.ID, "org_login": org.GitHubOrgLogin})
}

// ListIngestKeys handles GET /api/settings/ingest-keys
func (h *SettingsHandler) ListIngestKeys(w http.ResponseWriter, r *http.Request) {
	userID := apimiddleware.UserIDFromCtx(r.Context())
	org, err := h.queries.GetOrgByOwner(r.Context(), userID)
	if err != nil {
		jsonErr(w, "no org connected", http.StatusNotFound)
		return
	}
	keys, err := h.queries.ListIngestKeys(r.Context(), org.ID)
	if err != nil {
		jsonErr(w, "db error", http.StatusInternalServerError)
		return
	}
	jsonOK(w, keys)
}

// CreateIngestKey handles POST /api/settings/ingest-keys
func (h *SettingsHandler) CreateIngestKey(w http.ResponseWriter, r *http.Request) {
	userID := apimiddleware.UserIDFromCtx(r.Context())
	org, err := h.queries.GetOrgByOwner(r.Context(), userID)
	if err != nil {
		jsonErr(w, "no org connected", http.StatusNotFound)
		return
	}
	var body struct {
		Label string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonErr(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	label := strings.TrimSpace(body.Label)
	if label == "" {
		label = "My key"
	}
	rawKey, keyHash, err := ingest.GenerateKey()
	if err != nil {
		jsonErr(w, "key generation failed", http.StatusInternalServerError)
		return
	}
	key, err := h.queries.CreateIngestKey(r.Context(), org.ID, keyHash, label)
	if err != nil {
		jsonErr(w, "db error", http.StatusInternalServerError)
		return
	}
	// Return the raw key once — it is never retrievable again after this response.
	// created_at uses the same RFC 3339 encoding as the list endpoint.
	jsonOK(w, map[string]string{
		"id":         key.ID,
		"key":        rawKey,
		"label":      key.Label,
		"created_at": key.CreatedAt.Format(time.RFC3339Nano),
	})
}

// DeleteIngestKey handles DELETE /api/settings/ingest-keys/{id}
func (h *SettingsHandler) DeleteIngestKey(w http.ResponseWriter, r *http.Request) {
	userID := apimiddleware.UserIDFromCtx(r.Context())
	org, err := h.queries.GetOrgByOwner(r.Context(), userID)
	if err != nil {
		jsonErr(w, "no org connected", http.StatusNotFound)
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.queries.DeleteIngestKey(r.Context(), id, org.ID); err != nil {
		jsonErr(w, "not found", http.StatusNotFound)
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}
