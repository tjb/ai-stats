package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tjb/ai-stats/internal/db"
)

// isSecure returns true when the app is running behind HTTPS.
// We derive this from APP_URL rather than r.TLS so that TLS-terminating
// reverse proxies (nginx, ALB, Caddy) are handled correctly.
func isSecure() bool {
	return strings.HasPrefix(os.Getenv("APP_URL"), "https://")
}

const (
	cookieName   = "aistats_session"
	stateCookie  = "oauth_state"
	cookieMaxAge = int(7 * 24 * time.Hour / time.Second)
)

// Service handles OAuth + JWT cookie auth.
type Service struct {
	queries     *db.Queries
	jwtSecret   []byte
	githubOAuth *GitHubOAuth
}

func NewService(queries *db.Queries, jwtSecret []byte, oauth *GitHubOAuth) *Service {
	return &Service{
		queries:     queries,
		jwtSecret:   jwtSecret,
		githubOAuth: oauth,
	}
}

// BeginOAuth redirects the user to GitHub.
func (s *Service) BeginOAuth(w http.ResponseWriter, r *http.Request) {
	state, err := randomState()
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookie,
		Value:    state,
		Path:     "/",
		MaxAge:   300,
		HttpOnly: true,
		Secure:   isSecure(),
		SameSite: http.SameSiteLaxMode, // Lax required: GitHub redirects back with this cookie
	})
	http.Redirect(w, r, s.githubOAuth.AuthCodeURL(state), http.StatusFound)
}

// CompleteOAuth handles the GitHub callback, upserts the user and sets the session cookie.
// Returns the user and nil error on success.
func (s *Service) CompleteOAuth(ctx context.Context, w http.ResponseWriter, r *http.Request) (*db.User, error) {
	// Validate state
	stateCook, err := r.Cookie(stateCookie)
	if err != nil || r.URL.Query().Get("state") != stateCook.Value {
		return nil, fmt.Errorf("invalid state")
	}
	clearCookie(w, stateCookie)

	code := r.URL.Query().Get("code")
	token, err := s.githubOAuth.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}

	ghUser, err := s.githubOAuth.GetUser(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("get github user: %w", err)
	}

	user, err := s.queries.UpsertUser(ctx,
		ghUser.Email,
		ghUser.Name,
		ghUser.AvatarURL,
		"github",
		fmt.Sprintf("%d", ghUser.ID),
		token.AccessToken,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}

	if err := s.issueSessionCookie(w, r, user.ID, user.Email); err != nil {
		return nil, fmt.Errorf("issue session: %w", err)
	}
	return user, nil
}

// ClearSession clears the session cookie.
func (s *Service) ClearSession(w http.ResponseWriter, r *http.Request) {
	clearCookie(w, cookieName)
}

// UserIDFromRequest extracts and validates the JWT from the session cookie.
func (s *Service) UserIDFromRequest(r *http.Request) (string, error) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return "", fmt.Errorf("no session cookie")
	}
	claims, err := VerifyToken(c.Value, s.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("invalid session: %w", err)
	}
	return claims.UserID, nil
}

func (s *Service) issueSessionCookie(w http.ResponseWriter, r *http.Request, userID, email string) error {
	tok, err := IssueToken(userID, email, s.jwtSecret)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    tok,
		Path:     "/",
		MaxAge:   cookieMaxAge,
		HttpOnly: true,
		Secure:   isSecure(),
		SameSite: http.SameSiteStrictMode, // Strict: no cross-site requests need this cookie
	})
	return nil
}

func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isSecure(),
		SameSite: http.SameSiteStrictMode,
	})
}
