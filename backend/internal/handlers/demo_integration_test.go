package handlers_test

import (
	"net/http"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/tjb/ai-stats/internal/dbtest"
)

func TestDemo_ActivateThenDashboardShowsDemoData(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)
	user := dbtest.CreateUser(t, s.queries, "demo@example.com")
	token := s.login(t, user)

	// Before activation there is nothing to show.
	decode(t, s.request(t, http.MethodGet, "/api/dashboard", token, nil), http.StatusNotFound, nil)

	var activated map[string]string
	decode(t, s.request(t, http.MethodPost, "/api/demo/activate", token, nil), http.StatusOK, &activated)
	orgID := activated["org_id"]
	if orgID == "" {
		t.Fatalf("activate response = %v, want org_id", activated)
	}

	var dash dashboardBody
	decode(t, s.request(t, http.MethodGet, "/api/dashboard", token, nil), http.StatusOK, &dash)
	if dash.Org.ID != orgID || dash.Org.Name != "Acme Corp (Demo)" || dash.Org.GitHubOrgLogin != "demo-"+user.ID {
		t.Errorf("org = %+v, want demo org %s", dash.Org, orgID)
	}

	// 90 consecutive days of seeded data ending today (±1 day for the Go vs
	// database clock/timezone).
	pts := dash.Charts.ActiveUsers
	if len(pts) != 90 {
		t.Fatalf("active_users chart has %d points, want 90", len(pts))
	}
	for i := 1; i < len(pts); i++ {
		prev, _ := time.Parse("2006-01-02", pts[i-1].Date)
		cur, _ := time.Parse("2006-01-02", pts[i].Date)
		if cur.Sub(prev) != 24*time.Hour {
			t.Fatalf("chart dates not consecutive at %s -> %s", pts[i-1].Date, pts[i].Date)
		}
	}
	last, _ := time.Parse("2006-01-02", pts[89].Date)
	if gap := dbtest.Today(t, s.pool).Sub(last); gap < -24*time.Hour || gap > 24*time.Hour {
		t.Errorf("demo data ends %s, want about today", pts[89].Date)
	}
	for _, p := range pts {
		if p.Users < 40 || p.Users > 54 {
			t.Errorf("%s: %d active users, want 40-54", p.Date, p.Users)
		}
	}

	m := dash.Metrics
	if m.ActiveUsers < 40 || m.ActiveUsers > 54 {
		t.Errorf("active_users = %d, want 40-54", m.ActiveUsers)
	}
	if m.AcceptanceRate < 32 || m.AcceptanceRate > 47 {
		t.Errorf("acceptance_rate = %v, want 32-47", m.AcceptanceRate)
	}
	if m.LinesAccepted <= 0 || m.HoursSaved <= 0 || m.DollarValueSaved <= 0 || m.HourlyRate != 150 {
		t.Errorf("metrics = %+v, want positive lines/hours/value at $150/h", m)
	}

	var langs []string
	for _, l := range dash.Charts.Languages {
		langs = append(langs, l.Language)
		if l.Rate <= 0 || l.Rate > 100 {
			t.Errorf("language %s rate = %v, want (0, 100]", l.Language, l.Rate)
		}
	}
	sort.Strings(langs)
	if want := []string{"Go", "Python", "Rust", "TypeScript"}; !slices.Equal(langs, want) {
		t.Errorf("languages = %v, want %v", langs, want)
	}

	// Activating again is idempotent: same org, no duplicate rows.
	var again map[string]string
	decode(t, s.request(t, http.MethodPost, "/api/demo/activate", token, nil), http.StatusOK, &again)
	if again["org_id"] != orgID {
		t.Errorf("second activate org_id = %s, want %s", again["org_id"], orgID)
	}
	var rows int
	if err := s.pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM copilot_usage_snapshots WHERE organization_id = $1`, orgID).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 90 {
		t.Errorf("demo org has %d snapshots, want 90", rows)
	}

	// Insights renders the demo's Copilot breakdown too.
	var ins insightsBody
	decode(t, s.request(t, http.MethodGet, "/api/insights", token, nil), http.StatusOK, &ins)
	if len(ins.Languages) != 4 || len(ins.Editors) != 2 || len(ins.ActivityHeatmap) != 30 || ins.LinesAccepted <= 0 {
		t.Errorf("insights after demo: %d languages, %d editors, %d heatmap days, %d lines; want 4, 2, 30, >0",
			len(ins.Languages), len(ins.Editors), len(ins.ActivityHeatmap), ins.LinesAccepted)
	}
}

func TestDemo_EachUserGetsTheirOwnOrg(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)
	alice := dbtest.CreateUser(t, s.queries, "demo-alice@example.com")
	bob := dbtest.CreateUser(t, s.queries, "demo-bob@example.com")

	var a, b map[string]string
	decode(t, s.request(t, http.MethodPost, "/api/demo/activate", s.login(t, alice), nil), http.StatusOK, &a)
	decode(t, s.request(t, http.MethodPost, "/api/demo/activate", s.login(t, bob), nil), http.StatusOK, &b)
	if a["org_id"] == "" || a["org_id"] == b["org_id"] {
		t.Errorf("alice org %q, bob org %q; want two distinct demo orgs", a["org_id"], b["org_id"])
	}

	// A user who already has a real org keeps it.
	carol := dbtest.CreateUser(t, s.queries, "demo-carol@example.com")
	carolOrg := dbtest.CreateOrg(t, s.queries, carol.ID, "carol-real-org")
	var c map[string]string
	decode(t, s.request(t, http.MethodPost, "/api/demo/activate", s.login(t, carol), nil), http.StatusOK, &c)
	if c["org_id"] != carolOrg.ID {
		t.Errorf("carol's activate returned %q, want her existing org %s", c["org_id"], carolOrg.ID)
	}
}
