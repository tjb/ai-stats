package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	ghpkg "github.com/tjb/ai-stats/internal/github"
	apimiddleware "github.com/tjb/ai-stats/internal/middleware"
)

// ToolsHandler handles multi-tool sync requests.
type ToolsHandler struct {
	*GitHubHandler // reuse queries field via embedding
}

type syncToolRequest struct {
	ToolSource string `json:"tool_source"`
}

// Sync dispatches a sync based on the requested tool_source.
// POST /api/tools/sync
func (h *ToolsHandler) Sync(w http.ResponseWriter, r *http.Request) {
	userID := apimiddleware.UserIDFromCtx(r.Context())

	var req syncToolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, "invalid request body", http.StatusBadRequest)
		return
	}

	org, err := h.queries.GetOrgByOwner(r.Context(), userID)
	if err != nil {
		jsonErr(w, "no_org", http.StatusNotFound)
		return
	}

	// Demo orgs have a placeholder token — skip the real API call.
	if ghpkg.IsDemoOrg(org.GitHubOrgLogin) {
		jsonOK(w, map[string]interface{}{"tool_source": req.ToolSource, "synced": 0})
		return
	}

	switch req.ToolSource {
	case "copilot", "":
		// Default to Copilot when tool_source is omitted — keeps backward-compat
		// with existing callers that POST to /api/github/sync.
		synced, err := h.syncService().SyncCopilot(r.Context(), ghpkg.OrgFromDB(org))
		if err != nil {
			log.Printf("tools sync copilot org=%s: %v", org.ID, err)
			jsonErr(w, "sync failed", http.StatusBadGateway)
			return
		}
		jsonOK(w, map[string]interface{}{"tool_source": "copilot", "synced": synced})

	case "claude_code":
		synced, err := h.syncService().SyncClaudeCode(r.Context(), ghpkg.OrgFromDB(org))
		if err != nil {
			log.Printf("tools sync claude_code org=%s: %v", org.ID, err)
			jsonErr(w, "sync failed", http.StatusBadGateway)
			return
		}
		jsonOK(w, map[string]interface{}{"tool_source": "claude_code", "synced": synced})

	default:
		jsonErr(w, "unknown tool_source: must be 'copilot' or 'claude_code'", http.StatusBadRequest)
	}
}

// SyncAllResult holds the outcome for a single tool source.
type SyncAllResult = ghpkg.ToolResult

// SyncAll attempts to sync every active tool source for the org — the same
// work the background scheduler does on each run (see ghpkg.Service.SyncAll).
// POST /api/tools/sync/all
func (h *ToolsHandler) SyncAll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := apimiddleware.UserIDFromCtx(ctx)

	org, err := h.queries.GetOrgByOwner(ctx, userID)
	if err != nil {
		jsonErr(w, "no_org", http.StatusNotFound)
		return
	}

	// Demo orgs have a placeholder token — return a zero-result immediately.
	if ghpkg.IsDemoOrg(org.GitHubOrgLogin) {
		jsonOK(w, map[string]interface{}{"results": []SyncAllResult{}})
		return
	}

	results, err := h.syncService().SyncAll(ctx, ghpkg.OrgFromDB(org))
	if err != nil {
		log.Printf("sync/all org=%s: %v", org.ID, err)
	}

	jsonOK(w, map[string]interface{}{"results": results})
}
