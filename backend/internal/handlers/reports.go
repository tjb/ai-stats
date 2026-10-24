package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tjb/ai-stats/internal/db"
	apimiddleware "github.com/tjb/ai-stats/internal/middleware"
)

type ReportsHandler struct {
	queries *db.Queries
}

// ReportContent is the structured payload stored in content_json and returned
// to the frontend. Every field maps to one of the five dashboard sections.
type ReportContent struct {
	// Executive Summary
	TotalAcceptedLines  int     `json:"total_accepted_lines"`
	TotalLinesWritten   int     `json:"total_lines_written"`
	ActiveDevelopers    int     `json:"active_developers"`
	EstimatedHoursSaved float64 `json:"estimated_hours_saved"`
	EstimatedValueSaved float64 `json:"estimated_value_saved"`

	// Adoption
	AdoptionRate      *float64 `json:"adoption_rate"`
	GithubMemberCount *int     `json:"github_member_count"`

	// Quality Signal
	AcceptanceRate      float64 `json:"acceptance_rate"`
	TotalSuggestedLines int     `json:"total_suggested_lines"`

	// ROI Detail
	HourlyRate int `json:"hourly_rate"`

	// Planning vs Coding
	PlanningToolUses int     `json:"planning_tool_uses"`
	CodingToolUses   int     `json:"coding_tool_uses"`
	PlanningRatio    float64 `json:"planning_ratio"`

	// Date range echo
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

type reportResponse struct {
	ID         string          `json:"id"`
	Month      string          `json:"month"`
	StartDate  string          `json:"start_date"`
	EndDate    string          `json:"end_date"`
	Content    json.RawMessage `json:"content"`
	ShareToken *string         `json:"share_token"`
	CreatedAt  string          `json:"created_at"`
}

func toReportResponse(r db.Report) reportResponse {
	rr := reportResponse{
		ID:         r.ID,
		Month:      r.Month,
		Content:    r.ContentJSON,
		ShareToken: r.ShareToken,
		CreatedAt:  r.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}

	// Best-effort decode to surface start_date/end_date at the top level.
	var c ReportContent
	if err := json.Unmarshal(r.ContentJSON, &c); err == nil {
		rr.StartDate = c.StartDate
		rr.EndDate = c.EndDate
	}

	return rr
}

func generateShareToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generateShareToken: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// GetByShareToken handles GET /api/reports/share/:token — no auth required.
func (h *ReportsHandler) GetByShareToken(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		jsonErr(w, "token required", http.StatusBadRequest)
		return
	}
	report, err := h.queries.GetReportByShareToken(r.Context(), token)
	if err != nil {
		jsonErr(w, "not found", http.StatusNotFound)
		return
	}
	jsonOK(w, toReportResponse(report))
}

func (h *ReportsHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := apimiddleware.UserIDFromCtx(r.Context())

	org, err := h.queries.GetOrgByOwner(r.Context(), userID)
	if err != nil {
		jsonErr(w, "no_org", http.StatusNotFound)
		return
	}

	reports, err := h.queries.ListReports(r.Context(), org.ID, 12)
	if err != nil {
		jsonErr(w, "db error", http.StatusInternalServerError)
		return
	}

	out := make([]reportResponse, len(reports))
	for i, rp := range reports {
		out[i] = toReportResponse(rp)
	}
	jsonOK(w, out)
}

