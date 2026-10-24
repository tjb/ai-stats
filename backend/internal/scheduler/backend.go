package scheduler

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/tjb/ai-stats/internal/db"
	"github.com/tjb/ai-stats/internal/github"
)

// appBackend connects the scheduler to the database and the shared GitHub
// sync service used by the manual sync endpoints.
type appBackend struct {
	queries *db.Queries
	sync    *github.Service
}

// NewBackend returns the production Backend: orgs come from the database and
// each org runs the same syncs as POST /api/tools/sync/all.
func NewBackend(queries *db.Queries) Backend {
	return &appBackend{queries: queries, sync: github.NewService(queries)}
}

func (b *appBackend) ListOrgs(ctx context.Context) ([]github.Org, error) {
	orgs, err := b.queries.ListSyncableOrgs(ctx)
	out := make([]github.Org, 0, len(orgs))
	for i := range orgs {
		out = append(out, github.OrgFromDB(&orgs[i]))
	}
	return out, err
}

func (b *appBackend) SyncOrg(ctx context.Context, org github.Org) error {
	results, err := b.sync.SyncAll(ctx, org)
	parts := make([]string, 0, len(results))
	for _, r := range results {
		part := fmt.Sprintf("%s=%d", r.ToolSource, r.Synced)
		if r.Error != "" {
			part += "(failed)"
		}
		parts = append(parts, part)
	}
	log.Printf("scheduler: org=%s days synced: %s", org.ID, strings.Join(parts, " "))
	return err
}
