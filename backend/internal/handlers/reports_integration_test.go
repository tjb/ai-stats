package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/tjb/ai-stats/internal/dbtest"
)

type reportBody struct {
	ID         string  `json:"id"`
	Month      string  `json:"month"`
	StartDate  string  `json:"start_date"`
	EndDate    string  `json:"end_date"`
	ShareToken *string `json:"share_token"`
	CreatedAt  string  `json:"created_at"`
	Content    struct {
		TotalAcceptedLines  int      `json:"total_accepted_lines"`
		TotalLinesWritten   int      `json:"total_lines_written"`
		ActiveDevelopers    int      `json:"active_developers"`
		EstimatedHoursSaved float64  `json:"estimated_hours_saved"`
		EstimatedValueSaved float64  `json:"estimated_value_saved"`
		AdoptionRate        *float64 `json:"adoption_rate"`
		GithubMemberCount   *int     `json:"github_member_count"`
		AcceptanceRate      float64  `json:"acceptance_rate"`
		TotalSuggestedLines int      `json:"total_suggested_lines"`
		HourlyRate          int      `json:"hourly_rate"`
		PlanningToolUses    int      `json:"planning_tool_uses"`
		CodingToolUses      int      `json:"coding_tool_uses"`
		PlanningRatio       float64  `json:"planning_ratio"`
		StartDate           string   `json:"start_date"`
		EndDate             string   `json:"end_date"`
	} `json:"content"`
}

func TestReports_GenerateAndFetchByShareToken(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)
	ctx := context.Background()
	user := dbtest.CreateUser(t, s.queries, "reports@example.com")
	org := dbtest.CreateOrg(t, s.queries, user.ID, "reports-org")
	if err := s.queries.UpdateUserHourlyRate(ctx, user.ID, 120); err != nil {
		t.Fatal(err)
	}
	if err := s.queries.UpdateOrgMemberCount(ctx, org.ID, 22); err != nil {
		t.Fatal(err)
	}
	token := s.login(t, user)
	today := dbtest.Today(t, s.pool)
	start, end := today.AddDate(0, 0, -10), today.AddDate(0, 0, -1)

	s.seedCopilot(t, org.ID,
		copilotDay{date: start.AddDate(0, 0, -1), suggestions: 1000, acceptances: 500, lines: 999, activeUsers: 50}, // before range
		copilotDay{date: start, suggestions: 200, acceptances: 50, lines: 90, activeUsers: 7},
		copilotDay{date: today.AddDate(0, 0, -3), suggestions: 100, acceptances: 20, lines: 60, activeUsers: 9},
	)
	s.seedTool(t, org.ID,
		toolDay{source: "claude_code", date: today.AddDate(0, 0, -4), lines: 40, activeUsers: 3, planningUses: 6, codingUses: 2},
		toolDay{source: "claude_code", date: end, lines: 10, activeUsers: 11, codingUses: 2},
		toolDay{source: "claude_code", date: end.AddDate(0, 0, 1), lines: 500, activeUsers: 40, planningUses: 100, codingUses: 100}, // after range
	)

	reqBody := map[string]string{"start_date": start.Format("2006-01-02"), "end_date": end.Format("2006-01-02")}
	var gen reportBody
	decode(t, s.request(t, http.MethodPost, "/api/reports/generate", token, reqBody), http.StatusOK, &gen)

	if gen.ID == "" || gen.ShareToken == nil || *gen.ShareToken == "" {
		t.Fatalf("generated report = %+v, want id and share token", gen)
	}
	if gen.StartDate != reqBody["start_date"] || gen.EndDate != reqBody["end_date"] {
		t.Errorf("dates = %s..%s, want %s..%s", gen.StartDate, gen.EndDate, reqBody["start_date"], reqBody["end_date"])
	}
	c := gen.Content
	if c.TotalAcceptedLines != 150 || c.TotalSuggestedLines != 300 || c.TotalLinesWritten != 50 {
		t.Errorf("lines accepted/suggested/written = %d/%d/%d, want 150/300/50",
			c.TotalAcceptedLines, c.TotalSuggestedLines, c.TotalLinesWritten)
	}
	if c.ActiveDevelopers != 11 { // peak of Copilot (9) and tool (11) daily users
		t.Errorf("active_developers = %d, want 11", c.ActiveDevelopers)
	}
	if !approx(c.EstimatedHoursSaved, 5) || !approx(c.EstimatedValueSaved, 600) || c.HourlyRate != 120 {
		t.Errorf("hours/value/rate = %v/%v/%d, want 5/600/120", c.EstimatedHoursSaved, c.EstimatedValueSaved, c.HourlyRate)
	}
	// Acceptance rate is acceptances per suggestion (70/300), like the dashboard.
	if !approx(c.AcceptanceRate, 70.0/300.0*100) {
		t.Errorf("acceptance_rate = %v, want %v", c.AcceptanceRate, 70.0/300.0*100)
	}
	// Adoption is active developers over GitHub members: 11 of 22.
	if c.GithubMemberCount == nil || *c.GithubMemberCount != 22 || c.AdoptionRate == nil || !approx(*c.AdoptionRate, 50) {
		t.Errorf("adoption = %v%% of %v members, want 50%% of 22", deref(c.AdoptionRate), deref(c.GithubMemberCount))
	}
	if c.PlanningToolUses != 6 || c.CodingToolUses != 4 || !approx(c.PlanningRatio, 60) {
		t.Errorf("planning/coding/ratio = %d/%d/%v, want 6/4/60", c.PlanningToolUses, c.CodingToolUses, c.PlanningRatio)
	}

	// The share link is public: no session cookie.
	var shared reportBody
	decode(t, s.request(t, http.MethodGet, "/api/reports/share/"+*gen.ShareToken, "", nil), http.StatusOK, &shared)
	if shared.ID != gen.ID || !reflect.DeepEqual(shared.Content, gen.Content) {
		t.Errorf("shared report = %+v, want %+v", shared, gen)
	}

	var list []reportBody
	decode(t, s.request(t, http.MethodGet, "/api/reports", token, nil), http.StatusOK, &list)
	if len(list) != 1 || list[0].ID != gen.ID {
		t.Errorf("list = %+v, want just %s", list, gen.ID)
	}

	// Regenerating the same range updates the row in place and rotates the
	// token, which revokes the old link.
	var regen reportBody
	decode(t, s.request(t, http.MethodPost, "/api/reports/generate", token, reqBody), http.StatusOK, &regen)
	if regen.ID != gen.ID || regen.ShareToken == nil || *regen.ShareToken == *gen.ShareToken {
		t.Errorf("regenerated = id %s token %v, want id %s with a new token", regen.ID, regen.ShareToken, gen.ID)
	}
	decode(t, s.request(t, http.MethodGet, "/api/reports/share/"+*gen.ShareToken, "", nil), http.StatusNotFound, nil)
	decode(t, s.request(t, http.MethodGet, "/api/reports/share/"+*regen.ShareToken, "", nil), http.StatusOK, nil)
}

