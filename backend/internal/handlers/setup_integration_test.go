package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tjb/ai-stats/internal/auth"
	"github.com/tjb/ai-stats/internal/db"
	"github.com/tjb/ai-stats/internal/dbtest"
	"github.com/tjb/ai-stats/internal/handlers"
	apimiddleware "github.com/tjb/ai-stats/internal/middleware"
)

// sessionCookie is the cookie RequireAuth reads the JWT from.
const sessionCookie = "aistats_session"

// testServer wires the real handlers to an isolated, migrated database.
type testServer struct {
	pool      *pgxpool.Pool
	queries   *db.Queries
	jwtSecret []byte
	router    http.Handler
}

// newTestServer skips the test when TEST_DATABASE_URL is unset.
func newTestServer(t *testing.T) *testServer {
	t.Helper()
	pool, queries := dbtest.New(t)
	jwtSecret := []byte("integration-test-jwt-secret")
	githubOAuth := auth.NewGitHubOAuth("test-client-id", "test-client-secret",
		"http://localhost:8080/api/auth/callback")
	authSvc := auth.NewService(queries, jwtSecret, githubOAuth)
	h := handlers.New(queries, authSvc, jwtSecret)
	return &testServer{
		pool:      pool,
		queries:   queries,
		jwtSecret: jwtSecret,
		router:    newRouter(h, jwtSecret),
	}
}

// newRouter mirrors the routes in cmd/server/main.go that these tests cover.
// Routes that call out to GitHub (OAuth, org listing, sync) are left out.
func newRouter(h *handlers.Handlers, jwtSecret []byte) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(apimiddleware.SecurityHeaders)

	// Public
	r.Post("/api/auth/logout", h.Auth.Logout)
	r.Get("/api/reports/share/{token}", h.Reports.GetByShareToken)

	// Protected
	r.Group(func(r chi.Router) {
		r.Use(apimiddleware.RequireAuth(jwtSecret))
		r.Get("/api/me", h.Auth.Me)
		r.Get("/api/dashboard", h.Dashboard.Get)
		r.Get("/api/insights", h.Insights.Get)
		r.Get("/api/reports", h.Reports.List)
		r.Post("/api/reports/generate", h.Reports.Generate)
		r.Get("/api/settings", h.Settings.Get)
		r.Post("/api/settings/hourly-rate", h.Settings.UpdateHourlyRate)
		r.Post("/api/demo/activate", h.Demo.Activate)
		r.Get("/api/settings/ingest-keys", h.Settings.ListIngestKeys)
		r.Post("/api/settings/ingest-keys", h.Settings.CreateIngestKey)
		r.Delete("/api/settings/ingest-keys/{id}", h.Settings.DeleteIngestKey)
		r.Get("/api/onboarding", h.Onboarding.Get)
		r.Post("/api/onboarding/complete", h.Onboarding.Complete)
	})

	// Push ingest — API key auth, no JWT
	r.Post("/api/ingest/events", h.Ingest.PostEvents)

	return r
}

// login mints the same session JWT the OAuth callback issues for user.
func (s *testServer) login(t *testing.T, user *db.User) string {
	t.Helper()
	tok, err := auth.IssueToken(user.ID, user.Email, s.jwtSecret)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return tok
}

// request sends method+path through the router. A non-empty token is sent as
// the session cookie; a non-nil body is JSON-encoded unless it is already a
// string or []byte.
func (s *testServer) request(t *testing.T, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	return s.requestWithHeaders(t, method, path, token, body, nil)
}

func (s *testServer) requestWithHeaders(t *testing.T, method, path, token string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		rdr = bytes.NewBufferString(b)
	case []byte:
		rdr = bytes.NewReader(b)
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		rdr = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, rdr)
	if rdr != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	return w
}

// decode asserts the response status and unmarshals the JSON body into v.
func decode(t *testing.T, w *httptest.ResponseRecorder, wantStatus int, v any) {
	t.Helper()
	if w.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, wantStatus, w.Body.String())
	}
	if v == nil {
		return
	}
	if err := json.Unmarshal(w.Body.Bytes(), v); err != nil {
		t.Fatalf("decode response: %v; body: %s", err, w.Body.String())
	}
}

// errorBody is the shape jsonErr writes.
type errorBody struct {
	Error string `json:"error"`
}

// copilotDay is one seeded copilot_usage_snapshots row.
type copilotDay struct {
	date                            time.Time
	suggestions, acceptances, lines int
	activeUsers                     int
	breakdown                       []db.BreakdownItem
}

func (s *testServer) seedCopilot(t *testing.T, orgID string, days ...copilotDay) {
	t.Helper()
	for _, d := range days {
		if err := s.queries.UpsertSnapshot(context.Background(), orgID, d.date,
			d.suggestions, d.acceptances, d.lines, d.activeUsers, 0, d.breakdown); err != nil {
			t.Fatalf("seed copilot %s: %v", d.date.Format("2006-01-02"), err)
		}
	}
}

// toolDay is one seeded tool_usage_snapshots row.
type toolDay struct {
	source                          string
	date                            time.Time
	suggestions, acceptances, lines int
	activeUsers, codingDevelopers   int
	planningUses, codingUses        int
}

func (s *testServer) seedTool(t *testing.T, orgID string, days ...toolDay) {
	t.Helper()
	for _, d := range days {
		if err := s.queries.UpsertToolSnapshot(context.Background(), orgID, d.source, d.date,
			d.suggestions, d.acceptances, d.lines, d.activeUsers, 0, d.codingDevelopers,
			d.planningUses, d.codingUses, nil); err != nil {
			t.Fatalf("seed %s %s: %v", d.source, d.date.Format("2006-01-02"), err)
		}
	}
}

// approx reports whether two floats agree to within rounding noise.
func approx(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}
