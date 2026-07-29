package handlers

import (
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/tjb/ai-stats/internal/db"
	apimiddleware "github.com/tjb/ai-stats/internal/middleware"
)

type DemoHandler struct {
	queries *db.Queries
}

func (h *DemoHandler) Activate(w http.ResponseWriter, r *http.Request) {
	userID := apimiddleware.UserIDFromCtx(r.Context())

	// Return existing org if user already has one
	existing, err := h.queries.GetOrgByOwner(r.Context(), userID)
	if err == nil {
		jsonOK(w, map[string]string{"org_id": existing.ID})
		return
	}

	// Create a per-user demo org so multiple users can each have their own
	// demo dataset and cannot hijack each other's org ownership.
	demoLogin := "demo-" + userID
	demoOrg, err := h.queries.GetOrgByLogin(r.Context(), demoLogin)
	if err != nil {
		avatarURL := "https://avatars.githubusercontent.com/u/1?v=4"
		demoOrg, err = h.queries.CreateOrg(r.Context(),
			"demo-org-"+userID,
			demoLogin,
			"Acme Corp (Demo)",
			&avatarURL,
			"demo-token",
			userID,
		)
		if err != nil {
			jsonErr(w, "failed to create demo org", http.StatusInternalServerError)
			return
		}

		// Seed 90 days of data
		if err := h.seedDemoData(r, demoOrg.ID); err != nil {
			jsonErr(w, "failed to seed demo data", http.StatusInternalServerError)
			return
		}
	}

	jsonOK(w, map[string]string{"org_id": demoOrg.ID})
}

func (h *DemoHandler) seedDemoData(r *http.Request, orgID string) error {
	today := time.Now().Truncate(24 * time.Hour)
	for i := 89; i >= 0; i-- {
		date := today.AddDate(0, 0, -i)
		baseUsers := 40 + rand.IntN(15)
		suggestions := baseUsers*45 + rand.IntN(500)
		acceptances := int(float64(suggestions) * (0.32 + rand.Float64()*0.15))

		breakdown := []db.BreakdownItem{
			{
				Language: "TypeScript", Editor: "vscode",
				SuggestionsCount: int(float64(suggestions) * 0.45),
				AcceptancesCount: int(float64(acceptances) * 0.45),
				LinesAccepted:    int(float64(acceptances) * 0.45),
				ActiveUsers:      int(float64(baseUsers) * 0.7),
			},
			{
				Language: "Python", Editor: "vscode",
				SuggestionsCount: int(float64(suggestions) * 0.25),
				AcceptancesCount: int(float64(acceptances) * 0.28),
				LinesAccepted:    int(float64(acceptances) * 0.28),
				ActiveUsers:      int(float64(baseUsers) * 0.35),
			},
			{
				Language: "Go", Editor: "vscode",
				SuggestionsCount: int(float64(suggestions) * 0.15),
				AcceptancesCount: int(float64(acceptances) * 0.14),
				LinesAccepted:    int(float64(acceptances) * 0.14),
				ActiveUsers:      int(float64(baseUsers) * 0.2),
			},
			{
				Language: "Rust", Editor: "jetbrains",
				SuggestionsCount: int(float64(suggestions) * 0.1),
				AcceptancesCount: int(float64(acceptances) * 0.08),
				LinesAccepted:    int(float64(acceptances) * 0.08),
				ActiveUsers:      int(float64(baseUsers) * 0.1),
			},
		}

		if err := h.queries.UpsertSnapshot(r.Context(), orgID, date,
			suggestions, acceptances, int(float64(acceptances)*2.8),
			baseUsers, 52, breakdown,
		); err != nil {
			return err
		}
	}
	return nil
}
