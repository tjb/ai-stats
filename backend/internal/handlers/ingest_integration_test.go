package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tjb/ai-stats/internal/db"
	"github.com/tjb/ai-stats/internal/dbtest"
)

func (s *testServer) ingest(t *testing.T, headers map[string]string, body any) *httptest.ResponseRecorder {
	t.Helper()
	return s.requestWithHeaders(t, http.MethodPost, "/api/ingest/events", "", body, headers)
}

func (s *testServer) claudeCodeRows(t *testing.T, orgID string) []db.ToolUsageSnapshot {
	t.Helper()
	rows, err := s.queries.GetSnapshotsBySource(context.Background(), orgID, "claude_code", 30)
	if err != nil {
		t.Fatalf("GetSnapshotsBySource: %v", err)
	}
	return rows
}

func TestIngest_EndToEndWithRealKey(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)
	ctx := context.Background()
	user := dbtest.CreateUser(t, s.queries, "ingest@example.com")
	org := dbtest.CreateOrg(t, s.queries, user.ID, "ingest-org")
	other := dbtest.CreateUser(t, s.queries, "bystander@example.com")
	otherOrg := dbtest.CreateOrg(t, s.queries, other.ID, "bystander-org")
	token := s.login(t, user)
	key := s.createKey(t, token, "laptop")
	date := dbtest.Today(t, s.pool).AddDate(0, 0, -1)

	first := map[string]any{
		"tool_source": "claude_code",
		"date":        date.Format("2006-01-02"),
		"events": []map[string]any{
			{"type": "tool_use", "tool_name": "Edit", "lines_written": 5, "duration_minutes": 2, "user_email": "a@example.com"},
			{"type": "tool_use", "tool_name": "Write", "lines_written": 3, "duration_minutes": 1, "user_email": "b@example.com"},
			{"type": "tool_use", "tool_name": "Read", "user_email": "a@example.com"},
			{"type": "tool_use", "tool_name": "Bash", "user_email": "b@example.com"},
			{"type": "tool_use", "tool_name": "Frobnicate", "user_email": "c@example.com"},
			{"type": "session_end", "lines_written": 7, "duration_minutes": 4, "user_email": "a@example.com"},
			{"type": "session_start", "user_email": "d@example.com"}, // ignored entirely
		},
	}
	var accepted map[string]int
	decode(t, s.ingest(t, map[string]string{"X-AIStats-Key": key.Key}, first), http.StatusOK, &accepted)
	if accepted["accepted"] != 7 {
		t.Errorf("accepted = %v, want 7", accepted)
	}

	rows := s.claudeCodeRows(t, org.ID)
	if len(rows) != 1 {
		t.Fatalf("got %d claude_code rows, want 1", len(rows))
	}
	got := rows[0]
	if !got.Date.Equal(date) || got.TotalSuggestions != 6 || got.TotalAcceptances != 3 || got.TotalLinesAccepted != 15 ||
		got.TotalActiveUsers != 3 || got.TotalPlanningUses != 3 || got.TotalCodingUses != 2 ||
		got.TotalLicensedUsers != 0 || got.TotalCodingDevelopers != 0 {
		t.Errorf("row = %+v, want %s with 6 suggestions, 3 acceptances, 15 lines, 3 users, 3 planning, 2 coding",
			got, date.Format("2006-01-02"))
	}
	var meta map[string]any
	if err := json.Unmarshal(got.RawJSON, &meta); err != nil {
		t.Fatalf("raw_json: %v", err)
	}
	if meta["source"] != "push_ingest" || meta["total_duration_minutes"] != float64(7) {
		t.Errorf("raw_json = %s, want push_ingest with 7 minutes", got.RawJSON)
	}

	// Privacy: no raw email reaches the database.
	var leaks int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM tool_usage_snapshots WHERE raw_json::text LIKE '%@%'`).Scan(&leaks); err != nil {
		t.Fatal(err)
	}
	if leaks != 0 {
		t.Error("an email address was stored in tool_usage_snapshots.raw_json")
	}

	// A second batch for the same day (Bearer auth) accumulates counters and
	// keeps the larger distinct-user count.
	second := map[string]any{
		"tool_source": "claude_code",
		"date":        date.Format("2006-01-02"),
		"events": []map[string]any{
			{"type": "tool_use", "tool_name": "MultiEdit", "lines_written": 10, "duration_minutes": 3, "user_email": "e@example.com"},
		},
	}
	decode(t, s.ingest(t, map[string]string{"Authorization": "Bearer " + key.Key}, second), http.StatusOK, nil)
	rows = s.claudeCodeRows(t, org.ID)
	if len(rows) != 1 {
		t.Fatalf("got %d claude_code rows after second batch, want 1", len(rows))
	}
	got = rows[0]
	if got.TotalSuggestions != 7 || got.TotalAcceptances != 4 || got.TotalLinesAccepted != 25 ||
		got.TotalActiveUsers != 3 || got.TotalPlanningUses != 3 || got.TotalCodingUses != 3 {
		t.Errorf("accumulated row = %+v, want 7 suggestions, 4 acceptances, 25 lines, 3 users, 3 planning, 3 coding", got)
	}

	// last_used_at is written asynchronously after authentication.
	deadline := time.Now().Add(5 * time.Second)
	for {
		keys := s.listKeys(t, token)
		if len(keys) == 1 && keys[0].LastUsedAt != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("last_used_at never set: %+v", keys)
		}
		time.Sleep(20 * time.Millisecond)
	}

	// The pushed data shows up on the owner's dashboard and nowhere else.
	var dash dashboardBody
	decode(t, s.request(t, http.MethodGet, "/api/dashboard", token, nil), http.StatusOK, &dash)
	if dash.Metrics.LinesAccepted != 25 || dash.Metrics.ActiveUsers != 3 || !approx(dash.Metrics.AcceptanceRate, 57.1) {
		t.Errorf("dashboard metrics = %+v, want 25 lines, 3 users, 57.1%% (4/7)", dash.Metrics)
	}
	if rows := s.claudeCodeRows(t, otherOrg.ID); len(rows) != 0 {
		t.Errorf("bystander org got %d rows", len(rows))
	}

	// Revoking the key cuts off ingest immediately.
	decode(t, s.request(t, http.MethodDelete, "/api/settings/ingest-keys/"+key.ID, token, nil), http.StatusOK, nil)
	decode(t, s.ingest(t, map[string]string{"X-AIStats-Key": key.Key}, second), http.StatusUnauthorized, nil)
	if rows := s.claudeCodeRows(t, org.ID); rows[0].TotalSuggestions != 7 {
		t.Errorf("revoked key still wrote data: %+v", rows[0])
	}
}

func TestIngest_RejectsBadRequestsWithoutWriting(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)
	user := dbtest.CreateUser(t, s.queries, "ingest-bad@example.com")
	org := dbtest.CreateOrg(t, s.queries, user.ID, "ingest-bad-org")
	key := s.createKey(t, s.login(t, user), "ci")
	auth := map[string]string{"X-AIStats-Key": key.Key}
	date := dbtest.Today(t, s.pool).AddDate(0, 0, -1).Format("2006-01-02")
	event := map[string]any{"type": "tool_use", "tool_name": "Edit", "lines_written": 1, "user_email": "a@example.com"}
	payload := func(mut func(map[string]any)) map[string]any {
		p := map[string]any{"tool_source": "claude_code", "date": date, "events": []map[string]any{event}}
		mut(p)
		return p
	}

	cases := []struct {
		name    string
		headers map[string]string
		body    any
		want    int
	}{
		{"no key", nil, payload(func(map[string]any) {}), http.StatusUnauthorized},
		{"unknown key", map[string]string{"X-AIStats-Key": key.Key + "x"}, payload(func(map[string]any) {}), http.StatusUnauthorized},
		{"session cookie is not a key", map[string]string{"Authorization": "Bearer " + s.login(t, user)}, payload(func(map[string]any) {}), http.StatusUnauthorized},
		{"bad json", auth, "{", http.StatusBadRequest},
		{"no tool_source", auth, payload(func(p map[string]any) { delete(p, "tool_source") }), http.StatusBadRequest},
		{"no events", auth, payload(func(p map[string]any) { p["events"] = []any{} }), http.StatusBadRequest},
		{"bad date", auth, payload(func(p map[string]any) { p["date"] = "yesterday" }), http.StatusBadRequest},
		{"lines out of range", auth, payload(func(p map[string]any) {
			p["events"] = []map[string]any{{"type": "tool_use", "tool_name": "Edit", "lines_written": 100_001}}
		}), http.StatusBadRequest},
		{"negative duration", auth, payload(func(p map[string]any) {
			p["events"] = []map[string]any{{"type": "session_end", "duration_minutes": -1}}
		}), http.StatusBadRequest},
		{"body over 1 MB", auth, `{"tool_source":"claude_code","padding":"` + strings.Repeat("x", 1<<20) + `"}`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		if w := s.ingest(t, tc.headers, tc.body); w.Code != tc.want {
			t.Errorf("%s: status = %d, want %d; body: %s", tc.name, w.Code, tc.want, w.Body.String())
		}
	}
	if rows := s.claudeCodeRows(t, org.ID); len(rows) != 0 {
		t.Errorf("rejected requests wrote %d rows: %+v", len(rows), rows)
	}
}
