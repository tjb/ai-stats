package handlers_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/tjb/ai-stats/internal/db"
	"github.com/tjb/ai-stats/internal/dbtest"
)

type dashboardBody struct {
	Org struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		GitHubOrgLogin string `json:"github_org_login"`
	} `json:"org"`
	Metrics struct {
		ActiveUsers          int     `json:"active_users"`
		ActiveUsersChange    int     `json:"active_users_change"`
		AcceptanceRate       float64 `json:"acceptance_rate"`
		AcceptanceRateChange float64 `json:"acceptance_rate_change"`
		LinesAccepted        int     `json:"lines_accepted"`
		LinesAcceptedChange  int     `json:"lines_accepted_change"`
		HoursSaved           float64 `json:"hours_saved"`
		DollarValueSaved     float64 `json:"dollar_value_saved"`
		HourlyRate           int     `json:"hourly_rate"`
	} `json:"metrics"`
	Charts struct {
		AcceptanceRate []struct {
			Date string  `json:"date"`
			Rate float64 `json:"rate"`
		} `json:"acceptance_rate"`
		ActiveUsers []struct {
			Date  string `json:"date"`
			Users int    `json:"users"`
		} `json:"active_users"`
		Suggestions []struct {
			Date        string `json:"date"`
			Suggestions int    `json:"suggestions"`
			Acceptances int    `json:"acceptances"`
		} `json:"suggestions"`
		Languages []struct {
			Language string  `json:"language"`
			Rate     float64 `json:"rate"`
		} `json:"languages"`
	} `json:"charts"`
}

func TestDashboard_NoOrg(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)
	user := dbtest.CreateUser(t, s.queries, "lonely@example.com")

	var body errorBody
	decode(t, s.request(t, http.MethodGet, "/api/dashboard", s.login(t, user), nil), http.StatusNotFound, &body)
	if body.Error != "no_org" {
		t.Errorf("error = %q, want no_org", body.Error)
	}
}

func TestDashboard_EmptyOrgReturnsZeroedMetrics(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)
	user := dbtest.CreateUser(t, s.queries, "empty@example.com")
	org := dbtest.CreateOrg(t, s.queries, user.ID, "empty-org")

	var got dashboardBody
	decode(t, s.request(t, http.MethodGet, "/api/dashboard", s.login(t, user), nil), http.StatusOK, &got)
	if got.Org.ID != org.ID || got.Org.GitHubOrgLogin != "empty-org" {
		t.Errorf("org = %+v, want %s", got.Org, org.ID)
	}
	if got.Metrics.HourlyRate != 150 || got.Metrics.ActiveUsers != 0 || got.Metrics.AcceptanceRate != 0 {
		t.Errorf("metrics = %+v, want zeros with hourly rate 150", got.Metrics)
	}
	// Empty arrays, not null, so the frontend can map over them.
	raw := s.request(t, http.MethodGet, "/api/dashboard", s.login(t, user), nil).Body.String()
	for _, key := range []string{`"acceptance_rate":[]`, `"active_users":[]`, `"suggestions":[]`, `"languages":[]`} {
		if !strings.Contains(raw, key) {
			t.Errorf("body missing %s: %s", key, raw)
		}
	}
}

