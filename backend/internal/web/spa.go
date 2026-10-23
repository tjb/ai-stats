// Package web serves the built frontend single-page app (frontend/dist) from
// the Go server, so a deployment is a single origin: the SPA, /api and the
// OAuth callback all share APP_URL and no CORS or proxy setup is needed.
package web

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
)

const (
	indexFile = "index.html"

	// Vite emits content-hashed bundles under assets/, so a given URL never
	// changes and browsers may keep it for a year without revalidating.
	immutableCache = "public, max-age=31536000, immutable"
	// index.html and unhashed public/ files must be revalidated on every load
	// so a new deploy is picked up immediately (Last-Modified makes it a 304).
	revalidateCache = "no-cache"
)

// Handler serves a built SPA. Existing files are served as-is; any other
// GET/HEAD path gets index.html so client-side routes survive a reload.
// Mount it as the router's NotFound handler so API routes always win.
type Handler struct {
	fsys fs.FS
}

// New returns a Handler serving files from dir. File access is confined to dir
// by os.Root, which rejects "..", absolute paths and symlinks escaping it.
func New(dir string) (*Handler, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("open static dir: %w", err)
	}
	fsys := root.FS()
	if fi, err := fs.Stat(fsys, indexFile); err != nil || !fi.Mode().IsRegular() {
		root.Close()
		return nil, fmt.Errorf("%s has no %s (is the frontend built?)", dir, indexFile)
	}
	return &Handler{fsys: fsys}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Cleaning a rooted path drops every ".." that would climb above "/".
	urlPath := path.Clean("/" + r.URL.Path)
	if isBackendPath(urlPath) {
		// Unmatched API routes keep their plain 404 — never answer them with HTML.
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	name := strings.TrimPrefix(urlPath, "/")
	if name == "" {
		name = indexFile
	}
	if h.serveFile(w, r, name) {
		return
	}
	// A missing file (under assets/ or with an extension) is a real 404:
	// answering a stale bundle URL with HTML breaks the page in confusing ways.
	if isAsset(name) || path.Ext(name) != "" {
		http.NotFound(w, r)
		return
	}
	if !h.serveFile(w, r, indexFile) {
		http.NotFound(w, r)
	}
}

// serveFile writes the regular file name (slash-separated, relative to the
// static dir) and reports whether it existed.
func (h *Handler) serveFile(w http.ResponseWriter, r *http.Request, name string) bool {
	f, err := h.fsys.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		return false
	}
	content, ok := f.(io.ReadSeeker)
	if !ok {
		return false
	}
	if isAsset(name) {
		w.Header().Set("Cache-Control", immutableCache)
	} else {
		w.Header().Set("Cache-Control", revalidateCache)
	}
	// ServeContent sets Content-Type from the extension and handles HEAD,
	// Range and If-Modified-Since.
	http.ServeContent(w, r, name, fi.ModTime(), content)
	return true
}

// isAsset reports whether name is in Vite's hashed output directory.
func isAsset(name string) bool {
	return name == "assets" || strings.HasPrefix(name, "assets/")
}

// isBackendPath reports whether p (already cleaned) belongs to the Go server
// rather than the SPA.
func isBackendPath(p string) bool {
	for _, prefix := range []string{"/api", "/health"} {
		if p == prefix || strings.HasPrefix(p, prefix+"/") {
			return true
		}
	}
	return false
}
