package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tjb/ai-stats/internal/crypto"
)

// Queries holds all database operations.
// We write these by hand (no sqlc codegen) to keep the toolchain simple.
type Queries struct {
	pool          *pgxpool.Pool
	encryptionKey []byte
}

func New(pool *pgxpool.Pool, encryptionKey []byte) *Queries {
	return &Queries{pool: pool, encryptionKey: encryptionKey}
}

// ---- Users ----

type User struct {
	ID                    string
	Email                 string
	Name                  *string
	Image                 *string
	HourlyRate            int
	OnboardingCompletedAt *time.Time
	CreatedAt             time.Time
}

func (q *Queries) GetUserByID(ctx context.Context, id string) (*User, error) {
	u := &User{}
	err := q.pool.QueryRow(ctx, `
		SELECT id, email, name, image, hourly_rate, onboarding_completed_at, created_at
		FROM users WHERE id = $1
	`, id).Scan(&u.ID, &u.Email, &u.Name, &u.Image, &u.HourlyRate, &u.OnboardingCompletedAt, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (q *Queries) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	u := &User{}
	err := q.pool.QueryRow(ctx, `
		SELECT id, email, name, image, hourly_rate, created_at
		FROM users WHERE email = $1
	`, email).Scan(&u.ID, &u.Email, &u.Name, &u.Image, &u.HourlyRate, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// UpsertUser creates or updates a user by provider account, returning the user ID.
func (q *Queries) UpsertUser(ctx context.Context, email, name, image, provider, providerAccountID, accessToken string) (*User, error) {
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Upsert user
	u := &User{}
	err = tx.QueryRow(ctx, `
		INSERT INTO users (email, name, image)
		VALUES ($1, $2, $3)
		ON CONFLICT (email) DO UPDATE SET
			name  = EXCLUDED.name,
			image = EXCLUDED.image
		RETURNING id, email, name, image, hourly_rate, created_at
	`, email, nullStr(name), nullStr(image)).Scan(
		&u.ID, &u.Email, &u.Name, &u.Image, &u.HourlyRate, &u.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}

	// Upsert account (store latest access token)
	_, err = tx.Exec(ctx, `
		INSERT INTO accounts (user_id, provider, provider_account_id, access_token)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (provider, provider_account_id) DO UPDATE SET
			access_token = EXCLUDED.access_token
	`, u.ID, provider, providerAccountID, accessToken)
	if err != nil {
		return nil, fmt.Errorf("upsert account: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return u, nil
}

func (q *Queries) UpdateUserHourlyRate(ctx context.Context, userID string, rate int) error {
	_, err := q.pool.Exec(ctx, `
		UPDATE users SET hourly_rate = $1 WHERE id = $2
	`, rate, userID)
	return err
}

func (q *Queries) DeleteUser(ctx context.Context, userID string) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	return err
}

func (q *Queries) CompleteOnboarding(ctx context.Context, userID string) error {
	_, err := q.pool.Exec(ctx, `
		UPDATE users SET onboarding_completed_at = now() WHERE id = $1
	`, userID)
	return err
}

// GetUserAccessToken returns the stored GitHub access token for a user.
func (q *Queries) GetUserAccessToken(ctx context.Context, userID string) (string, error) {
	var tok string
	err := q.pool.QueryRow(ctx, `
		SELECT access_token FROM accounts
		WHERE user_id = $1 AND provider = 'github'
		LIMIT 1
	`, userID).Scan(&tok)
	return tok, err
}

// ---- Organizations ----

type Organization struct {
	ID                string
	GitHubOrgID       string
	GitHubOrgLogin    string
	Name              string
	AvatarURL         *string
	GitHubAccessToken string
	OwnerID           string
	GithubMemberCount *int
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (q *Queries) GetOrgByOwner(ctx context.Context, ownerID string) (*Organization, error) {
	o := &Organization{}
	err := q.pool.QueryRow(ctx, `
		SELECT id, github_org_id, github_org_login, name, avatar_url,
		       github_access_token, owner_id,
		       github_member_count, created_at, updated_at
		FROM organizations WHERE owner_id = $1
		ORDER BY created_at ASC LIMIT 1
	`, ownerID).Scan(
		&o.ID, &o.GitHubOrgID, &o.GitHubOrgLogin, &o.Name, &o.AvatarURL,
		&o.GitHubAccessToken, &o.OwnerID,
		&o.GithubMemberCount, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	o.GitHubAccessToken, err = crypto.Decrypt(o.GitHubAccessToken, q.encryptionKey)
	return o, err
}

func (q *Queries) GetOrgByID(ctx context.Context, orgID string) (*Organization, error) {
	o := &Organization{}
	err := q.pool.QueryRow(ctx, `
		SELECT id, github_org_id, github_org_login, name, avatar_url,
		       github_access_token, owner_id,
		       github_member_count, created_at, updated_at
		FROM organizations WHERE id = $1
	`, orgID).Scan(
		&o.ID, &o.GitHubOrgID, &o.GitHubOrgLogin, &o.Name, &o.AvatarURL,
		&o.GitHubAccessToken, &o.OwnerID,
		&o.GithubMemberCount, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	o.GitHubAccessToken, err = crypto.Decrypt(o.GitHubAccessToken, q.encryptionKey)
	return o, err
}

func (q *Queries) GetOrgByLogin(ctx context.Context, login string) (*Organization, error) {
	o := &Organization{}
	err := q.pool.QueryRow(ctx, `
		SELECT id, github_org_id, github_org_login, name, avatar_url,
		       github_access_token, owner_id,
		       github_member_count, created_at, updated_at
		FROM organizations WHERE github_org_login = $1
		LIMIT 1
	`, login).Scan(
		&o.ID, &o.GitHubOrgID, &o.GitHubOrgLogin, &o.Name, &o.AvatarURL,
		&o.GitHubAccessToken, &o.OwnerID,
		&o.GithubMemberCount, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	o.GitHubAccessToken, err = crypto.Decrypt(o.GitHubAccessToken, q.encryptionKey)
	return o, err
}

func (q *Queries) CreateOrg(ctx context.Context, githubOrgID, githubOrgLogin, name string, avatarURL *string, accessToken, ownerID string) (*Organization, error) {
	encToken, err := crypto.Encrypt(accessToken, q.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt access token: %w", err)
	}
	o := &Organization{}
	err = q.pool.QueryRow(ctx, `
		INSERT INTO organizations
			(github_org_id, github_org_login, name, avatar_url, github_access_token, owner_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, github_org_id, github_org_login, name, avatar_url,
		          github_access_token, owner_id,
		          github_member_count, created_at, updated_at
	`, githubOrgID, githubOrgLogin, name, avatarURL, encToken, ownerID).Scan(
		&o.ID, &o.GitHubOrgID, &o.GitHubOrgLogin, &o.Name, &o.AvatarURL,
		&o.GitHubAccessToken, &o.OwnerID,
		&o.GithubMemberCount, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	o.GitHubAccessToken, err = crypto.Decrypt(o.GitHubAccessToken, q.encryptionKey)
	return o, err
}

func (q *Queries) UpdateOrgOwner(ctx context.Context, orgID, ownerID string) error {
	_, err := q.pool.Exec(ctx, `UPDATE organizations SET owner_id = $1 WHERE id = $2`, ownerID, orgID)
	return err
}

func (q *Queries) DisconnectOrg(ctx context.Context, orgID string) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	return err
}

func (q *Queries) UpdateOrgMemberCount(ctx context.Context, orgID string, count int) error {
	_, err := q.pool.Exec(ctx, `
		UPDATE organizations SET github_member_count = $2, updated_at = now() WHERE id = $1
	`, orgID, count)
	return err
}

// ---- Copilot Usage Snapshots ----

type BreakdownItem struct {
	Language         string `json:"language"`
	Editor           string `json:"editor"`
	SuggestionsCount int    `json:"suggestions_count"`
	AcceptancesCount int    `json:"acceptances_count"`
	LinesSuggested   int    `json:"lines_suggested"`
	LinesAccepted    int    `json:"lines_accepted"`
	ActiveUsers      int    `json:"active_users"`
}

type UsageSnapshot struct {
	ID                 string
	OrganizationID     string
	Date               time.Time
	TotalSuggestions   int
	TotalAcceptances   int
	TotalPlanningUses  int
	TotalCodingUses    int
	TotalLinesAccepted int
	TotalActiveUsers   int
	TotalLicensedUsers int
	BreakdownJSON      []BreakdownItem
	CreatedAt          time.Time
}

func (q *Queries) GetSnapshots(ctx context.Context, orgID string, limit int) ([]UsageSnapshot, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, organization_id, date, total_suggestions, total_acceptances,
		       total_lines_accepted, total_active_users, total_licensed_users,
		       breakdown_json, created_at
		FROM copilot_usage_snapshots
		WHERE organization_id = $1
		ORDER BY date ASC
		LIMIT $2
	`, orgID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []UsageSnapshot
	for rows.Next() {
		var s UsageSnapshot
		var bdRaw []byte
		if err := rows.Scan(
			&s.ID, &s.OrganizationID, &s.Date,
			&s.TotalSuggestions, &s.TotalAcceptances, &s.TotalLinesAccepted,
			&s.TotalActiveUsers, &s.TotalLicensedUsers,
			&bdRaw, &s.CreatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(bdRaw, &s.BreakdownJSON)
		out = append(out, s)
	}
	return out, rows.Err()
}

func (q *Queries) GetSnapshotsByMonth(ctx context.Context, orgID, month string) ([]UsageSnapshot, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, organization_id, date, total_suggestions, total_acceptances,
		       total_lines_accepted, total_active_users, total_licensed_users,
		       breakdown_json, created_at
		FROM copilot_usage_snapshots
		WHERE organization_id = $1
		  AND to_char(date, 'YYYY-MM') = $2
		ORDER BY date ASC
	`, orgID, month)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []UsageSnapshot
	for rows.Next() {
		var s UsageSnapshot
		var bdRaw []byte
		if err := rows.Scan(
			&s.ID, &s.OrganizationID, &s.Date,
			&s.TotalSuggestions, &s.TotalAcceptances, &s.TotalLinesAccepted,
			&s.TotalActiveUsers, &s.TotalLicensedUsers,
			&bdRaw, &s.CreatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(bdRaw, &s.BreakdownJSON)
		out = append(out, s)
	}
	return out, rows.Err()
}

func (q *Queries) UpsertSnapshot(ctx context.Context, orgID string, date time.Time, totalSuggestions, totalAcceptances, totalLinesAccepted, totalActiveUsers, totalLicensedUsers int, breakdown []BreakdownItem) error {
	bdJSON, err := json.Marshal(breakdown)
	if err != nil {
		return err
	}
	_, err = q.pool.Exec(ctx, `
		INSERT INTO copilot_usage_snapshots
			(organization_id, date, total_suggestions, total_acceptances,
			 total_lines_accepted, total_active_users, total_licensed_users, breakdown_json)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (organization_id, date) DO UPDATE SET
			total_suggestions    = EXCLUDED.total_suggestions,
			total_acceptances    = EXCLUDED.total_acceptances,
			total_lines_accepted = EXCLUDED.total_lines_accepted,
			total_active_users   = EXCLUDED.total_active_users,
			total_licensed_users = EXCLUDED.total_licensed_users,
			breakdown_json       = EXCLUDED.breakdown_json
	`, orgID, date, totalSuggestions, totalAcceptances, totalLinesAccepted, totalActiveUsers, totalLicensedUsers, bdJSON)
	return err
}

// BulkUpsertSnapshots inserts multiple snapshots efficiently.
func (q *Queries) BulkUpsertSnapshots(ctx context.Context, snapshots []UsageSnapshot) error {
	for _, s := range snapshots {
		if err := q.UpsertSnapshot(ctx, s.OrganizationID, s.Date,
			s.TotalSuggestions, s.TotalAcceptances, s.TotalLinesAccepted,
			s.TotalActiveUsers, s.TotalLicensedUsers, s.BreakdownJSON,
		); err != nil {
			return err
		}
	}
	return nil
}

// ---- Tool Usage Snapshots (multi-source) ----

// ToolUsageSnapshot represents a row in tool_usage_snapshots.
type ToolUsageSnapshot struct {
	ID                    string
	OrganizationID        string
	ToolSource            string
	Date                  time.Time
	TotalSuggestions      int
	TotalAcceptances      int
	TotalLinesAccepted    int
	TotalActiveUsers      int
	TotalLicensedUsers    int
	TotalCodingDevelopers int
	TotalPlanningUses     int
	TotalCodingUses       int
	RawJSON               json.RawMessage
	CreatedAt             time.Time
}

// GetSnapshotsBySource returns snapshots for a specific tool source over the last `days` days.
func (q *Queries) GetSnapshotsBySource(ctx context.Context, orgID, toolSource string, days int) ([]ToolUsageSnapshot, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, organization_id, tool_source, date,
		       total_suggestions, total_acceptances, total_lines_accepted,
		       total_active_users, total_licensed_users, total_coding_developers,
		       total_planning_uses, total_coding_uses,
		       raw_json, created_at
		FROM tool_usage_snapshots
		WHERE organization_id = $1
		  AND tool_source     = $2
		  AND date >= CURRENT_DATE - ($3::int - 1)
		ORDER BY date ASC
	`, orgID, toolSource, days)
	if err != nil {
		return nil, fmt.Errorf("GetSnapshotsBySource: %w", err)
	}
	defer rows.Close()

	var out []ToolUsageSnapshot
	for rows.Next() {
		var s ToolUsageSnapshot
		if err := rows.Scan(
			&s.ID, &s.OrganizationID, &s.ToolSource, &s.Date,
			&s.TotalSuggestions, &s.TotalAcceptances, &s.TotalLinesAccepted,
			&s.TotalActiveUsers, &s.TotalLicensedUsers, &s.TotalCodingDevelopers,
			&s.TotalPlanningUses, &s.TotalCodingUses,
			&s.RawJSON, &s.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("GetSnapshotsBySource scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetAllSnapshots returns snapshots for all tool sources over the last `days` days,
// aggregated per date (totals summed, active users summed across sources).
func (q *Queries) GetAllSnapshots(ctx context.Context, orgID string, days int) ([]ToolUsageSnapshot, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT
		    gen_random_uuid()::text          AS id,
		    $1::text                         AS organization_id,
		    'all'                            AS tool_source,
		    date,
		    SUM(total_suggestions)           AS total_suggestions,
		    SUM(total_acceptances)           AS total_acceptances,
		    SUM(total_lines_accepted)        AS total_lines_accepted,
		    SUM(total_active_users)          AS total_active_users,
		    SUM(total_licensed_users)        AS total_licensed_users,
		    MAX(total_coding_developers)     AS total_coding_developers,
		    SUM(total_planning_uses)         AS total_planning_uses,
		    SUM(total_coding_uses)           AS total_coding_uses,
		    '{}'::jsonb                      AS raw_json,
		    MIN(created_at)                  AS created_at
		FROM tool_usage_snapshots
		WHERE organization_id = $1
		  AND date >= CURRENT_DATE - ($2::int - 1)
		GROUP BY date
		ORDER BY date ASC
	`, orgID, days)
	if err != nil {
		return nil, fmt.Errorf("GetAllSnapshots: %w", err)
	}
	defer rows.Close()

	var out []ToolUsageSnapshot
	for rows.Next() {
		var s ToolUsageSnapshot
		if err := rows.Scan(
			&s.ID, &s.OrganizationID, &s.ToolSource, &s.Date,
			&s.TotalSuggestions, &s.TotalAcceptances, &s.TotalLinesAccepted,
			&s.TotalActiveUsers, &s.TotalLicensedUsers, &s.TotalCodingDevelopers,
			&s.TotalPlanningUses, &s.TotalCodingUses,
			&s.RawJSON, &s.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("GetAllSnapshots scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetUtilizationStats returns the aggregate AI user count and total coding developers
// over the last `days` days, across all tool sources.
// Returns (aiUsers, totalDevs) as the numerator and best-estimate denominator.
// Both are peak-day headcounts: daily counts are not summed across days, since
// the same developers show up every day.
func (q *Queries) GetUtilizationStats(ctx context.Context, orgID string, days int) (aiUsers, totalDevs int, err error) {
	err = q.pool.QueryRow(ctx, `
		SELECT
		    COALESCE(MAX(day_users), 0) AS ai_users,
		    COALESCE(MAX(day_devs), 0)  AS total_devs
		FROM (
		    SELECT SUM(total_active_users)      AS day_users,
		           MAX(total_coding_developers) AS day_devs
		    FROM tool_usage_snapshots
		    WHERE organization_id = $1
		      AND date >= CURRENT_DATE - ($2::int - 1)
		    GROUP BY date
		) per_day
	`, orgID, days).Scan(&aiUsers, &totalDevs)
	if err != nil {
		return 0, 0, fmt.Errorf("GetUtilizationStats: %w", err)
	}
	return aiUsers, totalDevs, nil
}

// UtilizationPoint holds a single daily utilization percentage for trend charting.
type UtilizationPoint struct {
	Date               string  `json:"date"`
	UtilizationPercent float64 `json:"utilization_percent"`
}

// GetUtilizationTrend returns a daily utilization percentage series for the
// last `days` days across all tool sources.
func (q *Queries) GetUtilizationTrend(ctx context.Context, orgID string, days int) ([]UtilizationPoint, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT
		    date::text AS date,
		    COALESCE(ROUND(
		        (SUM(total_active_users)::numeric /
		         NULLIF(MAX(total_coding_developers), 0)) * 100, 1
		    ), 0) AS utilization_percent
		FROM tool_usage_snapshots
		WHERE organization_id = $1
		  AND date >= CURRENT_DATE - ($2::int - 1)
		GROUP BY date
		ORDER BY date ASC
	`, orgID, days)
	if err != nil {
		return []UtilizationPoint{}, fmt.Errorf("GetUtilizationTrend: %w", err)
	}
	defer rows.Close()

	out := []UtilizationPoint{}
	for rows.Next() {
		var p UtilizationPoint
		if err := rows.Scan(&p.Date, &p.UtilizationPercent); err != nil {
			return []UtilizationPoint{}, fmt.Errorf("GetUtilizationTrend scan: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return []UtilizationPoint{}, fmt.Errorf("GetUtilizationTrend rows: %w", err)
	}
	return out, nil
}

// GetUtilizationStatsPriorWindow returns the aggregate AI user count and total
// coding developers for the prior 30-day window (days -59 to -29), as peak-day
// headcounts like GetUtilizationStats.
func (q *Queries) GetUtilizationStatsPriorWindow(ctx context.Context, orgID string) (aiUsers, totalDevs int, err error) {
	err = q.pool.QueryRow(ctx, `
		SELECT
		    COALESCE(MAX(day_users), 0) AS ai_users,
		    COALESCE(MAX(day_devs), 0)  AS total_devs
		FROM (
		    SELECT SUM(total_active_users)      AS day_users,
		           MAX(total_coding_developers) AS day_devs
		    FROM tool_usage_snapshots
		    WHERE organization_id = $1
		      AND date >= CURRENT_DATE - 59
		      AND date < CURRENT_DATE - 29
		    GROUP BY date
		) per_day
	`, orgID).Scan(&aiUsers, &totalDevs)
	if err != nil {
		return 0, 0, fmt.Errorf("GetUtilizationStatsPriorWindow: %w", err)
	}
	return aiUsers, totalDevs, nil
}

// GetActiveToolSources returns distinct tool_source values that have had at least one
// snapshot for this org in the last 90 days. Used to determine which tools to sync.
func (q *Queries) GetActiveToolSources(ctx context.Context, orgID string) ([]string, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT DISTINCT tool_source
		FROM tool_usage_snapshots
		WHERE organization_id = $1
		  AND date >= CURRENT_DATE - 89
		ORDER BY tool_source
	`, orgID)
	if err != nil {
		return nil, fmt.Errorf("GetActiveToolSources: %w", err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, fmt.Errorf("GetActiveToolSources scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetCombinedSnapshots aggregates both copilot_usage_snapshots and tool_usage_snapshots
// by date, returning the most recent `limit` days, oldest first, in the UsageSnapshot
// shape the dashboard expects. This lets the
// dashboard show accurate metrics regardless of which tools the org uses.
// Copilot breakdown_json is preserved when present; other tools contribute to totals only.
func (q *Queries) GetCombinedSnapshots(ctx context.Context, orgID string, limit int) ([]UsageSnapshot, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT
		    gen_random_uuid()::text                   AS id,
		    $1::text                                  AS organization_id,
		    combined.date,
		    combined.total_suggestions,
		    combined.total_acceptances,
		    combined.total_lines_accepted,
		    combined.total_active_users,
		    combined.total_licensed_users,
		    combined.total_planning_uses,
		    combined.total_coding_uses,
		    COALESCE(cop.breakdown_json, '[]'::jsonb) AS breakdown_json,
		    combined.created_at
		FROM (
		    SELECT date,
		           SUM(total_suggestions)    AS total_suggestions,
		           SUM(total_acceptances)    AS total_acceptances,
		           SUM(total_lines_accepted) AS total_lines_accepted,
		           SUM(total_active_users)   AS total_active_users,
		           SUM(total_licensed_users) AS total_licensed_users,
		           SUM(total_planning_uses)  AS total_planning_uses,
		           SUM(total_coding_uses)    AS total_coding_uses,
		           MIN(created_at)           AS created_at
		    FROM (
		        SELECT date, total_suggestions, total_acceptances, total_lines_accepted,
		               total_active_users, total_licensed_users, 0 AS total_planning_uses,
		               0 AS total_coding_uses, created_at
		        FROM copilot_usage_snapshots
		        WHERE organization_id = $1
		        UNION ALL
		        SELECT date, total_suggestions, total_acceptances, total_lines_accepted,
		               total_active_users, total_licensed_users, total_planning_uses,
		               total_coding_uses, created_at
		        FROM tool_usage_snapshots
		        WHERE organization_id = $1
		    ) all_data
		    GROUP BY date
		    -- Keep the most recent days; the outer query re-sorts them ascending.
		    ORDER BY date DESC
		    LIMIT $2
		) combined
		LEFT JOIN copilot_usage_snapshots cop
		       ON cop.organization_id = $1 AND cop.date = combined.date
		ORDER BY combined.date ASC
	`, orgID, limit)
	if err != nil {
		return nil, fmt.Errorf("GetCombinedSnapshots: %w", err)
	}
	defer rows.Close()

	var out []UsageSnapshot
	for rows.Next() {
		var s UsageSnapshot
		var bdJSON []byte
		if err := rows.Scan(
			&s.ID, &s.OrganizationID, &s.Date,
			&s.TotalSuggestions, &s.TotalAcceptances, &s.TotalLinesAccepted,
			&s.TotalActiveUsers, &s.TotalLicensedUsers,
			&s.TotalPlanningUses, &s.TotalCodingUses,
			&bdJSON, &s.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("GetCombinedSnapshots scan: %w", err)
		}
		if len(bdJSON) > 0 && string(bdJSON) != "[]" {
			if err := json.Unmarshal(bdJSON, &s.BreakdownJSON); err != nil {
				return nil, fmt.Errorf("GetCombinedSnapshots breakdown decode: %w", err)
			}
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ToolBreakdownRow holds per-tool aggregate stats for the insights breakdown panel.
type ToolBreakdownRow struct {
	ToolSource    string `json:"tool_source"`
	ActiveUsers   int    `json:"active_users"`
	LinesAccepted int    `json:"lines_accepted"`
}

// GetToolBreakdown returns per-tool peak daily active users and total lines
// accepted for the last `days` days, ordered by active users descending.
func (q *Queries) GetToolBreakdown(ctx context.Context, orgID string, days int) ([]ToolBreakdownRow, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT
		    tool_source,
		    MAX(total_active_users)::int   AS active_users,
		    SUM(total_lines_accepted)::int AS lines_accepted
		FROM tool_usage_snapshots
		WHERE organization_id = $1
		  AND date >= CURRENT_DATE - ($2::int - 1)
		GROUP BY tool_source
		ORDER BY active_users DESC
	`, orgID, days)
	if err != nil {
		return []ToolBreakdownRow{}, fmt.Errorf("GetToolBreakdown: %w", err)
	}
	defer rows.Close()

	out := []ToolBreakdownRow{}
	for rows.Next() {
		var r ToolBreakdownRow
		if err := rows.Scan(&r.ToolSource, &r.ActiveUsers, &r.LinesAccepted); err != nil {
			return []ToolBreakdownRow{}, fmt.Errorf("GetToolBreakdown scan: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return []ToolBreakdownRow{}, fmt.Errorf("GetToolBreakdown rows: %w", err)
	}
	return out, nil
}

// GetWeeklyActiveUsers returns the max active users for this week (last 7 days)
// and the prior week (days -13 to -7), unioning copilot and tool snapshots.
func (q *Queries) GetWeeklyActiveUsers(ctx context.Context, orgID string) (int, int, error) {
	var thisWeek, prevWeek int
	err := q.pool.QueryRow(ctx, `
		SELECT
		  COALESCE(MAX(CASE WHEN date >= CURRENT_DATE - 6 THEN total_active_users END), 0) AS this_week,
		  COALESCE(MAX(CASE WHEN date >= CURRENT_DATE - 13 AND date < CURRENT_DATE - 6 THEN total_active_users END), 0) AS prev_week
		FROM (
		  SELECT date, total_active_users FROM copilot_usage_snapshots WHERE organization_id = $1 AND date >= CURRENT_DATE - 13
		  UNION ALL
		  SELECT date, total_active_users FROM tool_usage_snapshots WHERE organization_id = $1 AND date >= CURRENT_DATE - 13
		) combined
	`, orgID).Scan(&thisWeek, &prevWeek)
	if err != nil {
		return 0, 0, fmt.Errorf("GetWeeklyActiveUsers: %w", err)
	}
	return thisWeek, prevWeek, nil
}

// UpsertToolSnapshot inserts or updates a row in tool_usage_snapshots.
func (q *Queries) UpsertToolSnapshot(
	ctx context.Context,
	orgID, toolSource string,
	date time.Time,
	totalSuggestions, totalAcceptances, totalLinesAccepted, totalActiveUsers, totalLicensedUsers, totalCodingDevelopers int,
	totalPlanningUses, totalCodingUses int,
	rawJSON json.RawMessage,
) error {
	if rawJSON == nil {
		rawJSON = json.RawMessage("{}")
	}
	_, err := q.pool.Exec(ctx, `
		INSERT INTO tool_usage_snapshots
		    (organization_id, tool_source, date,
		     total_suggestions, total_acceptances, total_lines_accepted,
		     total_active_users, total_licensed_users, total_coding_developers,
		     total_planning_uses, total_coding_uses, raw_json)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (organization_id, tool_source, date) DO UPDATE SET
		    total_suggestions       = tool_usage_snapshots.total_suggestions       + EXCLUDED.total_suggestions,
		    total_acceptances       = tool_usage_snapshots.total_acceptances       + EXCLUDED.total_acceptances,
		    total_lines_accepted    = tool_usage_snapshots.total_lines_accepted    + EXCLUDED.total_lines_accepted,
		    total_active_users      = GREATEST(tool_usage_snapshots.total_active_users, EXCLUDED.total_active_users),
		    total_licensed_users    = GREATEST(tool_usage_snapshots.total_licensed_users, EXCLUDED.total_licensed_users),
		    total_coding_developers = GREATEST(tool_usage_snapshots.total_coding_developers, EXCLUDED.total_coding_developers),
		    total_planning_uses     = tool_usage_snapshots.total_planning_uses     + EXCLUDED.total_planning_uses,
		    total_coding_uses       = tool_usage_snapshots.total_coding_uses       + EXCLUDED.total_coding_uses,
		    raw_json                = EXCLUDED.raw_json
	`, orgID, toolSource, date,
		totalSuggestions, totalAcceptances, totalLinesAccepted,
		totalActiveUsers, totalLicensedUsers, totalCodingDevelopers,
		totalPlanningUses, totalCodingUses, rawJSON)
	if err != nil {
		return fmt.Errorf("UpsertToolSnapshot: %w", err)
	}
	return nil
}

// ---- Reports ----

type Report struct {
	ID             string
	OrganizationID string
	Month          string
	ContentJSON    json.RawMessage
	ShareToken     *string
	CreatedAt      time.Time
}

func (q *Queries) ListReports(ctx context.Context, orgID string, limit int) ([]Report, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, organization_id, month, content_json, share_token, created_at
		FROM reports
		WHERE organization_id = $1
		ORDER BY month DESC
		LIMIT $2
	`, orgID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Report
	for rows.Next() {
		var r Report
		if err := rows.Scan(&r.ID, &r.OrganizationID, &r.Month, &r.ContentJSON, &r.ShareToken, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (q *Queries) UpsertReport(ctx context.Context, orgID, month string, content interface{}) (*Report, error) {
	contentJSON, err := json.Marshal(content)
	if err != nil {
		return nil, err
	}
	r := &Report{}
	err = q.pool.QueryRow(ctx, `
		INSERT INTO reports (organization_id, month, content_json)
		VALUES ($1, $2, $3)
		ON CONFLICT (organization_id, month) DO UPDATE SET
			content_json = EXCLUDED.content_json
		RETURNING id, organization_id, month, content_json, share_token, created_at
	`, orgID, month, contentJSON).Scan(
		&r.ID, &r.OrganizationID, &r.Month, &r.ContentJSON, &r.ShareToken, &r.CreatedAt,
	)
	return r, err
}

// UpsertReportWithToken inserts or updates a report row, always writing the
// supplied shareToken. On conflict the content and token are both refreshed so
// that re-generating a report within the same date range issues a new token.
func (q *Queries) UpsertReportWithToken(ctx context.Context, orgID, month, shareToken string, content interface{}) (*Report, error) {
	contentJSON, err := json.Marshal(content)
	if err != nil {
		return nil, fmt.Errorf("UpsertReportWithToken marshal: %w", err)
	}
	r := &Report{}
	err = q.pool.QueryRow(ctx, `
		INSERT INTO reports (organization_id, month, content_json, share_token)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (organization_id, month) DO UPDATE SET
			content_json = EXCLUDED.content_json,
			share_token  = EXCLUDED.share_token
		RETURNING id, organization_id, month, content_json, share_token, created_at
	`, orgID, month, contentJSON, shareToken).Scan(
		&r.ID, &r.OrganizationID, &r.Month, &r.ContentJSON, &r.ShareToken, &r.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("UpsertReportWithToken: %w", err)
	}
	return r, nil
}

// ---- Ingest API Keys ----

// IngestAPIKey represents a row in ingest_api_keys.
type IngestAPIKey struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"-"` // never expose org ID to clients
	KeyHash        string     `json:"-"` // hash is internal — raw key shown once at creation only
	Label          string     `json:"label"`
	CreatedAt      time.Time  `json:"created_at"`
	LastUsedAt     *time.Time `json:"last_used_at"`
}

// CreateIngestKey inserts a new ingest API key row. keyHash must be the
// SHA-256 hex of the raw key. The raw key is never stored here.
func (q *Queries) CreateIngestKey(ctx context.Context, orgID, keyHash, label string) (*IngestAPIKey, error) {
	k := &IngestAPIKey{}
	err := q.pool.QueryRow(ctx, `
		INSERT INTO ingest_api_keys (organization_id, key_hash, label)
		VALUES ($1, $2, $3)
		RETURNING id, organization_id, key_hash, label, created_at, last_used_at
	`, orgID, keyHash, label).Scan(
		&k.ID, &k.OrganizationID, &k.KeyHash, &k.Label, &k.CreatedAt, &k.LastUsedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("db.CreateIngestKey: %w", err)
	}
	return k, nil
}

// GetIngestKeyByHash looks up an ingest API key by its SHA-256 hash.
// Called on every authenticated ingest request.
func (q *Queries) GetIngestKeyByHash(ctx context.Context, keyHash string) (*IngestAPIKey, error) {
	k := &IngestAPIKey{}
	err := q.pool.QueryRow(ctx, `
		SELECT id, organization_id, key_hash, label, created_at, last_used_at
		FROM ingest_api_keys
		WHERE key_hash = $1
	`, keyHash).Scan(
		&k.ID, &k.OrganizationID, &k.KeyHash, &k.Label, &k.CreatedAt, &k.LastUsedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("db.GetIngestKeyByHash: %w", err)
	}
	return k, nil
}

// UpdateIngestKeyLastUsed sets last_used_at = now() for the given key ID.
func (q *Queries) UpdateIngestKeyLastUsed(ctx context.Context, id string) error {
	_, err := q.pool.Exec(ctx, `
		UPDATE ingest_api_keys SET last_used_at = now() WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("db.UpdateIngestKeyLastUsed: %w", err)
	}
	return nil
}

// ListIngestKeys returns all ingest API keys for an organization, ordered by
// creation time. The raw key is never stored; only the hash is present.
func (q *Queries) ListIngestKeys(ctx context.Context, orgID string) ([]IngestAPIKey, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, organization_id, key_hash, label, created_at, last_used_at
		FROM ingest_api_keys
		WHERE organization_id = $1
		ORDER BY created_at ASC
	`, orgID)
	if err != nil {
		return nil, fmt.Errorf("db.ListIngestKeys: %w", err)
	}
	defer rows.Close()

	out := []IngestAPIKey{}
	for rows.Next() {
		var k IngestAPIKey
		if err := rows.Scan(
			&k.ID, &k.OrganizationID, &k.KeyHash, &k.Label, &k.CreatedAt, &k.LastUsedAt,
		); err != nil {
			return nil, fmt.Errorf("db.ListIngestKeys scan: %w", err)
		}
		out = append(out, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db.ListIngestKeys rows: %w", err)
	}
	return out, nil
}

// DeleteIngestKey removes a key by id scoped to orgID so that one org cannot
// delete another org's keys. Returns an error if the row was not found.
func (q *Queries) DeleteIngestKey(ctx context.Context, id, orgID string) error {
	tag, err := q.pool.Exec(ctx, `
		DELETE FROM ingest_api_keys WHERE id = $1 AND organization_id = $2
	`, id, orgID)
	if err != nil {
		return fmt.Errorf("db.DeleteIngestKey: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("db.DeleteIngestKey: not found")
	}
	return nil
}

// GetReportByShareToken returns the report row identified by share_token.
// No org scoping is applied — the token itself is the access credential.
// Returns pgx.ErrNoRows when no matching row exists.
func (q *Queries) GetReportByShareToken(ctx context.Context, shareToken string) (Report, error) {
	var r Report
	err := q.pool.QueryRow(ctx, `
		SELECT id, organization_id, month, content_json, share_token, created_at
		FROM reports
		WHERE share_token = $1
	`, shareToken).Scan(
		&r.ID, &r.OrganizationID, &r.Month, &r.ContentJSON, &r.ShareToken, &r.CreatedAt,
	)
	if err != nil {
		return Report{}, fmt.Errorf("db.GetReportByShareToken: %w", err)
	}
	return r, nil
}

// GetSnapshotsByDateRange returns copilot usage snapshots for an org between
// startDate and endDate inclusive, ordered by date ascending.
func (q *Queries) GetSnapshotsByDateRange(ctx context.Context, orgID string, startDate, endDate time.Time) ([]UsageSnapshot, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, organization_id, date, total_suggestions, total_acceptances,
		       total_lines_accepted, total_active_users, total_licensed_users,
		       breakdown_json, created_at
		FROM copilot_usage_snapshots
		WHERE organization_id = $1
		  AND date >= $2
		  AND date <= $3
		ORDER BY date ASC
	`, orgID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("db.GetSnapshotsByDateRange: %w", err)
	}
	defer rows.Close()

	var out []UsageSnapshot
	for rows.Next() {
		var s UsageSnapshot
		var bdRaw []byte
		if err := rows.Scan(
			&s.ID, &s.OrganizationID, &s.Date,
			&s.TotalSuggestions, &s.TotalAcceptances, &s.TotalLinesAccepted,
			&s.TotalActiveUsers, &s.TotalLicensedUsers,
			&bdRaw, &s.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("db.GetSnapshotsByDateRange scan: %w", err)
		}
		_ = json.Unmarshal(bdRaw, &s.BreakdownJSON)
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db.GetSnapshotsByDateRange rows: %w", err)
	}
	return out, nil
}

// GetToolSnapshotsByDateRange returns tool_usage_snapshots rows for an org
// between startDate and endDate inclusive, ordered by date ascending.
// All sources are returned; callers may filter by ToolSource if needed.
func (q *Queries) GetToolSnapshotsByDateRange(ctx context.Context, orgID string, startDate, endDate time.Time) ([]ToolUsageSnapshot, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, organization_id, tool_source, date,
		       total_suggestions, total_acceptances, total_lines_accepted,
		       total_active_users, total_licensed_users, total_coding_developers,
		       total_planning_uses, total_coding_uses,
		       raw_json, created_at
		FROM tool_usage_snapshots
		WHERE organization_id = $1
		  AND date >= $2
		  AND date <= $3
		ORDER BY date ASC
	`, orgID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("db.GetToolSnapshotsByDateRange: %w", err)
	}
	defer rows.Close()

	var out []ToolUsageSnapshot
	for rows.Next() {
		var s ToolUsageSnapshot
		if err := rows.Scan(
			&s.ID, &s.OrganizationID, &s.ToolSource, &s.Date,
			&s.TotalSuggestions, &s.TotalAcceptances, &s.TotalLinesAccepted,
			&s.TotalActiveUsers, &s.TotalLicensedUsers, &s.TotalCodingDevelopers,
			&s.TotalPlanningUses, &s.TotalCodingUses,
			&s.RawJSON, &s.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("db.GetToolSnapshotsByDateRange scan: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db.GetToolSnapshotsByDateRange rows: %w", err)
	}
	return out, nil
}

// ---- helpers ----

func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