// Mixed Copilot + push-ingest data, including a day with zero suggestions and
// days with zero coding developers, so every division-by-zero guard runs.
func TestDashboard_MixedSourcesWithZeroDays(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)
	user := dbtest.CreateUser(t, s.queries, "dash@example.com")
	org := dbtest.CreateOrg(t, s.queries, user.ID, "dash-org")
	today := dbtest.Today(t, s.pool)
	day := func(offset int) string { return today.AddDate(0, 0, offset).Format("2006-01-02") }

	s.seedCopilot(t, org.ID,
		copilotDay{date: today.AddDate(0, 0, -3), suggestions: 200, acceptances: 50, lines: 120, activeUsers: 8,
			breakdown: []db.BreakdownItem{
				{Language: "Go", Editor: "vscode", SuggestionsCount: 120, AcceptancesCount: 30},
				{Language: "Python", Editor: "jetbrains", SuggestionsCount: 80, AcceptancesCount: 20},
			}},
		// Zero suggestions: per-day acceptance rate must be 0, not NaN.
		copilotDay{date: today.AddDate(0, 0, -2)},
		copilotDay{date: today.AddDate(0, 0, -1), suggestions: 100, acceptances: 45, lines: 90, activeUsers: 6,
			breakdown: []db.BreakdownItem{
				{Language: "TypeScript", Editor: "vscode", SuggestionsCount: 60, AcceptancesCount: 30},
				{Language: "", Editor: "neovim", SuggestionsCount: 40, AcceptancesCount: 15},
			}},
	)
	s.seedTool(t, org.ID,
		toolDay{source: "claude_code", date: today.AddDate(0, 0, -1), suggestions: 10, acceptances: 4, lines: 40,
			activeUsers: 3, planningUses: 6, codingUses: 4},
		// Push ingest is same-day while Copilot lags: today has tool data only.
		toolDay{source: "claude_code", date: today, activeUsers: 2},
	)

	var got dashboardBody
	decode(t, s.request(t, http.MethodGet, "/api/dashboard", s.login(t, user), nil), http.StatusOK, &got)

	m := got.Metrics
	// Totals over the 4 days: 310 suggestions, 99 acceptances, 250 lines.
	if m.ActiveUsers != 9 { // peak day: 6 Copilot + 3 Claude Code
		t.Errorf("active_users = %d, want 9", m.ActiveUsers)
	}
	if !approx(m.AcceptanceRate, 31.9) { // 99/310
		t.Errorf("acceptance_rate = %v, want 31.9", m.AcceptanceRate)
	}
	if m.LinesAccepted != 250 {
		t.Errorf("lines_accepted = %d, want 250", m.LinesAccepted)
	}
	if !approx(m.HoursSaved, 8.3) || !approx(m.DollarValueSaved, 1250) { // 250 lines * 2 min, $150/h
		t.Errorf("hours/dollars = %v/%v, want 8.3/1250", m.HoursSaved, m.DollarValueSaved)
	}
	if m.ActiveUsersChange != 0 || m.AcceptanceRateChange != 0 || m.LinesAcceptedChange != 0 {
		t.Errorf("changes = %d/%v/%d, want 0 with no prior period", m.ActiveUsersChange, m.AcceptanceRateChange, m.LinesAcceptedChange)
	}

	wantDates := []string{day(-3), day(-2), day(-1), day(0)}
	wantRates := []float64{25, 0, 44.5, 0}
	wantUsers := []int{8, 0, 9, 2}
	wantSugg := []int{200, 0, 110, 0}
	if len(got.Charts.AcceptanceRate) != len(wantDates) || len(got.Charts.ActiveUsers) != len(wantDates) ||
		len(got.Charts.Suggestions) != len(wantDates) {
		t.Fatalf("chart lengths = %d/%d/%d, want %d", len(got.Charts.AcceptanceRate),
			len(got.Charts.ActiveUsers), len(got.Charts.Suggestions), len(wantDates))
	}
	for i, d := range wantDates {
		ar, au, sg := got.Charts.AcceptanceRate[i], got.Charts.ActiveUsers[i], got.Charts.Suggestions[i]
		if ar.Date != d || !approx(ar.Rate, wantRates[i]) {
			t.Errorf("acceptance_rate[%d] = %+v, want {%s %v}", i, ar, d, wantRates[i])
		}
		if au.Date != d || au.Users != wantUsers[i] {
			t.Errorf("active_users[%d] = %+v, want {%s %d}", i, au, d, wantUsers[i])
		}
		if sg.Date != d || sg.Suggestions != wantSugg[i] {
			t.Errorf("suggestions[%d] = %+v, want {%s %d}", i, sg, d, wantSugg[i])
		}
	}

	// Languages come from the most recent day that has a Copilot breakdown
	// (yesterday), even though today only has push-ingest data.
	langs := got.Charts.Languages
	if len(langs) != 2 || langs[0].Language != "TypeScript" || !approx(langs[0].Rate, 50) ||
		langs[1].Language != "Other" || !approx(langs[1].Rate, 37.5) {
		t.Errorf("languages = %+v, want [TypeScript 50, Other 37.5]", langs)
	}
}

