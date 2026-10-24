// Package scheduler re-runs the GitHub syncs for every connected organization
// on a fixed interval, so dashboards stay fresh without anyone pressing
// "Sync now".
package scheduler

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tjb/ai-stats/internal/github"
)

const (
	// DefaultInterval is the time between runs when SYNC_INTERVAL is unset.
	DefaultInterval = time.Hour
	// DefaultInitialDelay is how long after Start the first run begins, giving
	// the server time to come up before it starts calling GitHub.
	DefaultInitialDelay = 30 * time.Second
	// DefaultOrgTimeout bounds the syncs for a single org within a run.
	DefaultOrgTimeout = 10 * time.Minute
	// MinInterval is the shortest SYNC_INTERVAL accepted, to stay well clear
	// of GitHub API rate limits.
	MinInterval = time.Minute
)

// Config controls when the scheduler runs.
type Config struct {
	Interval     time.Duration // time between runs; 0 disables the scheduler
	InitialDelay time.Duration // wait between Start and the first run
	OrgTimeout   time.Duration // per-org sync timeout; <= 0 means DefaultOrgTimeout
}

// ConfigFromEnv builds a Config from SYNC_INTERVAL (see ParseInterval) with
// the default initial delay and per-org timeout.
func ConfigFromEnv() (Config, error) {
	interval, err := ParseInterval(os.Getenv("SYNC_INTERVAL"))
	if err != nil {
		return Config{}, err
	}
	return Config{
		Interval:     interval,
		InitialDelay: DefaultInitialDelay,
		OrgTimeout:   DefaultOrgTimeout,
	}, nil
}

// ParseInterval parses a SYNC_INTERVAL value: a Go duration such as "1h" or
// "15m" (at least MinInterval). An empty value means DefaultInterval; "0",
// "0s" or "off" disable background sync.
func ParseInterval(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	switch strings.ToLower(s) {
	case "":
		return DefaultInterval, nil
	case "0", "off":
		return 0, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("SYNC_INTERVAL %q: want a duration like 1h or 15m, or off: %w", s, err)
	}
	if d == 0 {
		return 0, nil
	}
	if d < MinInterval {
		return 0, fmt.Errorf("SYNC_INTERVAL %q: must be at least %s, or off", s, MinInterval)
	}
	return d, nil
}

// Backend is everything the scheduler needs from the rest of the app. It is
// an interface so the scheduler can be tested without a database or network.
type Backend interface {
	// ListOrgs returns the organizations to sync. It may return orgs alongside
	// a non-nil error when only some of them could be loaded.
	ListOrgs(ctx context.Context) ([]github.Org, error)
	// SyncOrg runs every sync for one org and should return once ctx is done.
	SyncOrg(ctx context.Context, org github.Org) error
}

// Scheduler runs Backend.SyncOrg for every non-demo org once per interval.
// Runs never overlap: a tick that fires while the previous run is still going
// is skipped.
type Scheduler struct {
	cfg     Config
	backend Backend

	mu     sync.Mutex
	cancel context.CancelFunc // set by Start, nil before
	wg     sync.WaitGroup     // the loop goroutine plus any in-flight run

	running atomic.Bool // a run is in progress
}

// New returns a Scheduler; call Start to begin running.
func New(cfg Config, backend Backend) *Scheduler {
	if cfg.OrgTimeout <= 0 {
		cfg.OrgTimeout = DefaultOrgTimeout
	}
	if cfg.InitialDelay < 0 {
		cfg.InitialDelay = 0
	}
	return &Scheduler{cfg: cfg, backend: backend}
}

// Start launches the background loop and returns immediately. It does nothing
// if the scheduler is disabled (Interval <= 0) or already started.
func (s *Scheduler) Start() {
	if s.cfg.Interval <= 0 {
		log.Printf("scheduler: background sync disabled (SYNC_INTERVAL=off)")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	log.Printf("scheduler: background sync every %s, first run in %s", s.cfg.Interval, s.cfg.InitialDelay)
	s.wg.Add(1)
	go s.loop(ctx)
}

// Stop cancels the loop and any in-flight run, then waits for them to return
// or for ctx to be done, whichever comes first.
func (s *Scheduler) Stop(ctx context.Context) error {
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("scheduler: waiting for in-flight sync: %w", ctx.Err())
	}
}

func (s *Scheduler) loop(ctx context.Context) {
	defer s.wg.Done()

	delay := time.NewTimer(s.cfg.InitialDelay)
	defer delay.Stop()
	select {
	case <-ctx.Done():
		return
	case <-delay.C:
	}
	s.tick(ctx)

	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

// tick starts a run in the background unless the previous one is still going.
// The run happens off the loop goroutine so Stop is never stuck behind it.
func (s *Scheduler) tick(ctx context.Context) {
	if ctx.Err() != nil {
		return // stopping; select may have picked the tick over ctx.Done
	}
	if !s.running.CompareAndSwap(false, true) {
		log.Printf("scheduler: previous sync run still in progress, skipping this tick")
		return
	}
	// Safe to Add while Stop may be in Wait: the loop's own count is held.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.running.Store(false)
		defer func() {
			if r := recover(); r != nil {
				log.Printf("scheduler: sync run panicked: %v", r)
			}
		}()
		s.run(ctx)
	}()
}

// run syncs every non-demo org once, one org at a time.
func (s *Scheduler) run(ctx context.Context) {
	start := time.Now()

	orgs, err := s.backend.ListOrgs(ctx)
	if err != nil {
		// Partial results are still worth syncing.
		log.Printf("scheduler: list orgs: %v", err)
	}

	var synced, failed int
	for _, org := range orgs {
		if ctx.Err() != nil {
			log.Printf("scheduler: sync run cancelled after %d of %d orgs", synced+failed, len(orgs))
			return
		}
		if github.IsDemoOrg(org.Login) {
			continue
		}
		if err := s.syncOrg(ctx, org); err != nil {
			failed++
			log.Printf("scheduler: sync org=%s: %v", org.ID, err)
			continue
		}
		synced++
	}
	log.Printf("scheduler: sync run finished in %s: %d orgs synced, %d failed",
		time.Since(start).Round(time.Millisecond), synced, failed)
}

// syncOrg syncs one org under the per-org timeout. A panic is turned into an
// error so one bad org can't stop the rest of the run or crash the process.
func (s *Scheduler) syncOrg(ctx context.Context, org github.Org) (err error) {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.OrgTimeout)
	defer cancel()
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return s.backend.SyncOrg(ctx, org)
}
