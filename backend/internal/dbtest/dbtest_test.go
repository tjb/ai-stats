package dbtest_test

import (
	"context"
	"testing"

	"github.com/tjb/ai-stats/internal/db"
	"github.com/tjb/ai-stats/internal/dbtest"
)

func TestNew_MigratesFreshDatabase(t *testing.T) {
	pool, q := dbtest.New(t)
	ctx := context.Background()

	var applied int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&applied); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if applied == 0 {
		t.Fatal("no migrations recorded in schema_migrations")
	}

	// Re-running is a no-op, as on every server restart.
	if err := db.Migrate(pool); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}

	user := dbtest.CreateUser(t, q, "harness@example.com")
	org := dbtest.CreateOrg(t, q, user.ID, "harness-org")

	got, err := q.GetOrgByOwner(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetOrgByOwner: %v", err)
	}
	if got.ID != org.ID || got.GitHubAccessToken != "gho_org_token" {
		t.Errorf("org round trip = %+v, want id %s with decrypted token", got, org.ID)
	}

	// The token is encrypted at rest with the harness key.
	var stored string
	if err := pool.QueryRow(ctx, `SELECT github_access_token FROM organizations WHERE id = $1`, org.ID).Scan(&stored); err != nil {
		t.Fatalf("select token: %v", err)
	}
	if stored == "gho_org_token" {
		t.Error("github_access_token stored in plaintext")
	}
}

func TestNew_DatabasesAreIsolated(t *testing.T) {
	poolA, qA := dbtest.New(t)
	poolB, _ := dbtest.New(t)
	ctx := context.Background()

	var nameA, nameB string
	if err := poolA.QueryRow(ctx, `SELECT current_database()`).Scan(&nameA); err != nil {
		t.Fatal(err)
	}
	if err := poolB.QueryRow(ctx, `SELECT current_database()`).Scan(&nameB); err != nil {
		t.Fatal(err)
	}
	if nameA == nameB {
		t.Fatalf("both pools point at %s", nameA)
	}

	dbtest.CreateUser(t, qA, "only-in-a@example.com")

	var inB int
	if err := poolB.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&inB); err != nil {
		t.Fatal(err)
	}
	if inB != 0 {
		t.Errorf("database B has %d users, want 0", inB)
	}
}

func TestToday_MatchesCurrentDate(t *testing.T) {
	pool, _ := dbtest.New(t)

	today := dbtest.Today(t, pool)
	var same bool
	if err := pool.QueryRow(context.Background(), `SELECT $1::date = CURRENT_DATE`, today).Scan(&same); err != nil {
		t.Fatal(err)
	}
	if !same {
		t.Errorf("Today() = %s does not round-trip to CURRENT_DATE", today.Format("2006-01-02"))
	}
}
