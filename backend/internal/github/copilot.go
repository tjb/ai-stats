package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/tjb/ai-stats/internal/db"
)

// DailyUsage mirrors the GitHub Copilot Usage API response shape.
type DailyUsage struct {
	Day                      string          `json:"day"`
	TotalSuggestionsCount    int             `json:"total_suggestions_count"`
	TotalAcceptancesCount    int             `json:"total_acceptances_count"`
	TotalLinesSuggested      int             `json:"total_lines_suggested"`
	TotalLinesAccepted       int             `json:"total_lines_accepted"`
	TotalActiveUsers         int             `json:"total_active_users"`
	TotalChatAcceptances     int             `json:"total_chat_acceptances"`
	TotalChatTurns           int             `json:"total_chat_turns"`
	TotalActiveChatUsers     int             `json:"total_active_chat_users"`
	Breakdown                []db.BreakdownItem `json:"breakdown"`
}

// FetchCopilotUsage fetches the last `days` days of Copilot usage from GitHub.
func FetchCopilotUsage(ctx context.Context, orgLogin, accessToken string, days int) ([]DailyUsage, error) {
	url := fmt.Sprintf("https://api.github.com/orgs/%s/copilot/usage?per_page=%d", orgLogin, days)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github api request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// continue
	case http.StatusForbidden:
		return nil, fmt.Errorf("insufficient permissions: ensure token has copilot_usage_data:read scope")
	case http.StatusNotFound:
		return nil, fmt.Errorf("organization not found or Copilot not enabled")
	default:
		return nil, fmt.Errorf("github api error: %s", resp.Status)
	}

	var usage []DailyUsage
	if err := json.NewDecoder(resp.Body).Decode(&usage); err != nil {
		return nil, fmt.Errorf("decode usage: %w", err)
	}
	return usage, nil
}

// SyncUsage fetches and upserts usage snapshots for the given org.
func SyncUsage(ctx context.Context, queries interface {
	UpsertSnapshot(ctx context.Context, orgID string, date time.Time, totalSuggestions, totalAcceptances, totalLinesAccepted, totalActiveUsers, totalLicensedUsers int, breakdown []db.BreakdownItem) error
}, org interface {
	GetID() string
	GetLogin() string
	GetToken() string
}) (int, error) {
	usage, err := FetchCopilotUsage(ctx, org.GetLogin(), org.GetToken(), 28)
	if err != nil {
		return 0, err
	}

	synced := 0
	for _, day := range usage {
		date, err := time.Parse("2006-01-02", day.Day)
		if err != nil {
			continue
		}
		if err := queries.UpsertSnapshot(ctx, org.GetID(), date,
			day.TotalSuggestionsCount,
			day.TotalAcceptancesCount,
			day.TotalLinesAccepted,
			day.TotalActiveUsers,
			day.TotalActiveUsers, // licensed users not in v1 endpoint, use active as proxy
			day.Breakdown,
		); err != nil {
			return synced, fmt.Errorf("upsert snapshot %s: %w", day.Day, err)
		}
		synced++
	}
	return synced, nil
}
