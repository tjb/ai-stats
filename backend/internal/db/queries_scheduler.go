package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/tjb/ai-stats/internal/crypto"
)

// ---- Background sync ----

// ListSyncableOrgs returns every organization that has a stored GitHub token,
// with the token decrypted, oldest first. Used by the background sync
// scheduler. Demo orgs are included; callers decide whether to skip them.
//
// An org whose token cannot be decrypted is left out and its error is joined
// into the returned error, so one bad row cannot stop every other org from
// syncing: callers should use the returned orgs even when err != nil.
func (q *Queries) ListSyncableOrgs(ctx context.Context) ([]Organization, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, github_org_id, github_org_login, name, avatar_url,
		       github_access_token, owner_id,
		       github_member_count, created_at, updated_at
		FROM organizations
		WHERE github_access_token <> ''
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("ListSyncableOrgs: %w", err)
	}
	defer rows.Close()

	out := []Organization{}
	var decryptErrs []error
	for rows.Next() {
		var o Organization
		if err := rows.Scan(
			&o.ID, &o.GitHubOrgID, &o.GitHubOrgLogin, &o.Name, &o.AvatarURL,
			&o.GitHubAccessToken, &o.OwnerID,
			&o.GithubMemberCount, &o.CreatedAt, &o.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("ListSyncableOrgs scan: %w", err)
		}
		token, err := crypto.Decrypt(o.GitHubAccessToken, q.encryptionKey)
		if err != nil {
			decryptErrs = append(decryptErrs, fmt.Errorf("ListSyncableOrgs org=%s: decrypt token: %w", o.ID, err))
			continue
		}
		if token == "" {
			continue
		}
		o.GitHubAccessToken = token
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListSyncableOrgs: %w", err)
	}
	return out, errors.Join(decryptErrs...)
}

// GetClaudeCodeAttributedSHAs returns the commit SHAs that Claude Code
// commit-trailer attribution has already counted for one org and day (stored
// in tool_usage_snapshots.raw_json.attributed_commits). The attribution sync
// uses it to avoid adding the same commit to the day's totals twice. Returns
// an empty slice when there is no row or the row holds no SHA list (e.g. it was
// last written by push ingest).
func (q *Queries) GetClaudeCodeAttributedSHAs(ctx context.Context, orgID string, date time.Time) ([]string, error) {
	var raw []byte
	err := q.pool.QueryRow(ctx, `
		SELECT COALESCE(
		         (SELECT CASE WHEN jsonb_typeof(raw_json->'attributed_commits') = 'array'
		                      THEN raw_json->'attributed_commits'
		                 END
		          FROM tool_usage_snapshots
		          WHERE organization_id = $1
		            AND tool_source = 'claude_code'
		            AND date = $2),
		         '[]'::jsonb)
	`, orgID, date).Scan(&raw)
	if err != nil {
		return nil, fmt.Errorf("GetClaudeCodeAttributedSHAs: %w", err)
	}
	shas := []string{}
	if err := json.Unmarshal(raw, &shas); err != nil {
		return nil, fmt.Errorf("GetClaudeCodeAttributedSHAs decode: %w", err)
	}
	return shas, nil
}
