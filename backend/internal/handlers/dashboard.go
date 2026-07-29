package handlers

import (
	"net/http"

	"github.com/tjb/ai-stats/internal/db"
	apimiddleware "github.com/tjb/ai-stats/internal/middleware"
	"github.com/tjb/ai-stats/internal/roi"
)

type DashboardHandler struct {
	queries *db.Queries
}

type DashboardResponse struct {
	Org           orgSummary       `json:"org"`
	Metrics       dashboardMetrics `json:"metrics"`
	Charts        dashboardCharts  `json:"charts"`
}

type orgSummary struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	GitHubOrgLogin    string  `json:"github_org_login"`
	AvatarURL         *string `json:"avatar_url"`
	GithubMemberCount *int    `json:"github_member_count"`
}

type dashboardMetrics struct {
	ActiveUsers         int     `json:"active_users"`
	ActiveUsersChange   int     `json:"active_users_change"`
	AcceptanceRate      float64 `json:"acceptance_rate"`
	AcceptanceRateChange float64 `json:"acceptance_rate_change"`
	LinesAccepted       int     `json:"lines_accepted"`
	LinesAcceptedChange int     `json:"lines_accepted_change"`
	HoursSaved          float64 `json:"hours_saved"`
	DollarValueSaved    float64 `json:"dollar_value_saved"`
	HourlyRate          int     `json:"hourly_rate"`
}

type dashboardCharts struct {
	AcceptanceRate []chartPoint         `json:"acceptance_rate"`
	ActiveUsers    []activeUserPoint    `json:"active_users"`
	Suggestions    []suggestionPoint    `json:"suggestions"`
	Languages      []languageRatePoint  `json:"languages"`
}

type chartPoint struct {
	Date string  `json:"date"`
	Rate float64 `json:"rate"`
}

type activeUserPoint struct {
	Date  string `json:"date"`
	Users int    `json:"users"`
}

type suggestionPoint struct {
	Date        string `json:"date"`
	Suggestions int    `json:"suggestions"`
	Acceptances int    `json:"acceptances"`
}

type languageRatePoint struct {
	Language string  `json:"language"`
	Rate     float64 `json:"rate"`
}

func (h *DashboardHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := apimiddleware.UserIDFromCtx(r.Context())

	org, err := h.queries.GetOrgByOwner(r.Context(), userID)
	if err != nil {
		jsonErr(w, "no_org", http.StatusNotFound)
		return
	}

	user, err := h.queries.GetUserByID(r.Context(), userID)
	if err != nil {
		jsonErr(w, "user not found", http.StatusInternalServerError)
		return
	}

	snapshots, err := h.queries.GetCombinedSnapshots(r.Context(), org.ID, 90)
	if err != nil {
		jsonErr(w, "db error", http.StatusInternalServerError)
		return
	}

	if len(snapshots) == 0 {
		jsonOK(w, DashboardResponse{
			Org: orgSummary{
				ID: org.ID, Name: org.Name,
				GitHubOrgLogin:    org.GitHubOrgLogin,
				AvatarURL:         org.AvatarURL,
				GithubMemberCount: org.GithubMemberCount,
			},
			Metrics: dashboardMetrics{HourlyRate: user.HourlyRate},
			Charts: dashboardCharts{
				AcceptanceRate: []chartPoint{},
				ActiveUsers:    []activeUserPoint{},
				Suggestions:    []suggestionPoint{},
				Languages:      []languageRatePoint{},
			},
		})
		return
	}

	last30 := snapshots
	if len(last30) > 30 {
		last30 = last30[len(last30)-30:]
	}
	prev30 := []db.UsageSnapshot{}
	if len(snapshots) > 30 {
		start := len(snapshots) - 60
		if start < 0 {
			start = 0
		}
		prev30 = snapshots[start : len(snapshots)-30]
	}

	type totals struct {
		suggestions   int
		acceptances   int
		linesAccepted int
		maxUsers      int
	}
	sum := func(ss []db.UsageSnapshot) totals {
		var t totals
		for _, s := range ss {
			t.suggestions += s.TotalSuggestions
			t.acceptances += s.TotalAcceptances
			t.linesAccepted += s.TotalLinesAccepted
			if s.TotalActiveUsers > t.maxUsers {
				t.maxUsers = s.TotalActiveUsers
			}
		}
		return t
	}

	t30 := sum(last30)
	tp := sum(prev30)

	ar := roi.AcceptanceRate(t30.acceptances, t30.suggestions)
	prevAR := roi.AcceptanceRate(tp.acceptances, tp.suggestions)
	roiResult := roi.Calculate(t30.linesAccepted, user.HourlyRate)

	// Build chart data from all snapshots (up to 90)
	arChart := []chartPoint{}
	usersChart := []activeUserPoint{}
	sugChart := []suggestionPoint{}
	for _, s := range snapshots {
		d := s.Date.Format("2006-01-02")
		arChart = append(arChart, chartPoint{Date: d, Rate: roi.AcceptanceRate(s.TotalAcceptances, s.TotalSuggestions)})
		usersChart = append(usersChart, activeUserPoint{Date: d, Users: s.TotalActiveUsers})
		sugChart = append(sugChart, suggestionPoint{Date: d, Suggestions: s.TotalSuggestions, Acceptances: s.TotalAcceptances})
	}

	// Language breakdown from last snapshot
	langChart := []languageRatePoint{}
	if len(snapshots) > 0 {
		last := snapshots[len(snapshots)-1]
		langMap := map[string]struct{ s, a int }{}
		for _, b := range last.BreakdownJSON {
			k := b.Language
			if k == "" {
				k = "Other"
			}
			v := langMap[k]
			v.s += b.SuggestionsCount
			v.a += b.AcceptancesCount
			langMap[k] = v
		}
		for lang, v := range langMap {
			langChart = append(langChart, languageRatePoint{
				Language: lang,
				Rate:     roi.AcceptanceRate(v.a, v.s),
			})
		}
		// Sort descending by rate
		for i := 0; i < len(langChart); i++ {
			for j := i + 1; j < len(langChart); j++ {
				if langChart[j].Rate > langChart[i].Rate {
					langChart[i], langChart[j] = langChart[j], langChart[i]
				}
			}
		}
		if len(langChart) > 6 {
			langChart = langChart[:6]
		}
	}

	jsonOK(w, DashboardResponse{
		Org: orgSummary{
			ID:                org.ID,
			Name:              org.Name,
			GitHubOrgLogin:    org.GitHubOrgLogin,
			AvatarURL:         org.AvatarURL,
			GithubMemberCount: org.GithubMemberCount,
		},
		Metrics: dashboardMetrics{
			ActiveUsers:          t30.maxUsers,
			ActiveUsersChange:    roi.PctChange(t30.maxUsers, tp.maxUsers),
			AcceptanceRate:       ar,
			AcceptanceRateChange: ar - prevAR,
			LinesAccepted:        t30.linesAccepted,
			LinesAcceptedChange:  roi.PctChange(t30.linesAccepted, tp.linesAccepted),
			HoursSaved:           roiResult.HoursSaved,
			DollarValueSaved:     roiResult.DollarValueSaved,
			HourlyRate:           user.HourlyRate,
		},
		Charts: dashboardCharts{
			AcceptanceRate: arChart,
			ActiveUsers:    usersChart,
			Suggestions:    sugChart,
			Languages:      langChart,
		},
	})
}
