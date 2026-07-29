package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

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
	if strings.HasPrefix(org.GitHubOrgLogin, "demo-") {
		jsonOK(w, map[string]interface{}{"tool_source": req.ToolSource, "synced": 0})
		return
	}

	switch req.ToolSource {
	case "copilot", "":
		// Default to Copilot when tool_source is omitted — keeps backward-compat
		// with existing callers that POST to /api/github/sync.
		synced, err := ghpkg.SyncUsage(r.Context(), &queriesAdapter{h.queries}, &orgAdapter{org})
		if err != nil {
			log.Printf("tools sync copilot org=%s: %v", org.ID, err)
			jsonErr(w, "sync failed", http.StatusBadGateway)
			return
		}
		jsonOK(w, map[string]interface{}{"tool_source": "copilot", "synced": synced})

	case "claude_code":
		synced, err := ghpkg.SyncClaudeCodeAttribution(
			r.Context(),
			org.ID,
			org.GitHubOrgLogin,
			org.GitHubAccessToken,
			h.queries,
		)
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
type SyncAllResult struct {
	ToolSource string `json:"tool_source"`
	Synced     int    `json:"synced"`
	Error      string `json:"error,omitempty"`
}

// SyncAll attempts to sync every active tool source for the org.
// It always tries Copilot (if a token is present) and then any tool sources
// previously seen in tool_usage_snapshots (e.g. claude_code). Push-ingest
// sources (codex, push_ingest) are skipped — they write data themselves.
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
	if strings.HasPrefix(org.GitHubOrgLogin, "demo-") {
		jsonOK(w, map[string]interface{}{"results": []SyncAllResult{}})
		return
	}

	var results []SyncAllResult

	// Always attempt Copilot — it's the primary source.
	copilotSynced, copilotErr := ghpkg.SyncUsage(ctx, &queriesAdapter{h.queries}, &orgAdapter{org})
	r1 := SyncAllResult{ToolSource: "copilot", Synced: copilotSynced}
	if copilotErr != nil {
		log.Printf("sync/all copilot org=%s: %v", org.ID, copilotErr)
		r1.Error = copilotErr.Error()
	}
	results = append(results, r1)

	// Check which additional tool sources are active in tool_usage_snapshots.
	activeSources, err := h.queries.GetActiveToolSources(ctx, org.ID)
	if err != nil {
		log.Printf("sync/all GetActiveToolSources org=%s: %v", org.ID, err)
	}

	for _, src := range activeSources {
		switch src {
		case "copilot":
			// already handled above
		case "claude_code":
			synced, err := ghpkg.SyncClaudeCodeAttribution(ctx, org.ID, org.GitHubOrgLogin, org.GitHubAccessToken, h.queries)
			r2 := SyncAllResult{ToolSource: "claude_code", Synced: synced}
			if err != nil {
				log.Printf("sync/all claude_code org=%s: %v", org.ID, err)
				r2.Error = err.Error()
			}
			results = append(results, r2)
		default:
			// push-ingest sources (codex, push_ingest, etc.) write data themselves — skip
		}
	}

	// Always refresh member count — best-effort, non-fatal.
	if memberCount, mcErr := ghpkg.GetOrgMemberCount(ctx, org.GitHubOrgLogin, org.GitHubAccessToken); mcErr == nil {
		if updateErr := h.queries.UpdateOrgMemberCount(ctx, org.ID, memberCount); updateErr != nil {
			log.Printf("sync/all UpdateOrgMemberCount org=%s: %v", org.ID, updateErr)
		}
	} else {
		log.Printf("sync/all GetOrgMemberCount org=%s: %v", org.ID, mcErr)
	}

	jsonOK(w, map[string]interface{}{"results": results})
}
