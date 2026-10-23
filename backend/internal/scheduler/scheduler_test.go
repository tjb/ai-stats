package scheduler

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tjb/ai-stats/internal/github"
)

// fakeBackend is an in-memory Backend that records what the scheduler asks of it.
type fakeBackend struct {
	mu        sync.Mutex
	orgs      []github.Org
	listErr   error
	listCalls int
	synced    []string // org IDs, in the order SyncOrg was called

	// Optional hooks; listFn gets the 1-based call number.
	listFn func(call int) ([]github.Org, error)
	syncFn func(ctx context.Context, org github.Org) error

	active    atomic.Int32 // SyncOrg calls in progress
	maxActive atomic.Int32
}

func (f *fakeBackend) ListOrgs(_ context.Context) ([]github.Org, error) {
	f.mu.Lock()
	f.listCalls++
	call, fn, orgs, err := f.listCalls, f.listFn, f.orgs, f.listErr
	f.mu.Unlock()
	if fn != nil {
		return fn(call)
	}
	return orgs, err
}

func (f *fakeBackend) SyncOrg(ctx context.Context, org github.Org) error {
	n := f.active.Add(1)
	defer f.active.Add(-1)
	for {
		m := f.maxActive.Load()
		if n <= m || f.maxActive.CompareAndSwap(m, n) {
			break
		}
	}

	f.mu.Lock()
	f.synced = append(f.synced, org.ID)
	fn := f.syncFn
	f.mu.Unlock()
	if fn != nil {
		return fn(ctx, org)
	}
	return nil
}

func (f *fakeBackend) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.listCalls
}

func (f *fakeBackend) syncedIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.synced...)
}

func (f *fakeBackend) syncCount(orgID string) int {
	n := 0
	for _, id := range f.syncedIDs() {
		if id == orgID {
			n++
		}
	}
	return n
}

// waitFor polls cond until it holds, failing the test after a generous timeout.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

// start creates and starts a Scheduler that is stopped when the test ends.
func start(t *testing.T, cfg Config, b Backend) *Scheduler {
	t.Helper()
	s := New(cfg, b)
	s.Start()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := s.Stop(ctx); err != nil {
			t.Errorf("Stop: %v", err)
		}
	})
	return s
}

func orgs(ids ...string) []github.Org {
	out := make([]github.Org, 0, len(ids))
	for _, id := range ids {
		out = append(out, github.Org{ID: id, Login: id, Token: "tok-" + id})
	}
	return out
}

func TestParseInterval(t *testing.T) {
	tests := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{in: "", want: DefaultInterval},
		{in: "  ", want: DefaultInterval},
		{in: "1h", want: time.Hour},
		{in: "15m", want: 15 * time.Minute},
		{in: "1h30m", want: 90 * time.Minute},
		{in: "0", want: 0},
		{in: "0s", want: 0},
		{in: "off", want: 0},
		{in: "OFF", want: 0},
		{in: "10s", wantErr: true}, // below MinInterval
		{in: "-1h", wantErr: true},
		{in: "hourly", wantErr: true},
		{in: "60", wantErr: true}, // no unit
	}
	for _, tt := range tests {
		got, err := ParseInterval(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseInterval(%q) = %v, want error", tt.in, got)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Errorf("ParseInterval(%q) = %v, %v; want %v, nil", tt.in, got, err, tt.want)
		}
	}
}

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("SYNC_INTERVAL", "30m")
	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	want := Config{Interval: 30 * time.Minute, InitialDelay: DefaultInitialDelay, OrgTimeout: DefaultOrgTimeout}
	if cfg != want {
		t.Errorf("ConfigFromEnv() = %+v, want %+v", cfg, want)
	}

	t.Setenv("SYNC_INTERVAL", "soon")
	if _, err := ConfigFromEnv(); err == nil {
		t.Error("ConfigFromEnv() with invalid SYNC_INTERVAL: want error")
	}
}