func (h *ReportsHandler) Generate(w http.ResponseWriter, r *http.Request) {
	userID := apimiddleware.UserIDFromCtx(r.Context())

	var body struct {
		StartDate string `json:"start_date"` // YYYY-MM-DD
		EndDate   string `json:"end_date"`   // YYYY-MM-DD
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonErr(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.StartDate == "" || body.EndDate == "" {
		jsonErr(w, "start_date and end_date are required (YYYY-MM-DD)", http.StatusBadRequest)
		return
	}

	startDate, err := time.Parse("2006-01-02", body.StartDate)
	if err != nil {
		jsonErr(w, "start_date must be YYYY-MM-DD", http.StatusBadRequest)
		return
	}
	endDate, err := time.Parse("2006-01-02", body.EndDate)
	if err != nil {
		jsonErr(w, "end_date must be YYYY-MM-DD", http.StatusBadRequest)
		return
	}
	if endDate.Before(startDate) {
		jsonErr(w, "end_date must be >= start_date", http.StatusBadRequest)
		return
	}
	if endDate.Sub(startDate) > 365*24*time.Hour {
		jsonErr(w, "date range must not exceed 365 days", http.StatusBadRequest)
		return
	}

	org, err := h.queries.GetOrgByOwner(r.Context(), userID)
	if err != nil {
		jsonErr(w, "no_org", http.StatusNotFound)
		return
	}

	user, err := h.queries.GetUserByID(r.Context(), userID)
	if err != nil {
		jsonErr(w, "user not found", http.StatusInternalServerError)
		return
	}

	// --- Copilot snapshots ---
	copilotSnaps, err := h.queries.GetSnapshotsByDateRange(r.Context(), org.ID, startDate, endDate)
	if err != nil {
		jsonErr(w, "db error", http.StatusInternalServerError)
		return
	}

	var (
		totalAccepted  int
		totalSuggested int
		copilotUsers   int
	)
	var totalAcceptances int
	for _, s := range copilotSnaps {
		totalAccepted += s.TotalLinesAccepted
		totalSuggested += s.TotalSuggestions
		totalAcceptances += s.TotalAcceptances
		if s.TotalActiveUsers > copilotUsers {
			copilotUsers = s.TotalActiveUsers
		}
	}

	// --- Tool snapshots (Claude Code, etc.) ---
	toolSnaps, err := h.queries.GetToolSnapshotsByDateRange(r.Context(), org.ID, startDate, endDate)
	if err != nil {
		jsonErr(w, "db error", http.StatusInternalServerError)
		return
	}

	var (
		totalLinesWritten int
		toolUsers         int
		planningUses      int
		codingUses        int
	)
	for _, s := range toolSnaps {
		totalLinesWritten += s.TotalLinesAccepted
		if s.TotalActiveUsers > toolUsers {
			toolUsers = s.TotalActiveUsers
		}
		planningUses += s.TotalPlanningUses
		codingUses += s.TotalCodingUses
	}

	// --- Aggregate ---
	activeDevelopers := copilotUsers
	if toolUsers > activeDevelopers {
		activeDevelopers = toolUsers
	}

	estimatedHoursSaved := float64(totalAccepted) / 30.0
	estimatedValueSaved := estimatedHoursSaved * float64(user.HourlyRate)

	// Acceptances per suggestion, as on the dashboard. (Accepted lines per
	// suggestion mixes units and can exceed 100%.)
	var acceptanceRate float64
	if totalSuggested > 0 {
		acceptanceRate = float64(totalAcceptances) / float64(totalSuggested) * 100
	}

	var planningRatio float64
	if planningUses+codingUses > 0 {
		planningRatio = float64(planningUses) / float64(planningUses+codingUses) * 100
	}

	var adoptionRate *float64
	if org.GithubMemberCount != nil && *org.GithubMemberCount > 0 {
		rate := float64(activeDevelopers) / float64(*org.GithubMemberCount) * 100
		adoptionRate = &rate
	}

	shareToken, err := generateShareToken()
	if err != nil {
		jsonErr(w, "internal error", http.StatusInternalServerError)
		return
	}

	content := ReportContent{
		TotalAcceptedLines:  totalAccepted,
		TotalLinesWritten:   totalLinesWritten,
		ActiveDevelopers:    activeDevelopers,
		EstimatedHoursSaved: estimatedHoursSaved,
		EstimatedValueSaved: estimatedValueSaved,
		AdoptionRate:        adoptionRate,
		GithubMemberCount:   org.GithubMemberCount,
		AcceptanceRate:      acceptanceRate,
		TotalSuggestedLines: totalSuggested,
		HourlyRate:          user.HourlyRate,
		PlanningToolUses:    planningUses,
		CodingToolUses:      codingUses,
		PlanningRatio:       planningRatio,
		StartDate:           body.StartDate,
		EndDate:             body.EndDate,
	}

	// Use "orgID:startDate:endDate" as the upsert key stored in the month column.
	upsertKey := org.ID + ":" + body.StartDate + ":" + body.EndDate

	report, err := h.queries.UpsertReportWithToken(r.Context(), org.ID, upsertKey, shareToken, content)
	if err != nil {
		jsonErr(w, "db error", http.StatusInternalServerError)
		return
	}

	jsonOK(w, toReportResponse(*report))
}
