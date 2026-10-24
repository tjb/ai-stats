package handlers_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/tjb/ai-stats/internal/dbtest"
	"github.com/tjb/ai-stats/internal/ingest"
)

type createdKey struct {
	ID        string `json:"id"`
	Key       string `json:"key"`
	Label     string `json:"label"`
	CreatedAt string `json:"created_at"`
}

type listedKey struct {
	ID         string     `json:"id"`
	Label      string     `json:"label"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

func (s *testServer) createKey(t *testing.T, token, label string) createdKey {
	t.Helper()
	var k createdKey
	decode(t, s.request(t, http.MethodPost, "/api/settings/ingest-keys", token, map[string]string{"label": label}), http.StatusOK, &k)
	return k
}

func (s *testServer) listKeys(t *testing.T, token string) []listedKey {
	t.Helper()
	var keys []listedKey
	decode(t, s.request(t, http.MethodGet, "/api/settings/ingest-keys", token, nil), http.StatusOK, &keys)
	return keys
}

func TestIngestKeys_CreateListDelete(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)
	ctx := context.Background()
	user := dbtest.CreateUser(t, s.queries, "keys@example.com")
	org := dbtest.CreateOrg(t, s.queries, user.ID, "keys-org")
	token := s.login(t, user)

	if keys := s.listKeys(t, token); len(keys) != 0 {
		t.Fatalf("new org has keys: %+v", keys)
	}

	ci := s.createKey(t, token, "  CI runner  ")
	if ci.ID == "" || ci.Key == "" || ci.Label != "CI runner" {
		t.Errorf("created key = %+v, want id, raw key and trimmed label", ci)
	}
	if _, err := time.Parse(time.RFC3339, ci.CreatedAt); err != nil {
		t.Errorf("created_at %q is not RFC 3339 like the list endpoint: %v", ci.CreatedAt, err)
	}
	// Only the hash is stored, and it resolves back to this org.
	stored, err := s.queries.GetIngestKeyByHash(ctx, ingest.HashKey(ci.Key))
	if err != nil || stored.ID != ci.ID || stored.OrganizationID != org.ID {
		t.Fatalf("lookup by hash = %+v, %v; want key %s for org %s", stored, err, ci.ID, org.ID)
	}
	var rawStored int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM ingest_api_keys WHERE key_hash = $1 OR label = $1`, ci.Key).Scan(&rawStored); err != nil {
		t.Fatal(err)
	}
	if rawStored != 0 {
		t.Error("raw key found in ingest_api_keys")
	}

	unnamed := s.createKey(t, token, "   ")
	if unnamed.Label != "My key" {
		t.Errorf("blank label = %q, want default \"My key\"", unnamed.Label)
	}
	if unnamed.Key == ci.Key {
		t.Error("two keys share the same raw value")
	}

	w := s.request(t, http.MethodGet, "/api/settings/ingest-keys", token, nil)
	for _, secret := range []string{ci.Key, "key_hash", "organization_id", org.ID} {
		if strings.Contains(w.Body.String(), secret) {
			t.Errorf("list response leaks %q: %s", secret, w.Body.String())
		}
	}
	keys := s.listKeys(t, token)
	if len(keys) != 2 || keys[0].ID != ci.ID || keys[1].ID != unnamed.ID {
		t.Fatalf("list = %+v, want [%s %s] in creation order", keys, ci.ID, unnamed.ID)
	}
	if keys[0].Label != "CI runner" || keys[0].LastUsedAt != nil || keys[0].CreatedAt.IsZero() {
		t.Errorf("listed key = %+v, want label CI runner, never used", keys[0])
	}

	decode(t, s.request(t, http.MethodPost, "/api/settings/ingest-keys", token, "{not json"), http.StatusBadRequest, nil)

	var ok map[string]bool
	decode(t, s.request(t, http.MethodDelete, "/api/settings/ingest-keys/"+ci.ID, token, nil), http.StatusOK, &ok)
	if !ok["ok"] {
		t.Errorf("delete response = %v, want ok", ok)
	}
	if keys := s.listKeys(t, token); len(keys) != 1 || keys[0].ID != unnamed.ID {
		t.Errorf("after delete list = %+v, want only %s", keys, unnamed.ID)
	}
	if _, err := s.queries.GetIngestKeyByHash(ctx, ingest.HashKey(ci.Key)); err == nil {
		t.Error("deleted key still resolves")
	}

	// Deleting again, or an id that never existed, is a 404.
	decode(t, s.request(t, http.MethodDelete, "/api/settings/ingest-keys/"+ci.ID, token, nil), http.StatusNotFound, nil)
	decode(t, s.request(t, http.MethodDelete, "/api/settings/ingest-keys/no-such-key", token, nil), http.StatusNotFound, nil)
}

