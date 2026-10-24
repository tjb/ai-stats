package handlers_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/tjb/ai-stats/internal/auth"
	"github.com/tjb/ai-stats/internal/dbtest"
)

func TestRequireAuth_RejectsMissingAndInvalidTokens(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)
	user := dbtest.CreateUser(t, s.queries, "auth@example.com")

	signed := func(t *testing.T, claims auth.Claims, secret []byte) string {
		t.Helper()
		tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
		if err != nil {
			t.Fatalf("sign: %v", err)
		}
		return tok
	}
	now := time.Now()
	valid := auth.Claims{
		UserID: user.ID,
		Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	}
	expired := valid
	expired.RegisteredClaims.IssuedAt = jwt.NewNumericDate(now.Add(-2 * time.Hour))
	expired.RegisteredClaims.ExpiresAt = jwt.NewNumericDate(now.Add(-time.Hour))
	unsigned, err := jwt.NewWithClaims(jwt.SigningMethodNone, valid).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign none: %v", err)
	}

	cases := []struct {
		name  string
		token string
	}{
		{"no cookie", ""},
		{"garbage", "not-a-jwt"},
		{"wrong secret", signed(t, valid, []byte("some-other-secret"))},
		{"expired", signed(t, expired, s.jwtSecret)},
		{"alg none", unsigned},
	}
	protected := []struct{ method, path string }{
		{http.MethodGet, "/api/me"},
		{http.MethodGet, "/api/dashboard"},
		{http.MethodGet, "/api/insights"},
		{http.MethodPost, "/api/reports/generate"},
		{http.MethodGet, "/api/settings/ingest-keys"},
		{http.MethodPost, "/api/demo/activate"},
	}
	for _, tc := range cases {
		for _, route := range protected {
			w := s.request(t, route.method, route.path, tc.token, nil)
			if w.Code != http.StatusUnauthorized {
				t.Errorf("%s %s with %s: status = %d, want 401", route.method, route.path, tc.name, w.Code)
			}
		}
	}

	// Sanity check: the same claims signed with the right secret get through.
	var me struct {
		ID         string `json:"id"`
		Email      string `json:"email"`
		HourlyRate int    `json:"hourly_rate"`
	}
	decode(t, s.request(t, http.MethodGet, "/api/me", signed(t, valid, s.jwtSecret), nil), http.StatusOK, &me)
	if me.ID != user.ID || me.Email != user.Email || me.HourlyRate != 150 {
		t.Errorf("/api/me = %+v, want user %s with default hourly rate 150", me, user.ID)
	}
}

func TestRequireAuth_TokenForDeletedUser(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)
	user := dbtest.CreateUser(t, s.queries, "gone@example.com")
	token := s.login(t, user)

	if err := s.queries.DeleteUser(t.Context(), user.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	// The JWT is still cryptographically valid, so the middleware lets it
	// through; handlers must then fail closed rather than serve empty data.
	var body errorBody
	decode(t, s.request(t, http.MethodGet, "/api/me", token, nil), http.StatusNotFound, &body)
	decode(t, s.request(t, http.MethodGet, "/api/dashboard", token, nil), http.StatusNotFound, &body)
	if body.Error != "no_org" {
		t.Errorf("dashboard error = %q, want no_org", body.Error)
	}
}
