package handlers

import (
	"net/http"

	"github.com/tjb/ai-stats/internal/db"
	ghpkg "github.com/tjb/ai-stats/internal/github"
	apimiddleware "github.com/tjb/ai-stats/internal/middleware"
)

type GitHubHandler struct {
	queries *db.Queries
}

// syncService returns the GitHub sync service shared with the background
// scheduler, so manual and scheduled syncs run the same code.
func (h *GitHubHandler) syncService() *ghpkg.Service {
	return ghpkg.NewService(h.queries)
}

func (h *GitHubHandler) Sync(w http.ResponseWriter, r *http.Request) {
	userID := apimiddleware.UserIDFromCtx(r.Context())

	org, err := h.queries.GetOrgByOwner(r.Context(), userID)
	if err != nil {
		jsonErr(w, "no_org", http.StatusNotFound)
		return
	}

	// Demo orgs have a placeholder token — skip the real API call.
	if ghpkg.IsDemoOrg(org.GitHubOrgLogin) {
		jsonOK(w, map[string]int{"synced": 0})
		return
	}

	svc := h.syncService()
	ghOrg := ghpkg.OrgFromDB(org)
	synced, err := svc.SyncCopilot(r.Context(), ghOrg)
	if err != nil {
		jsonErr(w, err.Error(), http.StatusBadGateway)
		return
	}

	// Best-effort: refresh member count. Errors are non-fatal.
	_ = svc.RefreshMemberCount(r.Context(), ghOrg)

	jsonOK(w, map[string]int{"synced": synced})
}
