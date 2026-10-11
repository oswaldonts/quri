package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

var built = fstest.MapFS{
	"index.html":       {Data: []byte("<html>app</html>")},
	"favicon.svg":      {Data: []byte("<svg/>")},
	"assets/app-1.js":  {Data: []byte("console.log(1)")},
	"assets/app-1.css": {Data: []byte("body{}")},
	".gitkeep":         {Data: nil},
}

// apiHandler stands in for the API: it answers 418 so a test can tell who handled a request.
var apiHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusTeapot)
})

func serve(files fstest.MapFS, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	Handler(files, apiHandler).ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func TestServesFrontend(t *testing.T) {
	tests := []struct {
		name      string
		method    string
		target    string
		wantCode  int
		wantBody  string
		wantCache string
	}{
		{"root is index.html", http.MethodGet, "/", http.StatusOK, "<html>app</html>", revalidate},
		{"hashed asset is immutable", http.MethodGet, "/assets/app-1.js", http.StatusOK, "console.log(1)", immutable},
		{"root file revalidates", http.MethodGet, "/favicon.svg", http.StatusOK, "<svg/>", revalidate},
		{"HEAD works", http.MethodHead, "/", http.StatusOK, "", revalidate},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serve(built, tt.method, tt.target)

			if rec.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantCode)
			}
			if got := rec.Body.String(); got != tt.wantBody {
				t.Errorf("body = %q, want %q", got, tt.wantBody)
			}
			if got := rec.Header().Get("Cache-Control"); got != tt.wantCache {
				t.Errorf("Cache-Control = %q, want %q", got, tt.wantCache)
			}
			if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
			}
		})
	}
}

func TestIndexHasSecurityHeaders(t *testing.T) {
	rec := serve(built, http.MethodGet, "/")

	if got := rec.Header().Get("Content-Security-Policy"); !strings.Contains(got, "default-src 'self'") {
		t.Errorf("Content-Security-Policy = %q, want default-src 'self'", got)
	}
	if got := rec.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Errorf("Referrer-Policy = %q, want no-referrer", got)
	}
}

func TestContentTypes(t *testing.T) {
	for target, want := range map[string]string{
		"/":                 "text/html",
		"/assets/app-1.js":  "text/javascript",
		"/assets/app-1.css": "text/css",
		"/favicon.svg":      "image/svg+xml",
	} {
		rec := serve(built, http.MethodGet, target)
		if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, want) {
			t.Errorf("%s: Content-Type = %q, want %q", target, got, want)
		}
	}
}

func TestDelegatesToAPI(t *testing.T) {
	tests := []struct {
		name   string
		files  fstest.MapFS
		method string
		target string
	}{
		{"API route", built, http.MethodGet, "/periods/2026-09/summary"},
		{"API health", built, http.MethodGet, "/health"},
		{"unknown path", built, http.MethodGet, "/missing.js"},
		{"directory", built, http.MethodGet, "/assets"},
		{"hidden file", built, http.MethodGet, "/.gitkeep"},
		{"dot segments", built, http.MethodGet, "/assets/../index.html"},
		{"escaping the root", built, http.MethodGet, "/../index.html"},
		{"double slash", built, http.MethodGet, "//index.html"},
		{"trailing slash on a file", built, http.MethodGet, "/favicon.svg/"},
		{"POST to a file path", built, http.MethodPost, "/"},
		{"DELETE to a file path", built, http.MethodDelete, "/favicon.svg"},
		{"UI not built", fstest.MapFS{".gitkeep": {}}, http.MethodGet, "/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serve(tt.files, tt.method, tt.target)

			if rec.Code != http.StatusTeapot {
				t.Errorf("status = %d, want %d (the API should have answered)", rec.Code, http.StatusTeapot)
			}
		})
	}
}
