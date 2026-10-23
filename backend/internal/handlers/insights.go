package handlers

import (
	"math"
	"net/http"

	"github.com/tjb/ai-stats/internal/db"
	apimiddleware "github.com/tjb/ai-stats/internal/middleware"
	"github.com/tjb/ai-stats/internal/roi"
)

type InsightsHandler struct {
	queries *db.Queries
}

type InsightsResponse struct {
	Languages             []languageStat        `json:"languages"`
	Editors               []editorStat          `json:"editors"`
	ActivityHeatmap       []heatmapPoint        `json:"activity_heatmap"`
	UtilizationPercent    float64               `json:"utilization_percent"`
	AIActiveUsers         int                   `json:"ai_active_users"`
	TotalCodingDevelopers int                   `json:"total_coding_developers"`
	UtilizationTrend      []db.UtilizationPoint `json:"utilization_trend"`
	UtilizationDeltaPP    float64               `json:"utilization_delta_pp"`
	UtilizationPrevPct    float64               `json:"utilization_prev_percent"`
	ToolBreakdown         []db.ToolBreakdownRow `json:"tool_breakdown"`
	HoursSaved            float64               `json:"hours_saved"`
	DollarValueSaved      float64               `json:"dollar_value_saved"`
	HourlyRate            int                   `json:"hourly_rate"`
	AcceptanceRate        float64               `json:"acceptance_rate"`
	LinesAccepted         int                   `json:"lines_accepted"`
	WeeklyActiveUsers     int                   `json:"weekly_active_users"`
	PrevWeekActiveUsers   int                   `json:"prev_week_active_users"`
	PlanningToolUses      int                   `json:"planning_tool_uses"`
	CodingToolUses        int                   `json:"coding_tool_uses"`
	PlanningRatio         float64               `json:"planning_ratio"`
	PrevPlanningRatio     float64               `json:"prev_planning_ratio"`
	PlanningTrend         []planningTrendPoint  `json:"planning_trend"`
}

type planningTrendPoint struct {
	Date             string  `json:"date"`
	PlanningRatioPct float64 `json:"planning_ratio_percent"`
	PlanningUses     int     `json:"planning_uses"`
	CodingUses       int     `json:"coding_uses"`
}

type languageStat struct {
	Language    string  `json:"language"`
	Suggestions int     `json:"suggestions"`
	Acceptances int     `json:"acceptances"`
	Lines       int     `json:"lines"`
	Rate        float64 `json:"rate"`
}

type editorStat struct {
	Editor      string  `json:"editor"`
	Suggestions int     `json:"suggestions"`
	Acceptances int     `json:"acceptances"`
	Users       int     `json:"users"`
	Rate        float64 `json:"rate"`
}

type heatmapPoint struct {
	Date  string `json:"date"`
	Users int    `json:"users"`
}

