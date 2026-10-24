package handlers_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/tjb/ai-stats/internal/db"
	"github.com/tjb/ai-stats/internal/dbtest"
)

type insightsBody struct {
	Languages []struct {
		Language    string  `json:"language"`
		Suggestions int     `json:"suggestions"`
		Acceptances int     `json:"acceptances"`
		Lines       int     `json:"lines"`
		Rate        float64 `json:"rate"`
	} `json:"languages"`
	Editors []struct {
		Editor      string  `json:"editor"`
		Suggestions int     `json:"suggestions"`
		Acceptances int     `json:"acceptances"`
		Users       int     `json:"users"`
		Rate        float64 `json:"rate"`
	} `json:"editors"`
	ActivityHeatmap []struct {
		Date  string `json:"date"`
		Users int    `json:"users"`
	} `json:"activity_heatmap"`
	UtilizationPercent    float64 `json:"utilization_percent"`
	AIActiveUsers         int     `json:"ai_active_users"`
	TotalCodingDevelopers int     `json:"total_coding_developers"`
	UtilizationTrend      []struct {
		Date               string  `json:"date"`
		UtilizationPercent float64 `json:"utilization_percent"`
	} `json:"utilization_trend"`
	UtilizationDeltaPP float64 `json:"utilization_delta_pp"`
	UtilizationPrevPct float64 `json:"utilization_prev_percent"`
	ToolBreakdown      []struct {
		ToolSource    string `json:"tool_source"`
		ActiveUsers   int    `json:"active_users"`
		LinesAccepted int    `json:"lines_accepted"`
	} `json:"tool_breakdown"`
	HoursSaved          float64 `json:"hours_saved"`
	DollarValueSaved    float64 `json:"dollar_value_saved"`
	HourlyRate          int     `json:"hourly_rate"`
	AcceptanceRate      float64 `json:"acceptance_rate"`
	LinesAccepted       int     `json:"lines_accepted"`
	WeeklyActiveUsers   int     `json:"weekly_active_users"`
	PrevWeekActiveUsers int     `json:"prev_week_active_users"`
	PlanningToolUses    int     `json:"planning_tool_uses"`
	CodingToolUses      int     `json:"coding_tool_uses"`
	PlanningRatio       float64 `json:"planning_ratio"`
	PrevPlanningRatio   float64 `json:"prev_planning_ratio"`
	PlanningTrend       []struct {
		Date             string  `json:"date"`
		PlanningRatioPct float64 `json:"planning_ratio_percent"`
		PlanningUses     int     `json:"planning_uses"`
		CodingUses       int     `json:"coding_uses"`
	} `json:"planning_trend"`
}

func TestInsights_NoOrg(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)
	user := dbtest.CreateUser(t, s.queries, "no-org-insights@example.com")

	var body errorBody
	decode(t, s.request(t, http.MethodGet, "/api/insights", s.login(t, user), nil), http.StatusNotFound, &body)
	if body.Error != "no_org" {
		t.Errorf("error = %q, want no_org", body.Error)
	}
}

func TestInsights_EmptyOrgReturnsEmptyArrays(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)
	user := dbtest.CreateUser(t, s.queries, "empty-insights@example.com")
	dbtest.CreateOrg(t, s.queries, user.ID, "empty-insights-org")

	w := s.request(t, http.MethodGet, "/api/insights", s.login(t, user), nil)
	var got insightsBody
	decode(t, w, http.StatusOK, &got)
	if got.AIActiveUsers != 0 || got.UtilizationPercent != 0 || got.HourlyRate != 150 {
		t.Errorf("got ai_active_users=%d utilization=%v hourly_rate=%d, want 0/0/150",
			got.AIActiveUsers, got.UtilizationPercent, got.HourlyRate)
	}
	raw := w.Body.String()
	for _, key := range []string{"languages", "editors", "activity_heatmap", "utilization_trend", "tool_breakdown", "planning_trend"} {
		if !strings.Contains(raw, `"`+key+`":[]`) {
			t.Errorf("%s is not an empty array: %s", key, raw)
		}
	}
}