// More than 90 days of history: the dashboard must show the most recent 90
// days and compare the last 30 against the 30 before them.
func TestDashboard_UsesMostRecent90Days(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)
	user := dbtest.CreateUser(t, s.queries, "history@example.com")
	org := dbtest.CreateOrg(t, s.queries, user.ID, "history-org")
	today := dbtest.Today(t, s.pool)

	var days []copilotDay
	for offset := 99; offset >= 0; offset-- {
		d := copilotDay{date: today.AddDate(0, 0, -offset), suggestions: 100, lines: 10}
		switch {
		case offset < 30: // current period
			d.acceptances, d.activeUsers = 50, 20
		case offset < 60: // prior period
			d.acceptances, d.activeUsers = 25, 10
		default: // older history, outside both windows
			d.acceptances, d.activeUsers = 5, 99
		}
		days = append(days, d)
	}
	s.seedCopilot(t, org.ID, days...)

	var got dashboardBody
	decode(t, s.request(t, http.MethodGet, "/api/dashboard", s.login(t, user), nil), http.StatusOK, &got)

	pts := got.Charts.ActiveUsers
	if len(pts) != 90 {
		t.Fatalf("active_users chart has %d points, want 90", len(pts))
	}
	if first, last := pts[0].Date, pts[89].Date; first != today.AddDate(0, 0, -89).Format("2006-01-02") ||
		last != today.Format("2006-01-02") {
		t.Errorf("chart spans %s..%s, want the 90 days ending today", first, last)
	}

	m := got.Metrics
	if m.ActiveUsers != 20 || m.ActiveUsersChange != 100 {
		t.Errorf("active_users = %d (%+d%%), want 20 (+100%%)", m.ActiveUsers, m.ActiveUsersChange)
	}
	if !approx(m.AcceptanceRate, 50) || !approx(m.AcceptanceRateChange, 25) {
		t.Errorf("acceptance_rate = %v (%+v), want 50 (+25)", m.AcceptanceRate, m.AcceptanceRateChange)
	}
	if m.LinesAccepted != 300 || m.LinesAcceptedChange != 0 {
		t.Errorf("lines_accepted = %d (%+d%%), want 300 (+0%%)", m.LinesAccepted, m.LinesAcceptedChange)
	}
}

func TestDashboard_OnlyShowsCallersOrg(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)
	alice := dbtest.CreateUser(t, s.queries, "alice@example.com")
	bob := dbtest.CreateUser(t, s.queries, "bob@example.com")
	aliceOrg := dbtest.CreateOrg(t, s.queries, alice.ID, "alice-org")
	bobOrg := dbtest.CreateOrg(t, s.queries, bob.ID, "bob-org")
	today := dbtest.Today(t, s.pool)

	s.seedCopilot(t, aliceOrg.ID, copilotDay{date: today.AddDate(0, 0, -1), suggestions: 10, acceptances: 5, lines: 7, activeUsers: 3})
	s.seedCopilot(t, bobOrg.ID, copilotDay{date: today.AddDate(0, 0, -1), suggestions: 1000, acceptances: 100, lines: 900, activeUsers: 40})

	var got dashboardBody
	decode(t, s.request(t, http.MethodGet, "/api/dashboard", s.login(t, alice), nil), http.StatusOK, &got)
	if got.Org.ID != aliceOrg.ID || got.Metrics.LinesAccepted != 7 || got.Metrics.ActiveUsers != 3 {
		t.Errorf("alice sees org %s with %d lines / %d users, want her own org with 7 / 3",
			got.Org.ID, got.Metrics.LinesAccepted, got.Metrics.ActiveUsers)
	}
}
