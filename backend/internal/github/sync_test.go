package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tjb/ai-stats/internal/db"
)

// fakeSyncStore is an in-memory SyncStore. UpsertToolSnapshot mimics the real
// query: counters are added to the existing row and raw_json is replaced.
type fakeSyncStore struct {
	mu            sync.Mutex
	toolRows      map[string]*fakeToolRow // keyed by YYYY-MM-DD (claude_code only)
	activeSources []string
	memberCount   int
}

type fakeToolRow struct {
	acceptances, lines, activeUsers, codingDevs int
	rawJSON                                     json.RawMessage
}

func newFakeSyncStore() *fakeSyncStore {
	return &fakeSyncStore{toolRows: map[string]*fakeToolRow{}}
}

func (f *fakeSyncStore) UpsertSnapshot(context.Context, string, time.Time, int, int, int, int, int, []db.BreakdownItem) error {
	return nil
}

func (f *fakeSyncStore) UpsertToolSnapshot(_ context.Context, _, toolSource string, date time.Time,
	_, acceptances, lines, activeUsers, _, codingDevs int, _, _ int, rawJSON json.RawMessage) error {
	if toolSource != "claude_code" {
		return fmt.Errorf("unexpected tool source %q", toolSource)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	key := date.Format("2006-01-02")
	row, ok := f.toolRows[key]
	if !ok {
		row = &fakeToolRow{}
		f.toolRows[key] = row
	}
	row.acceptances += acceptances
	row.lines += lines
	row.activeUsers = max(row.activeUsers, activeUsers)
	row.codingDevs = max(row.codingDevs, codingDevs)
	row.rawJSON = rawJSON
	return nil
}

func (f *fakeSyncStore) GetClaudeCodeAttributedSHAs(_ context.Context, _ string, date time.Time) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	row, ok := f.toolRows[date.Format("2006-01-02")]
	if !ok {
		return []string{}, nil
	}
	var raw claudeCodeRawJSON
	if err := json.Unmarshal(row.rawJSON, &raw); err != nil {
		return nil, err
	}
	return raw.AttributedCommits, nil
}

func (f *fakeSyncStore) GetActiveToolSources(context.Context, string) ([]string, error) {
	return f.activeSources, nil
}

func (f *fakeSyncStore) UpdateOrgMemberCount(_ context.Context, _ string, count int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.memberCount = count
	return nil
}

func (f *fakeSyncStore) row(day string) fakeToolRow {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.toolRows[day]; ok {
		return *r
	}
	return fakeToolRow{}
}

// fakeCommit is one commit served by fakeGitHub.
type fakeCommit struct {
	sha, author, message string
	additions, deletions int
}

// fakeGitHub serves the GitHub endpoints the syncs call for the org "acme",
// which has a single repo "app".
type fakeGitHub struct {
	mu            sync.Mutex
	commits       []fakeCommit
	detailFetches int
}

func (g *fakeGitHub) setCommits(c ...fakeCommit) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.commits = c
}

func (g *fakeGitHub) fetches() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.detailFetches
}

func (g *fakeGitHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now().UTC().Format(time.RFC3339)
	path := r.URL.Path
	switch {
	case path == "/orgs/acme/repos":
		fmt.Fprint(w, `[{"name":"app"}]`)
	case path == "/repos/acme/app/commits":
		items := []map[string]any{}
		for _, c := range g.commits {
			items = append(items, map[string]any{
				"sha":    c.sha,
				"commit": map[string]any{"message": c.message, "author": map[string]any{"date": now}},
				"author": map[string]any{"login": c.author},
			})
		}
		json.NewEncoder(w).Encode(items)
	case strings.HasPrefix(path, "/repos/acme/app/commits/"):
		sha := strings.TrimPrefix(path, "/repos/acme/app/commits/")
		for _, c := range g.commits {
			if c.sha == sha {
				g.detailFetches++
				json.NewEncoder(w).Encode(map[string]any{
					"sha":    c.sha,
					"commit": map[string]any{"message": c.message},
					"author": map[string]any{"login": c.author},
					"stats":  map[string]any{"additions": c.additions, "deletions": c.deletions},
				})
				return
			}
		}
		http.NotFound(w, r)
	case path == "/orgs/acme/members":
		w.Header().Set("Link", `<https://api.github.com/orgs/acme/members?per_page=1&page=7>; rel="last"`)
		fmt.Fprint(w, `[{"login":"someone"}]`)
	case strings.Contains(path, "/copilot/"):
		http.Error(w, "copilot unavailable", http.StatusInternalServerError)
	default:
		http.NotFound(w, r)
	}
}

// redirectTransport sends every request to target, whatever host it named.
type redirectTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (rt redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	r.URL.Scheme = rt.target.Scheme
	r.URL.Host = rt.target.Host
	return rt.base.RoundTrip(r)
}