// Copilot breakdowns plus two push-ingest tools, with a zero-suggestion day
// and a day with zero coding developers (the NULLIF path in the trend query).
func TestInsights_AggregatesAcrossSources(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)
	user := dbtest.CreateUser(t, s.queries, "insights@example.com")
	org := dbtest.CreateOrg(t, s.queries, user.ID, "insights-org")
	today := dbtest.Today(t, s.pool)
	day := func(offset int) string { return today.AddDate(0, 0, offset).Format("2006-01-02") }

	s.seedCopilot(t, org.ID,
		copilotDay{date: today.AddDate(0, 0, -1), suggestions: 100, acceptances: 40, lines: 80, activeUsers: 5,
			breakdown: []db.BreakdownItem{
				{Language: "TypeScript", Editor: "vscode", SuggestionsCount: 60, AcceptancesCount: 30, LinesAccepted: 50, ActiveUsers: 4},
				{Language: "Go", Editor: "jetbrains", SuggestionsCount: 40, AcceptancesCount: 10, LinesAccepted: 30, ActiveUsers: 2},
			}},
		// No suggestions at all; blank language/editor fall back to Other/Unknown.
		copilotDay{date: today.AddDate(0, 0, -2),
			breakdown: []db.BreakdownItem{{ActiveUsers: 1}}},
	)
	s.seedTool(t, org.ID,
		toolDay{source: "claude_code", date: today.AddDate(0, 0, -1), suggestions: 20, acceptances: 8, lines: 100,
			activeUsers: 4, codingDevelopers: 10, planningUses: 12, codingUses: 8},
		toolDay{source: "cursor", date: today.AddDate(0, 0, -1), lines: 30, activeUsers: 2, codingDevelopers: 8},
		// Zero coding developers: utilization for this day must be 0, not NULL.
		toolDay{source: "claude_code", date: today.AddDate(0, 0, -2), suggestions: 10, acceptances: 5, lines: 50,
			activeUsers: 4, planningUses: 5, codingUses: 5},
		toolDay{source: "claude_code", date: today.AddDate(0, 0, -9), suggestions: 4, acceptances: 2, lines: 10,
			activeUsers: 3, codingDevelopers: 10, planningUses: 2, codingUses: 2},
	)

	var got insightsBody
	decode(t, s.request(t, http.MethodGet, "/api/insights", s.login(t, user), nil), http.StatusOK, &got)

	// Languages, sorted by suggestions.
	if len(got.Languages) != 3 {
		t.Fatalf("languages = %+v, want 3 entries", got.Languages)
	}
	for i, want := range []struct {
		lang         string
		sugg, acc, l int
		rate         float64
	}{
		{"TypeScript", 60, 30, 50, 50},
		{"Go", 40, 10, 30, 25},
		{"Other", 0, 0, 0, 0},
	} {
		g := got.Languages[i]
		if g.Language != want.lang || g.Suggestions != want.sugg || g.Acceptances != want.acc ||
			g.Lines != want.l || !approx(g.Rate, want.rate) {
			t.Errorf("languages[%d] = %+v, want %+v", i, g, want)
		}
	}

	// Editors, sorted by users.
	if len(got.Editors) != 3 {
		t.Fatalf("editors = %+v, want 3 entries", got.Editors)
	}
	for i, want := range []struct {
		editor string
		users  int
		rate   float64
	}{
		{"vscode", 4, 50},
		{"jetbrains", 2, 25},
		{"Unknown", 1, 0},
	} {
		g := got.Editors[i]
		if g.Editor != want.editor || g.Users != want.users || !approx(g.Rate, want.rate) {
			t.Errorf("editors[%d] = %+v, want %+v", i, g, want)
		}
	}

	// Combined daily users (Copilot + every tool).
	wantHeat := []struct {
		date  string
		users int
	}{{day(-9), 3}, {day(-2), 4}, {day(-1), 11}}
	if len(got.ActivityHeatmap) != len(wantHeat) {
		t.Fatalf("activity_heatmap = %+v, want %d points", got.ActivityHeatmap, len(wantHeat))
	}
	for i, want := range wantHeat {
		if g := got.ActivityHeatmap[i]; g.Date != want.date || g.Users != want.users {
			t.Errorf("activity_heatmap[%d] = %+v, want %+v", i, g, want)
		}
	}

	// Utilization: peak day is yesterday with 4 + 2 tool users of 10 developers.
	if got.AIActiveUsers != 6 || got.TotalCodingDevelopers != 10 || !approx(got.UtilizationPercent, 60) {
		t.Errorf("utilization = %d of %d (%v%%), want 6 of 10 (60%%)",
			got.AIActiveUsers, got.TotalCodingDevelopers, got.UtilizationPercent)
	}
	if got.UtilizationPrevPct != 0 || got.UtilizationDeltaPP != 0 {
		t.Errorf("prior utilization = %v (delta %v), want 0 with no prior-window data",
			got.UtilizationPrevPct, got.UtilizationDeltaPP)
	}
	wantTrend := []struct {
		date string
		pct  float64
	}{{day(-9), 30}, {day(-2), 0}, {day(-1), 60}}
	if len(got.UtilizationTrend) != len(wantTrend) {
		t.Fatalf("utilization_trend = %+v, want %d points", got.UtilizationTrend, len(wantTrend))
	}
	for i, want := range wantTrend {
		if g := got.UtilizationTrend[i]; g.Date != want.date || !approx(g.UtilizationPercent, want.pct) {
			t.Errorf("utilization_trend[%d] = %+v, want %+v", i, g, want)
		}
	}

	// Per-tool: peak daily users, total lines.
	if len(got.ToolBreakdown) != 2 ||
		got.ToolBreakdown[0].ToolSource != "claude_code" || got.ToolBreakdown[0].ActiveUsers != 4 || got.ToolBreakdown[0].LinesAccepted != 160 ||
		got.ToolBreakdown[1].ToolSource != "cursor" || got.ToolBreakdown[1].ActiveUsers != 2 || got.ToolBreakdown[1].LinesAccepted != 30 {
		t.Errorf("tool_breakdown = %+v, want [claude_code 4 users/160 lines, cursor 2 users/30 lines]", got.ToolBreakdown)
	}

	// Totals: 55 acceptances / 134 suggestions, 270 lines at $150/h.
	if !approx(got.AcceptanceRate, 41) || got.LinesAccepted != 270 {
		t.Errorf("acceptance_rate/lines = %v/%d, want 41/270", got.AcceptanceRate, got.LinesAccepted)
	}
	if !approx(got.HoursSaved, 9) || !approx(got.DollarValueSaved, 1350) || got.HourlyRate != 150 {
		t.Errorf("hours/dollars/rate = %v/%v/%d, want 9/1350/150", got.HoursSaved, got.DollarValueSaved, got.HourlyRate)
	}

	// Largest single-source row in each 7-day window.
	if got.WeeklyActiveUsers != 5 || got.PrevWeekActiveUsers != 3 {
		t.Errorf("weekly active = %d (prev %d), want 5 (prev 3)", got.WeeklyActiveUsers, got.PrevWeekActiveUsers)
	}

	if got.PlanningToolUses != 19 || got.CodingToolUses != 15 ||
		!approx(got.PlanningRatio, 19.0/34.0*100) || got.PrevPlanningRatio != 0 {
		t.Errorf("planning = %d/%d ratio %v (prev %v), want 19/15 ratio %v (prev 0)",
			got.PlanningToolUses, got.CodingToolUses, got.PlanningRatio, got.PrevPlanningRatio, 19.0/34.0*100)
	}
	wantPlan := []struct {
		date        string
		pct         float64
		plan, coded int
	}{{day(-9), 50, 2, 2}, {day(-2), 50, 5, 5}, {day(-1), 60, 12, 8}}
	if len(got.PlanningTrend) != len(wantPlan) {
		t.Fatalf("planning_trend = %+v, want %d points", got.PlanningTrend, len(wantPlan))
	}
	for i, want := range wantPlan {
		g := got.PlanningTrend[i]
		if g.Date != want.date || !approx(g.PlanningRatioPct, want.pct) || g.PlanningUses != want.plan || g.CodingUses != want.coded {
			t.Errorf("planning_trend[%d] = %+v, want %+v", i, g, want)
		}
	}
}