func TestNewAppliesDefaultOrgTimeout(t *testing.T) {
	s := New(Config{Interval: time.Hour}, &fakeBackend{})
	if s.cfg.OrgTimeout != DefaultOrgTimeout {
		t.Errorf("OrgTimeout = %v, want %v", s.cfg.OrgTimeout, DefaultOrgTimeout)
	}
}

func TestDisabledSchedulerNeverRuns(t *testing.T) {
	b := &fakeBackend{orgs: orgs("a")}
	start(t, Config{Interval: 0}, b)

	time.Sleep(30 * time.Millisecond)
	if n := b.calls(); n != 0 {
		t.Errorf("ListOrgs called %d times with the scheduler disabled", n)
	}
}

func TestFirstRunWaitsForInitialDelay(t *testing.T) {
	b := &fakeBackend{orgs: orgs("a")}
	start(t, Config{Interval: time.Hour, InitialDelay: 250 * time.Millisecond}, b)

	time.Sleep(50 * time.Millisecond)
	if n := b.calls(); n != 0 {
		t.Fatalf("ListOrgs called %d times before the initial delay elapsed", n)
	}
	waitFor(t, "first run", func() bool { return b.syncCount("a") == 1 })
}

func TestRunsEveryIntervalAndSkipsDemoOrgs(t *testing.T) {
	b := &fakeBackend{orgs: []github.Org{
		{ID: "org-demo", Login: "demo-user-1", Token: "demo-token"},
		{ID: "org-a", Login: "acme", Token: "tok-a"},
		{ID: "org-b", Login: "globex", Token: "tok-b"},
	}}
	start(t, Config{Interval: 10 * time.Millisecond}, b)

	waitFor(t, "three runs", func() bool { return b.syncCount("org-b") >= 3 })
	if n := b.syncCount("org-demo"); n != 0 {
		t.Errorf("demo org synced %d times, want 0", n)
	}
	if got := b.syncedIDs()[:2]; got[0] != "org-a" || got[1] != "org-b" {
		t.Errorf("first run synced %v, want [org-a org-b]", got)
	}
}

func TestSkipsTickWhilePreviousRunIsInProgress(t *testing.T) {
	release := make(chan struct{})
	b := &fakeBackend{orgs: orgs("a")}
	b.syncFn = func(ctx context.Context, _ github.Org) error {
		select {
		case <-release:
		case <-ctx.Done():
		}
		return nil
	}
	start(t, Config{Interval: 5 * time.Millisecond}, b)

	waitFor(t, "first sync to start", func() bool { return b.syncCount("a") == 1 })
	time.Sleep(60 * time.Millisecond) // ~12 ticks while the first run is blocked
	if n := b.calls(); n != 1 {
		t.Errorf("ListOrgs called %d times while a run was in progress, want 1", n)
	}

	close(release)
	waitFor(t, "a run after the blocked one finished", func() bool { return b.calls() >= 2 })
	if m := b.maxActive.Load(); m != 1 {
		t.Errorf("max concurrent SyncOrg calls = %d, want 1", m)
	}
}

func TestOrgErrorsAndPanicsDoNotStopTheRun(t *testing.T) {
	b := &fakeBackend{orgs: orgs("fails", "panics", "ok")}
	b.syncFn = func(_ context.Context, org github.Org) error {
		switch org.ID {
		case "fails":
			return errors.New("github: 502 bad gateway")
		case "panics":
			panic("boom")
		}
		return nil
	}
	start(t, Config{Interval: 10 * time.Millisecond}, b)

	// "ok" syncing twice proves the run got past both bad orgs and that the
	// scheduler kept ticking afterwards.
	waitFor(t, "two runs", func() bool { return b.syncCount("ok") >= 2 })
	if got := b.syncedIDs()[:3]; got[0] != "fails" || got[1] != "panics" || got[2] != "ok" {
		t.Errorf("first run synced %v, want [fails panics ok]", got)
	}
}

