package github

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
)

// ---- GitHub API response types ----

type ghRepo struct {
	Name string `json:"name"`
}

type ghCommitListItem struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Date string `json:"date"`
		} `json:"author"`
	} `json:"commit"`
	Author *struct {
		Login string `json:"login"`
	} `json:"author"`
}

type ghCommitDetail struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
	} `json:"commit"`
	Author *struct {
		Login string `json:"login"`
	} `json:"author"`
	Stats struct {
		Additions int `json:"additions"`
		Deletions int `json:"deletions"`
	} `json:"stats"`
}

// attributedCommit holds the data we collect per Claude-attributed commit.
type attributedCommit struct {
	SHA      string
	Author   string
	NetLines int
	Date     time.Time
}

// ghClient is a shared HTTP client with a timeout for all GitHub API calls.
// Using http.DefaultClient (no timeout) would leave goroutines blocked
// indefinitely during GitHub outages or rate-limit storms.
var ghClient = &http.Client{Timeout: 30 * time.Second}

// ---- HTTP helpers ----

// ghGet performs an authenticated GET against the GitHub API.
// Returns (body bytes, status code, error).
// The caller is responsible for checking the status code.
func ghGet(ctx context.Context, url, token string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("ghGet new request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := ghClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("ghGet do: %w", err)
	}
	defer resp.Body.Close()

	var buf []byte
	tmp := make([]byte, 4096)
	for {
		n, readErr := resp.Body.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if readErr != nil {
			break
		}
	}
	return buf, resp.StatusCode, nil
}

// ---- Repo listing ----

func listOrgRepos(ctx context.Context, org, token string) ([]string, error) {
	var names []string
	page := 1
	for {
		url := fmt.Sprintf("https://api.github.com/orgs/%s/repos?per_page=100&page=%d", org, page)
		body, status, err := ghGet(ctx, url, token)
		if err != nil {
			return nil, fmt.Errorf("listOrgRepos page %d: %w", page, err)
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("listOrgRepos: unexpected status %d", status)
		}

		var repos []ghRepo
		if err := json.Unmarshal(body, &repos); err != nil {
			return nil, fmt.Errorf("listOrgRepos decode: %w", err)
		}
		for _, r := range repos {
			names = append(names, r.Name)
		}
		if len(repos) < 100 {
			break // last page
		}
		page++
	}
	return names, nil
}

// ---- Commit listing ----

func listRecentCommits(ctx context.Context, org, repo, token string, since time.Time) ([]ghCommitListItem, error) {
	sinceStr := since.UTC().Format(time.RFC3339)
	url := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/commits?since=%s&per_page=100",
		org, repo, sinceStr,
	)
	body, status, err := ghGet(ctx, url, token)
	if err != nil {
		return nil, fmt.Errorf("listRecentCommits %s/%s: %w", org, repo, err)
	}
	switch status {
	case http.StatusOK:
		// continue
	case http.StatusForbidden, http.StatusTooManyRequests:
		// Rate limited — return nil so the caller can skip and continue.
		return nil, nil
	case http.StatusNotFound:
		// Repo may be empty or inaccessible — skip silently.
		return nil, nil
	default:
		return nil, fmt.Errorf("listRecentCommits %s/%s: unexpected status %d", org, repo, status)
	}

	var commits []ghCommitListItem
	if err := json.Unmarshal(body, &commits); err != nil {
		return nil, fmt.Errorf("listRecentCommits decode %s/%s: %w", org, repo, err)
	}
	return commits, nil
}

// ---- Single commit detail ----

// fetchCommitDetail retrieves full commit stats for a single SHA.
// Returns nil, nil when the request is rate-limited (403/429).
func fetchCommitDetail(ctx context.Context, org, repo, sha, token string) (*ghCommitDetail, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/%s", org, repo, sha)
	body, status, err := ghGet(ctx, url, token)
	if err != nil {
		return nil, fmt.Errorf("fetchCommitDetail %s: %w", sha, err)
	}
	switch status {
	case http.StatusOK:
		// continue
	case http.StatusForbidden, http.StatusTooManyRequests:
		return nil, nil // signal: rate limited
	default:
		return nil, fmt.Errorf("fetchCommitDetail %s: unexpected status %d", sha, status)
	}

	var detail ghCommitDetail
	if err := json.Unmarshal(body, &detail); err != nil {
		return nil, fmt.Errorf("fetchCommitDetail decode %s: %w", sha, err)
	}
	return &detail, nil
}