// The same four developers active every day are four developers, not the sum
// of daily counts; the prior window is compared the same way.
func TestInsights_UtilizationCountsHeadcountNotUserDays(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)
	user := dbtest.CreateUser(t, s.queries, "headcount@example.com")
	org := dbtest.CreateOrg(t, s.queries, user.ID, "headcount-org")
	today := dbtest.Today(t, s.pool)

	for _, offset := range []int{-1, -2, -3} {
		s.seedTool(t, org.ID, toolDay{source: "claude_code", date: today.AddDate(0, 0, offset), activeUsers: 4, codingDevelopers: 10})
	}
	s.seedTool(t, org.ID,
		toolDay{source: "claude_code", date: today.AddDate(0, 0, -40), activeUsers: 1, codingDevelopers: 8},
		toolDay{source: "claude_code", date: today.AddDate(0, 0, -45), activeUsers: 2, codingDevelopers: 10},
	)

	var got insightsBody
	decode(t, s.request(t, http.MethodGet, "/api/insights", s.login(t, user), nil), http.StatusOK, &got)
	if got.AIActiveUsers != 4 || got.TotalCodingDevelopers != 10 || !approx(got.UtilizationPercent, 40) {
		t.Errorf("utilization = %d of %d (%v%%), want 4 of 10 (40%%)",
			got.AIActiveUsers, got.TotalCodingDevelopers, got.UtilizationPercent)
	}
	if !approx(got.UtilizationPrevPct, 20) || !approx(got.UtilizationDeltaPP, 20) {
		t.Errorf("prior = %v%%, delta = %v pp, want 20%% and +20 pp", got.UtilizationPrevPct, got.UtilizationDeltaPP)
	}
	if len(got.ToolBreakdown) != 1 || got.ToolBreakdown[0].ActiveUsers != 4 {
		t.Errorf("tool_breakdown = %+v, want claude_code with 4 users", got.ToolBreakdown)
	}
}