// useFakeGitHub routes all outgoing HTTP (every client without its own
// Transport, including ghClient and http.DefaultClient) to a fake GitHub.
func useFakeGitHub(t *testing.T) *fakeGitHub {
	t.Helper()
	gh := &fakeGitHub{}
	srv := httptest.NewServer(gh)
	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	orig := http.DefaultTransport
	http.DefaultTransport = redirectTransport{target: target, base: orig}
	t.Cleanup(func() {
		http.DefaultTransport = orig
		srv.Close()
	})
	return gh
}

const claudeTrailer = "\n\nCo-Authored-By: Claude <noreply@anthropic.com>"

func TestSyncClaudeCodeAttributionCountsEachCommitOnce(t *testing.T) {
	gh := useFakeGitHub(t)
	store := newFakeSyncStore()
	ctx := context.Background()
	today := time.Now().UTC().Format("2006-01-02")

	gh.setCommits(
		fakeCommit{sha: "c1", author: "alice", message: "feat: one" + claudeTrailer, additions: 10, deletions: 2},
		fakeCommit{sha: "c2", author: "bob", message: "fix: by hand"},
		fakeCommit{sha: "c3", author: "carol", message: "feat: three" + claudeTrailer, additions: 5},
	)
	for run := 1; run <= 2; run++ {
		if _, err := SyncClaudeCodeAttribution(ctx, "org-1", "acme", "tok", store); err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		got := store.row(today)
		if got.acceptances != 2 || got.lines != 13 || got.activeUsers != 2 || got.codingDevs != 3 {
			t.Errorf("after run %d: commits=%d lines=%d aiAuthors=%d devs=%d, want 2 13 2 3",
				run, got.acceptances, got.lines, got.activeUsers, got.codingDevs)
		}
	}
	if n := gh.fetches(); n != 2 {
		t.Errorf("commit detail fetched %d times, want 2 (counted commits are not refetched)", n)
	}

	// A new attributed commit is added on top of what was already counted.
	gh.setCommits(
		fakeCommit{sha: "c1", author: "alice", message: "feat: one" + claudeTrailer, additions: 10, deletions: 2},
		fakeCommit{sha: "c4", author: "alice", message: "feat: four" + claudeTrailer, additions: 1},
	)
	if _, err := SyncClaudeCodeAttribution(ctx, "org-1", "acme", "tok", store); err != nil {
		t.Fatal(err)
	}
	got := store.row(today)
	if got.acceptances != 3 || got.lines != 14 {
		t.Errorf("after new commit: commits=%d lines=%d, want 3 14", got.acceptances, got.lines)
	}
	shas, _ := store.GetClaudeCodeAttributedSHAs(ctx, "org-1", time.Now().UTC())
	if strings.Join(shas, ",") != "c1,c3,c4" {
		t.Errorf("attributed_commits = %v, want [c1 c3 c4]", shas)
	}
}

func TestServiceSyncAll(t *testing.T) {
	gh := useFakeGitHub(t)
	gh.setCommits(fakeCommit{sha: "c1", author: "alice", message: "feat" + claudeTrailer, additions: 3})
	store := newFakeSyncStore()
	store.activeSources = []string{"claude_code", "codex", "copilot"}

	results, err := NewService(store).SyncAll(context.Background(), Org{ID: "org-1", Login: "acme", Token: "tok"})

	if len(results) != 2 {
		t.Fatalf("results = %+v, want copilot and claude_code only", results)
	}
	if results[0].ToolSource != "copilot" || results[0].Error == "" {
		t.Errorf("results[0] = %+v, want a failed copilot sync", results[0])
	}
	if want := (ToolResult{ToolSource: "claude_code", Synced: 1}); results[1] != want {
		t.Errorf("results[1] = %+v, want %+v", results[1], want)
	}
	if err == nil || !strings.Contains(err.Error(), "copilot") {
		t.Errorf("err = %v, want it to report the copilot failure", err)
	} else if strings.Contains(err.Error(), "\n") {
		t.Errorf("err = %q, want a single line for the logs", err)
	}
	if store.memberCount != 7 {
		t.Errorf("member count = %d, want 7", store.memberCount)
	}
}

func TestLockAttributionSerialisesPerOrg(t *testing.T) {
	unlock, err := lockAttribution(context.Background(), "lock-org-1")
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := lockAttribution(ctx, "lock-org-1"); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("second lock on the same org = %v, want deadline exceeded", err)
	}

	unlockOther, err := lockAttribution(context.Background(), "lock-org-2")
	if err != nil {
		t.Fatalf("lock on another org: %v", err)
	}
	unlockOther()

	unlock()
	unlock2, err := lockAttribution(context.Background(), "lock-org-1")
	if err != nil {
		t.Fatalf("lock after unlock: %v", err)
	}
	unlock2()
}

func TestIsDemoOrg(t *testing.T) {
	for login, want := range map[string]bool{
		"demo-123": true,
		"demo-":    true,
		"acme":     false,
		"my-demo-": false,
		"Demo-1":   false,
	} {
		if got := IsDemoOrg(login); got != want {
			t.Errorf("IsDemoOrg(%q) = %v, want %v", login, got, want)
		}
	}
}