// ---- Attribution check ----

// isClaudeAttributed returns true if the commit message contains a Co-Authored-By
// trailer referencing Anthropic or Claude Code.
func isClaudeAttributed(message string) bool {
	lower := strings.ToLower(message)
	lines := strings.Split(lower, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "co-authored-by:") {
			if strings.Contains(trimmed, "anthropic.com") || strings.Contains(trimmed, "claude") {
				return true
			}
		}
	}
	return false
}

// ---- Raw JSON payload ----

type claudeCodeRawJSON struct {
	AttributedCommits []string `json:"attributed_commits"`
	Confidence        string   `json:"confidence"`
}

// ---- Main sync function ----

// claudeCodeStore is the subset of db.Queries used by SyncClaudeCodeAttribution.
type claudeCodeStore interface {
	GetClaudeCodeAttributedSHAs(ctx context.Context, orgID string, date time.Time) ([]string, error)
	UpsertToolSnapshot(ctx context.Context, orgID, toolSource string, date time.Time,
		totalSuggestions, totalAcceptances, totalLinesAccepted,
		totalActiveUsers, totalLicensedUsers, totalCodingDevelopers int,
		totalPlanningUses, totalCodingUses int,
		rawJSON json.RawMessage) error
}

// SyncClaudeCodeAttribution scans the last 24 hours of commits across all repos in
// the GitHub org identified by orgLogin, identifies commits attributed to Claude Code
// via git trailers, aggregates them by date, and upserts the results into
// tool_usage_snapshots keyed by orgID (the internal UUID).
//
// UpsertToolSnapshot adds to the stored totals and consecutive runs scan
// overlapping windows, so commits already listed in a day's
// raw_json.attributed_commits are not counted again. Concurrent runs for the
// same org can still double-count; Service.SyncClaudeCode serialises them.
func SyncClaudeCodeAttribution(ctx context.Context, orgID, orgLogin, token string, queries claudeCodeStore) (int, error) {
	since := time.Now().UTC().Add(-24 * time.Hour)

	repos, err := listOrgRepos(ctx, orgLogin, token)
	if err != nil {
		return 0, fmt.Errorf("SyncClaudeCodeAttribution list repos: %w", err)
	}

	// counted caches, per day, the SHAs earlier runs already added to the totals.
	counted := map[string][]string{}
	countedSHAs := map[string]struct{}{}
	loadCounted := func(dayKey string) ([]string, error) {
		if shas, ok := counted[dayKey]; ok {
			return shas, nil
		}
		date, err := time.Parse("2006-01-02", dayKey)
		if err != nil {
			return nil, err
		}
		shas, err := queries.GetClaudeCodeAttributedSHAs(ctx, orgID, date)
		if err != nil {
			return nil, fmt.Errorf("SyncClaudeCodeAttribution load counted commits %s: %w", dayKey, err)
		}
		counted[dayKey] = shas
		for _, sha := range shas {
			countedSHAs[sha] = struct{}{}
		}
		return shas, nil
	}

	// Map from date string (YYYY-MM-DD) → accumulated data.
	type dayBucket struct {
		shas       map[string]struct{}
		authors    map[string]struct{} // Claude-attributed committers
		allAuthors map[string]struct{} // ALL committers (utilization denominator)
		netLines   int
	}
	buckets := map[string]*dayBucket{}

	// getOrCreateBucket returns the bucket for a given day, creating it if needed.
	getOrCreateBucket := func(dayKey string) *dayBucket {
		b, ok := buckets[dayKey]
		if !ok {
			b = &dayBucket{
				shas:       map[string]struct{}{},
				authors:    map[string]struct{}{},
				allAuthors: map[string]struct{}{},
			}
			buckets[dayKey] = b
		}
		return b
	}

	// seenSHAs deduplicates all commits across repos/branches to avoid double-counting.
	seenSHAs := map[string]struct{}{}

	for _, repoName := range repos {
		commits, err := listRecentCommits(ctx, orgLogin, repoName, token, since)
		if err != nil {
			// Log and continue — one inaccessible repo should not abort the sync.
			log.Printf("SyncClaudeCodeAttribution: skipping repo %s: %v", repoName, err)
			continue
		}
		if commits == nil {
			// Rate limited or empty — skip.
			continue
		}

		for _, c := range commits {
			if _, alreadySeen := seenSHAs[c.SHA]; alreadySeen {
				continue
			}
			seenSHAs[c.SHA] = struct{}{}

			// Parse commit date for bucket key.
			dateStr := c.Commit.Author.Date
			commitTime, parseErr := time.Parse(time.RFC3339, dateStr)
			if parseErr != nil {
				commitTime, parseErr = time.Parse("2006-01-02", dateStr)
				if parseErr != nil {
					log.Printf("SyncClaudeCodeAttribution: unparseable date %q for %s", dateStr, c.SHA)
					continue
				}
			}
			dayKey := commitTime.UTC().Format("2006-01-02")
			bucket := getOrCreateBucket(dayKey)

			// Track every committer regardless of AI attribution — this is the
			// denominator for the utilization metric.
			if c.Author != nil && c.Author.Login != "" {
				bucket.allAuthors[c.Author.Login] = struct{}{}
			}

			// Quick check: skip detail fetch for non-Claude commits.
			if !isClaudeAttributed(c.Commit.Message) {
				continue
			}

			// Already counted by an earlier run: keep its author in the distinct
			// count, but don't fetch it or add it to the totals again.
			if _, err := loadCounted(dayKey); err != nil {
				return 0, err
			}
			if _, ok := countedSHAs[c.SHA]; ok {
				if c.Author != nil && c.Author.Login != "" {
					bucket.authors[c.Author.Login] = struct{}{}
				}
				continue
			}

			detail, err := fetchCommitDetail(ctx, orgLogin, repoName, c.SHA, token)
			if err != nil {
				log.Printf("SyncClaudeCodeAttribution: detail fetch %s: %v", c.SHA, err)
				continue
			}
			if detail == nil {
				// Rate limited — skip remainder of this repo.
				break
			}

			// Re-check attribution on the full message (list endpoint may truncate).
			if !isClaudeAttributed(detail.Commit.Message) {
				continue
			}

			bucket.shas[c.SHA] = struct{}{}
			bucket.netLines += detail.Stats.Additions - detail.Stats.Deletions

			authorLogin := ""
			if detail.Author != nil {
				authorLogin = detail.Author.Login
			} else if c.Author != nil {
				authorLogin = c.Author.Login
			}
			if authorLogin != "" {
				bucket.authors[authorLogin] = struct{}{}
			}
		}
	}

	// Upsert one row per day into tool_usage_snapshots.
	upserted := 0
	for dayKey, bucket := range buckets {
		date, err := time.Parse("2006-01-02", dayKey)
		if err != nil {
			continue
		}

		// Keep the SHAs earlier runs counted so later runs keep skipping them.
		prevSHAs, err := loadCounted(dayKey)
		if err != nil {
			return upserted, err
		}
		shaList := make([]string, 0, len(prevSHAs)+len(bucket.shas))
		shaList = append(shaList, prevSHAs...)
		for sha := range bucket.shas {
			shaList = append(shaList, sha)
		}
		sort.Strings(shaList)

		rawPayload := claudeCodeRawJSON{
			AttributedCommits: shaList,
			Confidence:        "medium",
		}
		rawBytes, err := json.Marshal(rawPayload)
		if err != nil {
			return upserted, fmt.Errorf("SyncClaudeCodeAttribution marshal raw: %w", err)
		}

		numCommits := len(bucket.shas)
		numAIAuthors := len(bucket.authors)
		numAllDevs := len(bucket.allAuthors)

		if err := queries.UpsertToolSnapshot(
			ctx,
			orgID,
			"claude_code",
			date,
			0,               // total_suggestions — not applicable
			numCommits,      // total_acceptances = number of attributed commits
			bucket.netLines, // total_lines_accepted = net lines
			numAIAuthors,    // total_active_users = distinct Claude-attributed authors
			0,               // total_licensed_users — unknown
			numAllDevs,      // total_coding_developers = all unique committers that day
			0,               // total_planning_uses — not tracked from commit attribution
			0,               // total_coding_uses — not tracked from commit attribution
			json.RawMessage(rawBytes),
		); err != nil {
			return upserted, fmt.Errorf("SyncClaudeCodeAttribution upsert %s: %w", dayKey, err)
		}
		upserted++
	}

	return upserted, nil
}