func TestIngestKeys_CannotDeleteAnotherOrgsKey(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)
	alice := dbtest.CreateUser(t, s.queries, "alice-keys@example.com")
	mallory := dbtest.CreateUser(t, s.queries, "mallory-keys@example.com")
	dbtest.CreateOrg(t, s.queries, alice.ID, "alice-keys-org")
	dbtest.CreateOrg(t, s.queries, mallory.ID, "mallory-keys-org")
	aliceToken, malloryToken := s.login(t, alice), s.login(t, mallory)

	aliceKey := s.createKey(t, aliceToken, "alice prod")
	malloryKey := s.createKey(t, malloryToken, "mallory")

	var body errorBody
	decode(t, s.request(t, http.MethodDelete, "/api/settings/ingest-keys/"+aliceKey.ID, malloryToken, nil), http.StatusNotFound, &body)
	if body.Error != "not found" {
		t.Errorf("error = %q, want not found", body.Error)
	}

	// Alice's key survives and still authenticates; each org sees only its own.
	if keys := s.listKeys(t, aliceToken); len(keys) != 1 || keys[0].ID != aliceKey.ID {
		t.Errorf("alice's keys = %+v, want [%s]", keys, aliceKey.ID)
	}
	if keys := s.listKeys(t, malloryToken); len(keys) != 1 || keys[0].ID != malloryKey.ID {
		t.Errorf("mallory's keys = %+v, want [%s]", keys, malloryKey.ID)
	}
	if _, err := s.queries.GetIngestKeyByHash(context.Background(), ingest.HashKey(aliceKey.Key)); err != nil {
		t.Errorf("alice's key no longer resolves: %v", err)
	}
}

func TestIngestKeys_RequireOrg(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)
	user := dbtest.CreateUser(t, s.queries, "keys-no-org@example.com")
	token := s.login(t, user)

	decode(t, s.request(t, http.MethodGet, "/api/settings/ingest-keys", token, nil), http.StatusNotFound, nil)
	decode(t, s.request(t, http.MethodPost, "/api/settings/ingest-keys", token, map[string]string{"label": "x"}), http.StatusNotFound, nil)
	decode(t, s.request(t, http.MethodDelete, "/api/settings/ingest-keys/anything", token, nil), http.StatusNotFound, nil)
}

func TestSettings_HourlyRateFlowsIntoROI(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)
	user := dbtest.CreateUser(t, s.queries, "rate@example.com")
	org := dbtest.CreateOrg(t, s.queries, user.ID, "rate-org")
	token := s.login(t, user)
	s.seedCopilot(t, org.ID, copilotDay{date: dbtest.Today(t, s.pool).AddDate(0, 0, -1), suggestions: 10, acceptances: 5, lines: 60, activeUsers: 1})

	for _, bad := range []any{map[string]int{"hourly_rate": 0}, map[string]int{"hourly_rate": 10001}, "nope"} {
		decode(t, s.request(t, http.MethodPost, "/api/settings/hourly-rate", token, bad), http.StatusBadRequest, nil)
	}
	decode(t, s.request(t, http.MethodPost, "/api/settings/hourly-rate", token, map[string]int{"hourly_rate": 90}), http.StatusOK, nil)

	var settings struct {
		User struct {
			Email      string `json:"email"`
			HourlyRate int    `json:"hourly_rate"`
		} `json:"user"`
		Org *struct {
			ID string `json:"id"`
		} `json:"org"`
	}
	decode(t, s.request(t, http.MethodGet, "/api/settings", token, nil), http.StatusOK, &settings)
	if settings.User.HourlyRate != 90 || settings.User.Email != user.Email || settings.Org == nil || settings.Org.ID != org.ID {
		t.Errorf("settings = %+v, want rate 90 and org %s", settings, org.ID)
	}

	// 60 lines * 2 min = 2 h, at $90/h.
	var dash dashboardBody
	decode(t, s.request(t, http.MethodGet, "/api/dashboard", token, nil), http.StatusOK, &dash)
	if dash.Metrics.HourlyRate != 90 || !approx(dash.Metrics.HoursSaved, 2) || !approx(dash.Metrics.DollarValueSaved, 180) {
		t.Errorf("dashboard ROI = %+v, want 2 h worth $180 at $90/h", dash.Metrics)
	}
}

func TestOnboarding_CompleteIsRemembered(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)
	user := dbtest.CreateUser(t, s.queries, "onboarding@example.com")
	token := s.login(t, user)

	var state map[string]bool
	decode(t, s.request(t, http.MethodGet, "/api/onboarding", token, nil), http.StatusOK, &state)
	if state["completed"] {
		t.Fatal("new user is already onboarded")
	}
	decode(t, s.request(t, http.MethodPost, "/api/onboarding/complete", token,
		map[string]any{"tools": []string{"claude_code"}, "github_org_login": "acme"}), http.StatusOK, nil)
	decode(t, s.request(t, http.MethodGet, "/api/onboarding", token, nil), http.StatusOK, &state)
	if !state["completed"] {
		t.Error("onboarding not marked complete")
	}
}