func TestListOrgsFailuresAreRetriedNextTick(t *testing.T) {
	b := &fakeBackend{}
	b.listFn = func(call int) ([]github.Org, error) {
		switch call {
		case 1:
			return nil, errors.New("db: connection refused")
		case 2:
			panic("list exploded")
		case 3:
			// Partial result: one org failed to load, the rest should still sync.
			return orgs("a"), errors.New("org=b: decrypt token")
		}
		return nil, nil
	}
	start(t, Config{Interval: 10 * time.Millisecond}, b)

	waitFor(t, "partial list to sync", func() bool { return b.syncCount("a") == 1 })
	if n := b.calls(); n < 3 {
		t.Errorf("ListOrgs called %d times, want >= 3", n)
	}
}

func TestOrgTimeoutBoundsEachOrg(t *testing.T) {
	var (
		mu       sync.Mutex
		slowErr  error
		deadline bool
	)
	b := &fakeBackend{orgs: orgs("slow", "next")}
	b.syncFn = func(ctx context.Context, org github.Org) error {
		if org.ID != "slow" {
			return nil
		}
		_, ok := ctx.Deadline()
		<-ctx.Done() // a sync that only stops when its context does
		mu.Lock()
		slowErr, deadline = ctx.Err(), ok
		mu.Unlock()
		return ctx.Err()
	}
	start(t, Config{Interval: time.Hour, OrgTimeout: 20 * time.Millisecond}, b)

	waitFor(t, "org after the slow one", func() bool { return b.syncCount("next") == 1 })
	mu.Lock()
	defer mu.Unlock()
	if !deadline {
		t.Error("SyncOrg context had no deadline")
	}
	if !errors.Is(slowErr, context.DeadlineExceeded) {
		t.Errorf("slow org ctx.Err() = %v, want deadline exceeded", slowErr)
	}
}

func TestStopCancelsInFlightRun(t *testing.T) {
	gotErr := make(chan error, 1)
	b := &fakeBackend{orgs: orgs("a", "b")}
	b.syncFn = func(ctx context.Context, org github.Org) error {
		if org.ID == "a" {
			<-ctx.Done()
			gotErr <- ctx.Err()
			return ctx.Err()
		}
		return nil
	}
	s := New(Config{Interval: 5 * time.Millisecond, OrgTimeout: time.Hour}, b)
	s.Start()

	waitFor(t, "sync to start", func() bool { return b.syncCount("a") == 1 })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := s.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	select {
	case err := <-gotErr:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("in-flight sync ctx.Err() = %v, want canceled", err)
		}
	default:
		t.Fatal("Stop returned before the in-flight sync finished")
	}

	// Nothing runs after Stop: not the rest of the cancelled run, nor new ticks.
	calls := b.calls()
	time.Sleep(30 * time.Millisecond)
	if n := b.calls(); n != calls {
		t.Errorf("ListOrgs called %d more times after Stop", n-calls)
	}
	if n := b.syncCount("b"); n != 0 {
		t.Errorf("org b synced %d times after its run was cancelled", n)
	}
}

func TestStopDuringInitialDelay(t *testing.T) {
	b := &fakeBackend{orgs: orgs("a")}
	s := New(Config{Interval: time.Hour, InitialDelay: time.Hour}, b)
	s.Start()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if n := b.calls(); n != 0 {
		t.Errorf("ListOrgs called %d times, want 0", n)
	}
}

func TestStopGivesUpOnSyncThatIgnoresCancellation(t *testing.T) {
	release := make(chan struct{})
	b := &fakeBackend{orgs: orgs("stuck")}
	b.syncFn = func(context.Context, github.Org) error {
		<-release // ignores ctx
		return nil
	}
	s := New(Config{Interval: time.Hour}, b)
	s.Start()
	waitFor(t, "sync to start", func() bool { return b.syncCount("stuck") == 1 })

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := s.Stop(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Stop = %v, want deadline exceeded", err)
	}

	close(release)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel2()
	if err := s.Stop(ctx2); err != nil {
		t.Errorf("second Stop after the sync returned: %v", err)
	}
}