func (h *InsightsHandler) Get(w http.ResponseWriter, r *http.Request) {
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

	// Fetch up to 90 days of snapshots to support the planning trend and prior-period comparison.
	snapshots, err := h.queries.GetCombinedSnapshots(r.Context(), org.ID, 90)
	if err != nil {
		jsonErr(w, "db error", http.StatusInternalServerError)
		return
	}

	// Partition into current (last 30), prior (days 31–60), and full set for trend.
	current30 := snapshots
	if len(current30) > 30 {
		current30 = snapshots[len(snapshots)-30:]
	}
	var prior30 []db.UsageSnapshot
	if len(snapshots) > 30 {
		start := len(snapshots) - 60
		if start < 0 {
			start = 0
		}
		prior30 = snapshots[start : len(snapshots)-30]
	}

	langMap := map[string]*struct{ s, a, l, u int }{}
	editorMap := map[string]*struct{ s, a, u int }{}

	for _, snap := range current30 {
		for _, b := range snap.BreakdownJSON {
			lang := b.Language
			if lang == "" {
				lang = "Other"
			}
			editor := b.Editor
			if editor == "" {
				editor = "Unknown"
			}

			if langMap[lang] == nil {
				langMap[lang] = &struct{ s, a, l, u int }{}
			}
			langMap[lang].s += b.SuggestionsCount
			langMap[lang].a += b.AcceptancesCount
			langMap[lang].l += b.LinesAccepted
			langMap[lang].u += b.ActiveUsers

			if editorMap[editor] == nil {
				editorMap[editor] = &struct{ s, a, u int }{}
			}
			editorMap[editor].s += b.SuggestionsCount
			editorMap[editor].a += b.AcceptancesCount
			editorMap[editor].u += b.ActiveUsers
		}
	}

	langStats := []languageStat{}
	for lang, v := range langMap {
		langStats = append(langStats, languageStat{
			Language:    lang,
			Suggestions: v.s,
			Acceptances: v.a,
			Lines:       v.l,
			Rate:        roi.AcceptanceRate(v.a, v.s),
		})
	}
	// Sort by suggestions desc
	for i := 0; i < len(langStats); i++ {
		for j := i + 1; j < len(langStats); j++ {
			if langStats[j].Suggestions > langStats[i].Suggestions {
				langStats[i], langStats[j] = langStats[j], langStats[i]
			}
		}
	}
	if len(langStats) > 10 {
		langStats = langStats[:10]
	}

	editorStats := []editorStat{}
	for editor, v := range editorMap {
		editorStats = append(editorStats, editorStat{
			Editor:      editor,
			Suggestions: v.s,
			Acceptances: v.a,
			Users:       v.u,
			Rate:        roi.AcceptanceRate(v.a, v.s),
		})
	}
	// Sort by users desc
	for i := 0; i < len(editorStats); i++ {
		for j := i + 1; j < len(editorStats); j++ {
			if editorStats[j].Users > editorStats[i].Users {
				editorStats[i], editorStats[j] = editorStats[j], editorStats[i]
			}
		}
	}

	// Compute 30-day totals for ROI and acceptance rate from the combined snapshots.
	var totalAcceptances, totalSuggestions, totalLinesAccepted int
	for _, s := range current30 {
		totalAcceptances += s.TotalAcceptances
		totalSuggestions += s.TotalSuggestions
		totalLinesAccepted += s.TotalLinesAccepted
	}

	heatmap := []heatmapPoint{}
	for _, s := range current30 {
		heatmap = append(heatmap, heatmapPoint{
			Date:  s.Date.Format("2006-01-02"),
			Users: s.TotalActiveUsers,
		})
	}

	roiResult := roi.Calculate(totalLinesAccepted, user.HourlyRate)
	acceptanceRate := roi.AcceptanceRate(totalAcceptances, totalSuggestions)

	weeklyActive, prevWeekActive, err := h.queries.GetWeeklyActiveUsers(r.Context(), org.ID)
	if err != nil {
		weeklyActive, prevWeekActive = 0, 0
	}

	aiUsers, totalDevs, err := h.queries.GetUtilizationStats(r.Context(), org.ID, 30)
	if err != nil {
		aiUsers, totalDevs = 0, 0
	}
	var utilizationPct float64
	if totalDevs > 0 {
		utilizationPct = float64(aiUsers) / float64(totalDevs) * 100
		if utilizationPct > 100 {
			utilizationPct = 100
		}
	}

	trend, err := h.queries.GetUtilizationTrend(r.Context(), org.ID, 30)
	if err != nil {
		trend = []db.UtilizationPoint{}
	}

	prevAI, prevDevs, err := h.queries.GetUtilizationStatsPriorWindow(r.Context(), org.ID)
	if err != nil {
		prevAI, prevDevs = 0, 0
	}
	var prevPct float64
	if prevDevs > 0 {
		prevPct = math.Round(float64(prevAI)/float64(prevDevs)*100*10) / 10
		if prevPct > 100 {
			prevPct = 100
		}
	}
	// No prior-window denominator means nothing to compare against.
	var deltaPP float64
	if prevDevs > 0 {
		deltaPP = math.Round((utilizationPct-prevPct)*10) / 10
	}

	toolBreakdown, err := h.queries.GetToolBreakdown(r.Context(), org.ID, 30)
	if err != nil {
		toolBreakdown = []db.ToolBreakdownRow{}
	}

	// Planning ratio — current 30-day window.
	var planningUses, codingUses int
	for _, s := range current30 {
		planningUses += s.TotalPlanningUses
		codingUses += s.TotalCodingUses
	}
	var planningRatio float64
	if planningUses+codingUses > 0 {
		planningRatio = float64(planningUses) / float64(planningUses+codingUses) * 100
	}

	// Planning ratio — prior 30-day window (days 31–60).
	var prevPlanningUses, prevCodingUses int
	for _, s := range prior30 {
		prevPlanningUses += s.TotalPlanningUses
		prevCodingUses += s.TotalCodingUses
	}
	var prevPlanningRatio float64
	if prevPlanningUses+prevCodingUses > 0 {
		prevPlanningRatio = float64(prevPlanningUses) / float64(prevPlanningUses+prevCodingUses) * 100
	}

	// Planning trend — all fetched snapshots (up to 90 days), one point per day.
	planningTrend := []planningTrendPoint{}
	for _, s := range snapshots {
		total := s.TotalPlanningUses + s.TotalCodingUses
		var ratioPct float64
		if total > 0 {
			ratioPct = float64(s.TotalPlanningUses) / float64(total) * 100
		}
		planningTrend = append(planningTrend, planningTrendPoint{
			Date:             s.Date.Format("2006-01-02"),
			PlanningRatioPct: ratioPct,
			PlanningUses:     s.TotalPlanningUses,
			CodingUses:       s.TotalCodingUses,
		})
	}

	jsonOK(w, InsightsResponse{
		Languages:             langStats,
		Editors:               editorStats,
		ActivityHeatmap:       heatmap,
		UtilizationPercent:    math.Round(utilizationPct*10) / 10,
		AIActiveUsers:         aiUsers,
		TotalCodingDevelopers: totalDevs,
		UtilizationTrend:      trend,
		UtilizationDeltaPP:    deltaPP,
		UtilizationPrevPct:    prevPct,
		ToolBreakdown:         toolBreakdown,
		HoursSaved:            roiResult.HoursSaved,
		DollarValueSaved:      roiResult.DollarValueSaved,
		HourlyRate:            user.HourlyRate,
		AcceptanceRate:        acceptanceRate,
		LinesAccepted:         totalLinesAccepted,
		WeeklyActiveUsers:     weeklyActive,
		PrevWeekActiveUsers:   prevWeekActive,
		PlanningToolUses:      planningUses,
		CodingToolUses:        codingUses,
		PlanningRatio:         planningRatio,
		PrevPlanningRatio:     prevPlanningRatio,
		PlanningTrend:         planningTrend,
	})
}
