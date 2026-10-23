package db

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tjb/ai-stats/internal/crypto"
)

// newTestQueries connects to TEST_DATABASE_URL, applies migrations and returns
// Queries plus a unique suffix for test rows. Skips when the variable is unset.
func newTestQueries(t *testing.T) (*Queries, string) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping database test")
	}
	pool, err := NewPool(dsn)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := Migrate(pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	key := make([]byte, 32)
	return New(pool, key), time.Now().Format("150405.000000000")
}

func TestListSyncableOrgs(t *testing.T) {
	q, suffix := newTestQueries(t)
	ctx := context.Background()

	user, err := q.UpsertUser(ctx, "sched-"+suffix+"@example.com", "Sched", "", "github", "sched-"+suffix, "tok")
	if err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	t.Cleanup(func() { _ = q.DeleteUser(context.Background(), user.ID) })

	good, err := q.CreateOrg(ctx, "gh-good-"+suffix, "acme-"+suffix, "Acme", nil, "real-token", user.ID)
	if err != nil {
		t.Fatalf("CreateOrg: %v", err)
	}
	demo, err := q.CreateOrg(ctx, "gh-demo-"+suffix, "demo-"+suffix, "Demo", nil, "demo-token", user.ID)
	if err != nil {
		t.Fatalf("CreateOrg demo: %v", err)
	}
	// A token encrypted with a different key cannot be decrypted.
	otherKey := make([]byte, 32)
	otherKey[0] = 1
	badToken, err := crypto.Encrypt("x", otherKey)
	if err != nil {
		t.Fatal(err)
	}
	var badID string
	if err := q.pool.QueryRow(ctx, `
		INSERT INTO organizations (github_org_id, github_org_login, name, github_access_token, owner_id)
		VALUES ($1, $2, 'Bad', $3, $4) RETURNING id
	`, "gh-bad-"+suffix, "bad-"+suffix, badToken, user.ID).Scan(&badID); err != nil {
		t.Fatalf("insert bad org: %v", err)
	}
	if _, err := q.pool.Exec(ctx, `
		INSERT INTO organizations (github_org_id, github_org_login, name, github_access_token, owner_id)
		VALUES ($1, $2, 'Empty', '', $3)
	`, "gh-empty-"+suffix, "empty-"+suffix, user.ID); err != nil {
		t.Fatalf("insert tokenless org: %v", err)
	}

	orgs, err := q.ListSyncableOrgs(ctx)
	if err == nil || !strings.Contains(err.Error(), badID) {
		t.Errorf("err = %v, want a decrypt error naming org %s", err, badID)
	}
	byID := map[string]Organization{}
	for _, o := range orgs {
		if o.OwnerID == user.ID {
			byID[o.ID] = o
		}
	}
	if len(byID) != 2 {
		t.Fatalf("listed %d of this test's orgs, want 2 (good + demo): %+v", len(byID), byID)
	}
	if got := byID[good.ID].GitHubAccessToken; got != "real-token" {
		t.Errorf("good org token = %q, want decrypted %q", got, "real-token")
	}
	if _, ok := byID[demo.ID]; !ok {
		t.Error("demo org missing; the scheduler, not the query, skips demo orgs")
	}
}

func TestGetClaudeCodeAttributedSHAs(t *testing.T) {
	q, suffix := newTestQueries(t)
	ctx := context.Background()

	user, err := q.UpsertUser(ctx, "sha-"+suffix+"@example.com", "Sha", "", "github", "sha-"+suffix, "tok")
	if err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	t.Cleanup(func() { _ = q.DeleteUser(context.Background(), user.ID) })
	org, err := q.CreateOrg(ctx, "gh-sha-"+suffix, "sha-"+suffix, "Sha", nil, "tok", user.ID)
	if err != nil {
		t.Fatalf("CreateOrg: %v", err)
	}

	day1 := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	day2 := day1.AddDate(0, 0, 1)
	day3 := day1.AddDate(0, 0, 2)
	upsert := func(date time.Time, raw string) {
		t.Helper()
		if err := q.UpsertToolSnapshot(ctx, org.ID, "claude_code", date, 0, 1, 1, 1, 0, 1, 0, 0, json.RawMessage(raw)); err != nil {
			t.Fatalf("UpsertToolSnapshot: %v", err)
		}
	}
	upsert(day1, `{"attributed_commits":["a1","a2"],"confidence":"medium"}`)
	upsert(day2, `{"source":"push_ingest","confidence":"high"}`)

	for _, tt := range []struct {
		name string
		date time.Time
		want string
	}{
		{"row with SHAs", day1, "a1,a2"},
		{"row written by push ingest", day2, ""},
		{"no row", day3, ""},
	} {
		got, err := q.GetClaudeCodeAttributedSHAs(ctx, org.ID, tt.date)
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		if got == nil || strings.Join(got, ",") != tt.want {
			t.Errorf("%s: got %#v, want %q", tt.name, got, tt.want)
		}
	}
}