func TestReports_ShareTokenNotFound(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)

	var body errorBody
	decode(t, s.request(t, http.MethodGet, "/api/reports/share/does-not-exist", "", nil), http.StatusNotFound, &body)
	if body.Error != "not found" {
		t.Errorf("error = %q, want not found", body.Error)
	}
}

func TestReports_GenerateValidation(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)
	user := dbtest.CreateUser(t, s.queries, "report-validation@example.com")
	token := s.login(t, user)

	// No org yet: valid input still has nothing to report on.
	decode(t, s.request(t, http.MethodPost, "/api/reports/generate", token,
		map[string]string{"start_date": "2026-01-01", "end_date": "2026-01-31"}), http.StatusNotFound, nil)

	dbtest.CreateOrg(t, s.queries, user.ID, "report-validation-org")
	cases := []struct {
		name string
		body any
	}{
		{"not json", "{"},
		{"missing dates", map[string]string{}},
		{"bad start", map[string]string{"start_date": "01/01/2026", "end_date": "2026-01-31"}},
		{"bad end", map[string]string{"start_date": "2026-01-01", "end_date": "soon"}},
		{"end before start", map[string]string{"start_date": "2026-02-01", "end_date": "2026-01-31"}},
		{"over a year", map[string]string{"start_date": "2025-01-01", "end_date": "2026-01-03"}},
	}
	for _, tc := range cases {
		if w := s.request(t, http.MethodPost, "/api/reports/generate", token, tc.body); w.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400; body: %s", tc.name, w.Code, w.Body.String())
		}
	}

	// An empty range is valid and yields zeros, not NaN or nulls.
	var empty reportBody
	decode(t, s.request(t, http.MethodPost, "/api/reports/generate", token,
		map[string]string{"start_date": "2026-01-01", "end_date": "2026-01-31"}), http.StatusOK, &empty)
	if empty.Content.AcceptanceRate != 0 || empty.Content.PlanningRatio != 0 || empty.Content.AdoptionRate != nil {
		raw, _ := json.Marshal(empty.Content)
		t.Errorf("empty report content = %s, want zero rates and null adoption", raw)
	}
}

func deref[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}
