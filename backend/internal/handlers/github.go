package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/tjb/ai-stats/internal/db"
	ghpkg "github.com/tjb/ai-stats/internal/github"
	apimiddleware "github.com/tjb/ai-stats/internal/middleware"
)

type GitHubHandler struct {
	queries *db.Queries
}

// orgAdapter bridges db.Organization to the interface expected by ghpkg.SyncUsage.
type orgAdapter struct {
	org *db.Organization
}

func (a *orgAdapter) GetID() string    { return a.org.ID }
func (a *orgAdapter) GetLogin() string { return a.org.GitHubOrgLogin }
func (a *orgAdapter) GetToken() string { return a.org.GitHubAccessToken }

// queriesAdapter bridges db.Queries to the interface expected by ghpkg.SyncUsage.
type queriesAdapter struct {
	q *db.Queries
}

func (a *queriesAdapter) UpsertSnapshot(ctx context.Context, orgID string, date time.Time, totalSuggestions, totalAcceptances, totalLinesAccepted, totalActiveUsers, totalLicensedUsers int, breakdown []db.BreakdownItem) error {
	return a.q.UpsertSnapshot(ctx, orgID, date, totalSuggestions, totalAcceptances, totalLinesAccepted, totalActiveUsers, totalLicensedUsers, breakdown)
}

func (h *GitHubHandler) Sync(w http.ResponseWriter, r *http.Request) {
	userID := apimiddleware.UserIDFromCtx(r.Context())

	org, err := h.queries.GetOrgByOwner(r.Context(), userID)
	if err != nil {
		jsonErr(w, "no_org", http.StatusNotFound)
		return
	}

	// Demo orgs have a placeholder token — skip the real API call.
	if strings.HasPrefix(org.GitHubOrgLogin, "demo-") {
		jsonOK(w, map[string]int{"synced": 0})
		return
	}

	synced, err := ghpkg.SyncUsage(r.Context(), &queriesAdapter{h.queries}, &orgAdapter{org})
	if err != nil {
		jsonErr(w, err.Error(), http.StatusBadGateway)
		return
	}

	// Best-effort: refresh member count. Errors are non-fatal.
	if memberCount, mcErr := ghpkg.GetOrgMemberCount(r.Context(), org.GitHubOrgLogin, org.GitHubAccessToken); mcErr == nil {
		_ = h.queries.UpdateOrgMemberCount(r.Context(), org.ID, memberCount)
	}

	jsonOK(w, map[string]int{"synced": synced})
}
