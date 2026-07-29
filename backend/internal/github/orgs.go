package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
)

// OrgInfo holds minimal GitHub org data needed to create an org record.
type OrgInfo struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

// ListUserOrgs returns the GitHub organizations the authenticated user belongs to.
func ListUserOrgs(ctx context.Context, accessToken string) ([]OrgInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.github.com/user/orgs?per_page=100", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api error: %s", resp.Status)
	}

	var orgs []OrgInfo
	if err := json.NewDecoder(resp.Body).Decode(&orgs); err != nil {
		return nil, fmt.Errorf("decode orgs: %w", err)
	}
	return orgs, nil
}

// reLinkLast matches the page number from the GitHub Link header's "last" rel.
// Example: <https://api.github.com/orgs/foo/members?per_page=1&page=42>; rel="last"
var reLinkLast = regexp.MustCompile(`[?&]page=(\d+)[^>]*>;\s*rel="last"`)

// GetOrgMemberCount returns the number of members in a GitHub organization.
// It calls GET /orgs/{orgLogin}/members?per_page=1 and reads the Link header to
// determine the total count. If there is no Link header the count is the number
// of items in the response body (0 or 1).
func GetOrgMemberCount(ctx context.Context, orgLogin, accessToken string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.github.com/orgs/"+orgLogin+"/members?per_page=1", nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("github api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("github api error: %s", resp.Status)
	}

	// If a Link header is present, the last page number equals the member count.
	if link := resp.Header.Get("Link"); link != "" {
		if m := reLinkLast.FindStringSubmatch(link); len(m) == 2 {
			n, err := strconv.Atoi(m[1])
			if err == nil {
				return n, nil
			}
		}
	}

	// No Link header — org has 0 or 1 members; count items in the body.
	var members []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&members); err != nil {
		return 0, fmt.Errorf("decode members: %w", err)
	}
	return len(members), nil
}

// GetOrg fetches a single org by login, populating the Name field (which /user/orgs omits).
func GetOrg(ctx context.Context, orgLogin, accessToken string) (*OrgInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.github.com/orgs/"+orgLogin, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("organization not found: %s", orgLogin)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api error: %s", resp.Status)
	}

	var org OrgInfo
	if err := json.NewDecoder(resp.Body).Decode(&org); err != nil {
		return nil, fmt.Errorf("decode org: %w", err)
	}
	return &org, nil
}
