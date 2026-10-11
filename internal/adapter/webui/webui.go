// Package webui serves the single-page frontend from the same binary as the API.
//
// Serving both from one origin is what lets the session cookie (SameSite=Strict) and the
// frontend's relative fetches work with no CORS configuration.
package webui

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

const (
	// Vite names everything under assets/ with a content hash, so those files never change.
	immutable = "public, max-age=31536000, immutable"
	// index.html and the other root files keep their name across builds: always revalidate.
	revalidate = "no-cache"

	// The UI loads only its own files and talks only to its own origin.
	contentSecurityPolicy = "default-src 'self'; img-src 'self' data:; base-uri 'none'; " +
		"form-action 'self'; frame-ancestors 'none'"
)

// Handler serves the files of the frontend and hands every other request to api. A request
// is the frontend's when it is a GET or HEAD for a file that exists: "/" is index.html,
// and the API's routes never collide because no file in the build is named like one.
//
// The frontend routes with the URL hash (#/...), so the server never sees a client-side
// route and needs no fallback to index.html.
func Handler(files fs.FS, api http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name, ok := lookup(files, r)
		if !ok {
			api.ServeHTTP(w, r)
			return
		}

		header := w.Header()
		header.Set("X-Content-Type-Options", "nosniff")
		if strings.HasPrefix(name, "assets/") {
			header.Set("Cache-Control", immutable)
		} else {
			header.Set("Cache-Control", revalidate)
		}
		if name == "index.html" {
			header.Set("Content-Security-Policy", contentSecurityPolicy)
			header.Set("Referrer-Policy", "no-referrer")
		}
		http.ServeFileFS(w, r, files, name)
	})
}

// lookup maps the request to a file name inside files, or reports that it isn't one.
func lookup(files fs.FS, r *http.Request) (string, bool) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return "", false
	}
	p := r.URL.Path
	var name string
	switch {
	case p == "/":
		name = "index.html"
	case strings.HasPrefix(p, "/") && path.Clean(p) == p:
		name = p[1:]
	default:
		// Not canonical ("/a/../b", trailing slash, double slash): never a file of the build.
		return "", false
	}
	if !fs.ValidPath(name) || hidden(name) {
		return "", false
	}
	info, err := fs.Stat(files, name)
	if err != nil || info.IsDir() {
		return "", false
	}
	return name, true
}

// hidden reports whether any element of name starts with a dot (.gitkeep and the like).
func hidden(name string) bool {
	for _, element := range strings.Split(name, "/") {
		if strings.HasPrefix(element, ".") {
			return true
		}
	}
	return false
}
