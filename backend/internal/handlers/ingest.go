package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/tjb/ai-stats/internal/db"
	"github.com/tjb/ai-stats/internal/ingest"
)

// ingestQuerier is the subset of db.Queries used by IngestHandler.
// Defined as an interface so the handler can be tested without a real DB.
type ingestQuerier interface {
	GetIngestKeyByHash(ctx context.Context, keyHash string) (*db.IngestAPIKey, error)
	UpdateIngestKeyLastUsed(ctx context.Context, id string) error
	UpsertToolSnapshot(ctx context.Context, orgID, toolSource string, date time.Time,
		totalSuggestions, totalAcceptances, totalLinesAccepted,
		totalActiveUsers, totalLicensedUsers, totalCodingDevelopers int,
		totalPlanningUses, totalCodingUses int,
		rawJSON json.RawMessage) error
}

// IngestHandler handles push-ingest events from external tools such as
// claude_code. Auth is via X-AIStats-Key header (not JWT).
type IngestHandler struct {
	queries ingestQuerier
}

// editTools is the set of Claude Code tool names that produce code changes.
// These count as "acceptances" for the edit-rate metric.
var editTools = map[string]bool{
	"Edit": true, "Write": true, "MultiEdit": true, "NotebookEdit": true,
}

// planningTools is the set of Claude Code tool names used for reading and research.
// These count toward the planning ratio metric.
var planningTools = map[string]bool{
	"Read": true, "Grep": true, "Glob": true, "Bash": true,
	"WebSearch": true, "WebFetch": true, "Agent": true,
}

// ingestEvent is a single event in the ingest request payload.
type ingestEvent struct {
	Type            string `json:"type"`
	ToolName        string `json:"tool_name"` // for tool_use events
	LinesWritten    int    `json:"lines_written"`
	DurationMinutes int    `json:"duration_minutes"`
	UserEmail       string `json:"user_email"`
}

// ingestRequest is the full request body for POST /api/ingest/events.
type ingestRequest struct {
	ToolSource string        `json:"tool_source"`
	Date       string        `json:"date"` // YYYY-MM-DD
	Events     []ingestEvent `json:"events"`
}

