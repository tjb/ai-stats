// Package dbtest provides a real-Postgres harness for integration tests.
//
// Point TEST_DATABASE_URL at a Postgres server the test role can CREATE
// DATABASE on, e.g.
//
//	TEST_DATABASE_URL=postgres://aistats:aistats@localhost:5432/aistats_test?sslmode=disable
//
// Each call to New creates a fresh, fully migrated database (dbtest_<random>)
// and drops it when the test finishes, so tests never see each other's rows and
// may run in parallel. When TEST_DATABASE_URL is unset the calling test is
// skipped, which keeps `go test ./...` green on machines without Postgres.
//
// If a test binary is killed before its cleanup runs, leftover databases can be
// removed with: SELECT 'DROP DATABASE ' || quote_ident(datname) || ';'
// FROM pg_database WHERE datname LIKE 'dbtest\_%';
package dbtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tjb/ai-stats/internal/crypto"
	"github.com/tjb/ai-stats/internal/db"
)

// EnvVar names the environment variable holding the Postgres server URL.
const EnvVar = "TEST_DATABASE_URL"

// encryptionKeyHex is a fixed AES-256 key for test databases. It only ever
// protects throwaway rows, so it is fine for it to be public.
const encryptionKeyHex = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

// EncryptionKey returns the AES-256 key the Queries from New are built with.
func EncryptionKey() []byte {
	key, err := crypto.ParseKey(encryptionKeyHex)
	if err != nil {
		panic(fmt.Sprintf("dbtest: bad test encryption key: %v", err))
	}
	return key
}

// New creates an isolated database on the TEST_DATABASE_URL server, applies
// every migration with db.Migrate and returns a pool plus Queries for it.
// The pool is closed and the database dropped in t.Cleanup.
func New(t testing.TB) (*pgxpool.Pool, *db.Queries) {
	t.Helper()

	serverURL := os.Getenv(EnvVar)
	if serverURL == "" {
		t.Skipf("%s is not set; skipping database integration test", EnvVar)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	adminCfg, err := pgx.ParseConfig(serverURL)
	if err != nil {
		t.Fatalf("dbtest: parse %s: %v", EnvVar, err)
	}

	name, err := randomName()
	if err != nil {
		t.Fatalf("dbtest: %v", err)
	}
	ident := pgx.Identifier{name}.Sanitize()

	admin, err := pgx.ConnectConfig(ctx, adminCfg)
	if err != nil {
		t.Fatalf("dbtest: connect to %s: %v", EnvVar, err)
	}
	_, err = admin.Exec(ctx, "CREATE DATABASE "+ident)
	admin.Close(ctx)
	if err != nil {
		t.Fatalf("dbtest: create database %s (the role needs CREATEDB): %v", name, err)
	}

	var pool *pgxpool.Pool
	t.Cleanup(func() {
		if pool != nil {
			pool.Close()
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		admin, err := pgx.ConnectConfig(ctx, adminCfg)
		if err != nil {
			t.Errorf("dbtest: connect to drop %s: %v", name, err)
			return
		}
		defer admin.Close(ctx)
		// FORCE terminates stragglers, e.g. fire-and-forget goroutines in handlers.
		if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+ident+" WITH (FORCE)"); err != nil {
			t.Errorf("dbtest: drop database %s: %v", name, err)
		}
	})

	poolCfg, err := pgxpool.ParseConfig(serverURL)
	if err != nil {
		t.Fatalf("dbtest: parse %s: %v", EnvVar, err)
	}
	poolCfg.ConnConfig.Database = name
	// Keep pools small: many test databases may share one server.
	poolCfg.MaxConns = 4
	poolCfg.MinConns = 0

	pool, err = pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Fatalf("dbtest: connect to %s: %v", name, err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("dbtest: ping %s: %v", name, err)
	}

	if err := db.Migrate(pool); err != nil {
		t.Fatalf("dbtest: migrate %s: %v", name, err)
	}

	return pool, db.New(pool, EncryptionKey())
}

// CreateUser inserts a GitHub-authenticated user the same way the OAuth
// callback does and returns it. The stored GitHub token is a placeholder.
func CreateUser(t testing.TB, q *db.Queries, email string) *db.User {
	t.Helper()
	u, err := q.UpsertUser(context.Background(), email, "Test User", "",
		"github", "gh-"+email, "gho_test_token")
	if err != nil {
		t.Fatalf("dbtest: create user %s: %v", email, err)
	}
	return u
}

// CreateOrg inserts an organization owned by ownerID with the given GitHub
// login and returns it.
func CreateOrg(t testing.TB, q *db.Queries, ownerID, login string) *db.Organization {
	t.Helper()
	o, err := q.CreateOrg(context.Background(), "gh-"+login, login, login+" Inc", nil,
		"gho_org_token", ownerID)
	if err != nil {
		t.Fatalf("dbtest: create org %s: %v", login, err)
	}
	return o
}

// Today returns the server's CURRENT_DATE as midnight UTC. Seed relative
// dates from it so rows line up with the CURRENT_DATE windows in queries.go.
func Today(t testing.TB, pool *pgxpool.Pool) time.Time {
	t.Helper()
	var d time.Time
	if err := pool.QueryRow(context.Background(), `SELECT CURRENT_DATE`).Scan(&d); err != nil {
		t.Fatalf("dbtest: select current_date: %v", err)
	}
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
}

func randomName() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("random database name: %w", err)
	}
	return "dbtest_" + hex.EncodeToString(b), nil
}
