package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/tjb/ai-stats/internal/db"
)

// Org identifies a connected GitHub organization to sync.
type Org struct {
	ID    string // internal organization UUID
	Login string // GitHub org login
	Token string // decrypted GitHub access token
}

// OrgFromDB converts a stored organization into the Org the syncs take.
func OrgFromDB(o *db.Organization) Org {
	return Org{ID: o.ID, Login: o.GitHubOrgLogin, Token: o.GitHubAccessToken}
}

// IsDemoOrg reports whether login belongs to a seeded demo org. Demo orgs have
// a placeholder token, so syncing them would only produce GitHub API errors.
func IsDemoOrg(login string) bool {
	return strings.HasPrefix(login, "demo-")
}

// SyncStore is the subset of db.Queries the sync service needs.
type SyncStore interface {
	UpsertSnapshot(ctx context.Context, orgID string, date time.Time, totalSuggestions, totalAcceptances, totalLinesAccepted, totalActiveUsers, totalLicensedUsers int, breakdown []db.BreakdownItem) error
	UpsertToolSnapshot(ctx context.Context, orgID, toolSource string, date time.Time,
		totalSuggestions, totalAcceptances, totalLinesAccepted,
		totalActiveUsers, totalLicensedUsers, totalCodingDevelopers int,
		totalPlanningUses, totalCodingUses int,
		rawJSON json.RawMessage) error
	GetClaudeCodeAttributedSHAs(ctx context.Context, orgID string, date time.Time) ([]string, error)
	GetActiveToolSources(ctx context.Context, orgID string) ([]string, error)
	UpdateOrgMemberCount(ctx context.Context, orgID string, count int) error
}

var _ SyncStore = (*db.Queries)(nil)

// Service is the single entry point for pulling data from GitHub. The manual
// sync endpoints and the background scheduler both go through it, so a manual
// "Sync now" and a scheduled run always do the same work.
type Service struct {
	store SyncStore
}

// NewService returns a Service that writes synced data to store.
func NewService(store SyncStore) *Service {
	return &Service{store: store}
}

// copilotOrg adapts Org to the accessor interface SyncUsage expects.
type copilotOrg struct{ Org }

func (o copilotOrg) GetID() string    { return o.ID }
func (o copilotOrg) GetLogin() string { return o.Login }
func (o copilotOrg) GetToken() string { return o.Token }

// SyncCopilot pulls Copilot usage for org and returns the number of days stored.
func (s *Service) SyncCopilot(ctx context.Context, org Org) (int, error) {
	return SyncUsage(ctx, s.store, copilotOrg{org})
}

// SyncClaudeCode runs Claude Code commit-trailer attribution for org and
// returns the number of days stored. Runs for the same org are serialised:
// the attribution adds new commits to the stored totals, so two overlapping
// runs (e.g. "Sync now" during a scheduled run) would count them twice.
func (s *Service) SyncClaudeCode(ctx context.Context, org Org) (int, error) {
	unlock, err := lockAttribution(ctx, org.ID)
	if err != nil {
		return 0, err
	}
	defer unlock()
	return SyncClaudeCodeAttribution(ctx, org.ID, org.Login, org.Token, s.store)
}

// RefreshMemberCount stores the org's current GitHub member count.
func (s *Service) RefreshMemberCount(ctx context.Context, org Org) error {
	n, err := GetOrgMemberCount(ctx, org.Login, org.Token)
	if err != nil {
		return fmt.Errorf("get member count: %w", err)
	}
	if err := s.store.UpdateOrgMemberCount(ctx, org.ID, n); err != nil {
		return fmt.Errorf("update member count: %w", err)
	}
	return nil
}

// ToolResult is the outcome of syncing one tool source.
type ToolResult struct {
	ToolSource string `json:"tool_source"`
	Synced     int    `json:"synced"`
	Error      string `json:"error,omitempty"`
}

// SyncAll runs every pull-based sync for org: Copilot always, Claude Code
// commit attribution when it has data for the org in the last 90 days, and a
// member-count refresh. Push-ingest sources (codex, push_ingest, ...) write
// their own data and are skipped.
//
// Every step runs even if an earlier one fails. results has one entry per tool
// source attempted; the returned error joins every failure (errors.Is/As see
// each one), including the best-effort steps that have no entry in results.
func (s *Service) SyncAll(ctx context.Context, org Org) ([]ToolResult, error) {
	var (
		results []ToolResult
		errs    []error
	)
	record := func(source string, synced int, err error) {
		r := ToolResult{ToolSource: source, Synced: synced}
		if err != nil {
			r.Error = err.Error()
			errs = append(errs, fmt.Errorf("%s: %w", source, err))
		}
		results = append(results, r)
	}

	// Always attempt Copilot — it's the primary source.
	synced, err := s.SyncCopilot(ctx, org)
	record("copilot", synced, err)

	// Check which additional tool sources are active in tool_usage_snapshots.
	activeSources, err := s.store.GetActiveToolSources(ctx, org.ID)
	if err != nil {
		errs = append(errs, fmt.Errorf("active tool sources: %w", err))
	}
	for _, src := range activeSources {
		switch src {
		case "claude_code":
			synced, err := s.SyncClaudeCode(ctx, org)
			record("claude_code", synced, err)
		default:
			// copilot was synced above; push-ingest sources write data themselves.
		}
	}

	// Always refresh member count — best-effort, reported only via the error.
	if err := s.RefreshMemberCount(ctx, org); err != nil {
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		return results, nil
	}
	return results, syncErrors(errs)
}

// syncErrors joins several sync failures into one single-line error (unlike
// errors.Join, which uses newlines) so each log entry keeps its org prefix.
type syncErrors []error

func (e syncErrors) Error() string {
	msgs := make([]string, len(e))
	for i, err := range e {
		msgs[i] = err.Error()
	}
	return strings.Join(msgs, "; ")
}

func (e syncErrors) Unwrap() []error { return e }

// attributionLocks holds one single-slot semaphore per org ID. It is package
// level because the HTTP handlers build a Service per request while the
// scheduler keeps its own.
var attributionLocks sync.Map // org ID -> chan struct{}

// lockAttribution waits until no other Claude Code attribution is running for
// orgID, or ctx is done, and returns the matching unlock func.
func lockAttribution(ctx context.Context, orgID string) (func(), error) {
	v, _ := attributionLocks.LoadOrStore(orgID, make(chan struct{}, 1))
	sem := v.(chan struct{})
	select {
	case sem <- struct{}{}:
		return func() { <-sem }, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("waiting for running Claude Code sync: %w", ctx.Err())
	}
}