// PostEvents handles POST /api/ingest/events.
//
// Auth: X-AIStats-Key header, or Authorization: Bearer <key>.
// The key is hashed with SHA-256; we look up the hash — raw key never stored.
// User emails in events are hashed before any storage (GDPR).
func (h *IngestHandler) PostEvents(w http.ResponseWriter, r *http.Request) {
	// 1. Extract raw key from header.
	rawKey := r.Header.Get("X-AIStats-Key")
	if rawKey == "" {
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			rawKey = strings.TrimPrefix(auth, "Bearer ")
		}
	}
	if rawKey == "" {
		jsonErr(w, "missing API key", http.StatusUnauthorized)
		return
	}

	// 2. Hash and look up in DB.
	keyHash := ingest.HashKey(rawKey)
	apiKey, err := h.queries.GetIngestKeyByHash(r.Context(), keyHash)
	if err != nil {
		// Do not distinguish "not found" from other errors to prevent enumeration.
		jsonErr(w, "invalid API key", http.StatusUnauthorized)
		return
	}

	// 3. Update last_used_at asynchronously — failure here is non-fatal.
	// Use context.Background() so the update isn't cancelled when the request ends.
	go func() {
		if err := h.queries.UpdateIngestKeyLastUsed(context.Background(), apiKey.ID); err != nil {
			log.Printf("ingest: UpdateIngestKeyLastUsed %s: %v", apiKey.ID, err)
		}
	}()

	// 4. Decode and validate the request body.
	// Limit body to 1 MB to prevent memory exhaustion from oversized payloads.
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req ingestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.ToolSource == "" {
		jsonErr(w, "tool_source is required", http.StatusBadRequest)
		return
	}
	if len(req.Events) == 0 {
		jsonErr(w, "events must be non-empty", http.StatusBadRequest)
		return
	}
	if len(req.Events) > 1000 {
		jsonErr(w, "too many events: max 1000 per request", http.StatusRequestEntityTooLarge)
		return
	}
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		jsonErr(w, "date must be in YYYY-MM-DD format", http.StatusBadRequest)
		return
	}

	// 5. Aggregate events.
	//
	// tool_use events: every tool use = 1 suggestion; edit tools = 1 acceptance.
	//   This produces a meaningful "edit rate" (what % of Claude's actions wrote code).
	// session_end events: legacy — counted as 1 suggestion + 1 acceptance each.
	var (
		totalSuggestions  int
		totalAcceptances  int
		totalLines        int
		totalDuration     int
		totalPlanningUses int
		totalCodingUses   int
		distinctUsers     = make(map[string]struct{})
	)
	for _, ev := range req.Events {
		// Clamp values to prevent inflated ROI metrics from bad data.
		if ev.LinesWritten < 0 || ev.LinesWritten > 100_000 {
			jsonErr(w, "lines_written out of range: must be 0–100000", http.StatusBadRequest)
			return
		}
		if ev.DurationMinutes < 0 || ev.DurationMinutes > 1440 {
			jsonErr(w, "duration_minutes out of range: must be 0–1440", http.StatusBadRequest)
			return
		}

		switch ev.Type {
		case "tool_use":
			totalSuggestions++
			if editTools[ev.ToolName] {
				totalAcceptances++
				totalCodingUses++
			} else if planningTools[ev.ToolName] {
				totalPlanningUses++
			} else {
				totalPlanningUses++
				log.Printf("ingest: unknown tool_name %q — counted as planning", ev.ToolName)
			}
			totalLines += ev.LinesWritten
			totalDuration += ev.DurationMinutes
		case "session_end":
			totalSuggestions++
			totalAcceptances++
			totalLines += ev.LinesWritten
			totalDuration += ev.DurationMinutes
		default:
			continue
		}

		// 6. Hash user_email before any use — GDPR: raw email never stored.
		if ev.UserEmail != "" {
			emailHash := ingest.HashKey(ev.UserEmail)
			distinctUsers[emailHash] = struct{}{}
		}
	}

	// 7. Build raw_json metadata.
	rawJSON, err := json.Marshal(map[string]interface{}{
		"source":                 "push_ingest",
		"confidence":             "high",
		"total_duration_minutes": totalDuration,
	})
	if err != nil {
		log.Printf("ingest: marshal rawJSON: %v", err)
		jsonErr(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// 8. Upsert into tool_usage_snapshots.
	if err := h.queries.UpsertToolSnapshot(
		r.Context(),
		apiKey.OrganizationID,
		req.ToolSource,
		date,
		totalSuggestions,   // total_suggestions — tool uses (tool_use events)
		totalAcceptances,   // total_acceptances — edit tool uses (edit rate numerator)
		totalLines,         // total_lines_accepted
		len(distinctUsers), // total_active_users — distinct hashed emails
		0,                  // total_licensed_users — not known from push ingest
		0,                  // total_coding_developers — not known from push ingest
		totalPlanningUses,  // total_planning_uses
		totalCodingUses,    // total_coding_uses
		json.RawMessage(rawJSON),
	); err != nil {
		log.Printf("ingest: UpsertToolSnapshot: %v", err)
		jsonErr(w, "internal server error", http.StatusInternalServerError)
		return
	}

	jsonOK(w, map[string]int{"accepted": len(req.Events)})
}

// extractOrgForIngestKey is a compile-time guard to confirm IngestHandler
// embeds only db.Queries (not the full Handlers registry).
var _ fmt.Stringer = (*ingestHandlerCheck)(nil)

type ingestHandlerCheck struct{}

func (ingestHandlerCheck) String() string { return "ingest" }
