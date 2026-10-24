package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

const (
	indexHTML = "<!doctype html><title>AI Stats</title>"
	bundleJS  = "console.log('app')"
	secret    = "TOP SECRET"
)

// newTestDir lays out a built SPA plus a secret file next to (not inside) it.
func newTestDir(t *testing.T) string {
	t.Helper()
	parent := t.TempDir()
	dir := filepath.Join(parent, "dist")
	files := map[string]string{
		"index.html":               indexHTML,
		"assets/index-Bx3k9a.js":   bundleJS,
		"assets/index-Q2w8zz.css":  "body{}",
		"favicon.svg":              "<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>",
		"install.sh":               "#!/usr/bin/env bash\necho hi\n",
		"../secret.txt":            secret,
		"../outside/secret.txt":    secret,
		"assets/nested/chunk-1.js": bundleJS,
	}
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Symlinks that point outside the static dir must not be followed.
	if err := os.Symlink(filepath.Join(parent, "secret.txt"), filepath.Join(dir, "leak.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(parent, "outside"), filepath.Join(dir, "leakdir")); err != nil {
		t.Fatal(err)
	}
	return dir
}

func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	h, err := New(newTestDir(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h
}

func TestHandler(t *testing.T) {
	h := newTestHandler(t)

	tests := []struct {
		name        string
		method      string
		target      string
		wantStatus  int
		wantBody    string // exact body; empty means "don't check"
		wantType    string // Content-Type prefix
		wantCache   string
		notContains string
	}{
		{name: "root serves index", target: "/", wantStatus: 200, wantBody: indexHTML, wantType: "text/html", wantCache: revalidateCache},
		{name: "index.html directly", target: "/index.html", wantStatus: 200, wantBody: indexHTML, wantCache: revalidateCache},
		{name: "client route falls back to index", target: "/dashboard", wantStatus: 200, wantBody: indexHTML, wantType: "text/html", wantCache: revalidateCache},
		{name: "nested client route falls back to index", target: "/r/0123456789abcdef", wantStatus: 200, wantBody: indexHTML},
		{name: "health-like SPA route is not the health check", target: "/healthz", wantStatus: 200, wantBody: indexHTML},
		{name: "head on client route", method: http.MethodHead, target: "/settings", wantStatus: 200, wantType: "text/html", wantCache: revalidateCache},
		{name: "hashed asset is immutable", target: "/assets/index-Bx3k9a.js", wantStatus: 200, wantBody: bundleJS, wantType: "text/javascript", wantCache: immutableCache},
		{name: "nested hashed asset is immutable", target: "/assets/nested/chunk-1.js", wantStatus: 200, wantBody: bundleJS, wantCache: immutableCache},
		{name: "css asset content type", target: "/assets/index-Q2w8zz.css", wantStatus: 200, wantType: "text/css", wantCache: immutableCache},
		{name: "public file revalidates", target: "/favicon.svg", wantStatus: 200, wantType: "image/svg+xml", wantCache: revalidateCache},
		{name: "public install script", target: "/install.sh", wantStatus: 200, wantCache: revalidateCache},
		{name: "missing asset is a 404", target: "/assets/index-old.js", wantStatus: 404, notContains: indexHTML},
		{name: "assets dir listing is a 404", target: "/assets/", wantStatus: 404, notContains: indexHTML},
		{name: "assets dir without slash is a 404", target: "/assets", wantStatus: 404, notContains: indexHTML},
		{name: "missing file with extension is a 404", target: "/robots.txt", wantStatus: 404, notContains: indexHTML},
		{name: "unknown api route is a 404", target: "/api/does-not-exist", wantStatus: 404, notContains: indexHTML},
		{name: "bare api prefix is a 404", target: "/api", wantStatus: 404, notContains: indexHTML},
		{name: "api route reached through dot segments", target: "/foo/../api/me", wantStatus: 404, notContains: indexHTML},
		{name: "unknown health subpath is a 404", target: "/health/deep", wantStatus: 404, notContains: indexHTML},
		{name: "post to client route", method: http.MethodPost, target: "/dashboard", wantStatus: 405, notContains: indexHTML},
		{name: "traversal with dot segments", target: "/../secret.txt", wantStatus: 404, notContains: secret},
		{name: "traversal from assets", target: "/assets/../../secret.txt", wantStatus: 404, notContains: secret},
		{name: "traversal with encoded slash", target: "/..%2fsecret.txt", wantStatus: 404, notContains: secret},
		{name: "symlink to file outside dir", target: "/leak.txt", wantStatus: 404, notContains: secret},
		{name: "symlink to dir outside dir", target: "/leakdir/secret.txt", wantStatus: 404, notContains: secret},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method := tt.method
			if method == "" {
				method = http.MethodGet
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(method, tt.target, nil))

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %q)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			body := rec.Body.String()
			if tt.wantBody != "" && body != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
			if method == http.MethodHead && body != "" {
				t.Errorf("HEAD body = %q, want empty", body)
			}
			if tt.notContains != "" && strings.Contains(body, tt.notContains) {
				t.Errorf("body %q must not contain %q", body, tt.notContains)
			}
			if got := rec.Header().Get("Content-Type"); tt.wantType != "" && !strings.HasPrefix(got, tt.wantType) {
				t.Errorf("Content-Type = %q, want prefix %q", got, tt.wantType)
			}
			if got := rec.Header().Get("Cache-Control"); tt.wantCache != "" && got != tt.wantCache {
				t.Errorf("Cache-Control = %q, want %q", got, tt.wantCache)
			}
			if tt.wantStatus == 405 && rec.Header().Get("Allow") != "GET, HEAD" {
				t.Errorf("Allow = %q, want %q", rec.Header().Get("Allow"), "GET, HEAD")
			}
		})
	}
}

func TestHandlerConditionalGet(t *testing.T) {
	h := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req.Header.Set("If-Modified-Since", time.Now().Add(time.Hour).UTC().Format(http.TimeFormat))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != revalidateCache {
		t.Errorf("Cache-Control = %q, want %q", got, revalidateCache)
	}
}

func TestNew(t *testing.T) {
	if _, err := New(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("New(missing dir) succeeded, want error")
	}
	if _, err := New(t.TempDir()); err == nil {
		t.Error("New(dir without index.html) succeeded, want error")
	}
	noIndex := t.TempDir()
	if err := os.Mkdir(filepath.Join(noIndex, "index.html"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := New(noIndex); err == nil {
		t.Error("New(dir whose index.html is a directory) succeeded, want error")
	}
}

// TestRouterNotFound mirrors the wiring in cmd/server: the SPA is the chi
// NotFound handler, so every registered route keeps its own response.
func TestRouterNotFound(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	r.Get("/api/me", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
	})
	r.NotFound(newTestHandler(t).ServeHTTP)

	tests := []struct {
		method, target string
		wantStatus     int
		wantBody       string
	}{
		{http.MethodGet, "/health", 200, "ok"},
		{http.MethodGet, "/api/me", 401, `{"error":"unauthorized"}` + "\n"},
		{http.MethodPost, "/api/me", 405, ""},
		{http.MethodGet, "/api/nope", 404, "404 page not found\n"},
		{http.MethodGet, "/dashboard", 200, indexHTML},
		{http.MethodGet, "/assets/index-Bx3k9a.js", 200, bundleJS},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.target, nil))
		if rec.Code != tt.wantStatus || rec.Body.String() != tt.wantBody {
			t.Errorf("%s %s = %d %q, want %d %q", tt.method, tt.target, rec.Code, rec.Body.String(), tt.wantStatus, tt.wantBody)
		}
	}
}
