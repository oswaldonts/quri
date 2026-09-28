package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// sendRaw is like send but without attaching a session cookie, for testing the
// auth boundary itself.
func sendRaw(h http.Handler, method, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHealthAndLoginArePublic(t *testing.T) {
	h := newTestRouter(t)

	if rec := sendRaw(h, http.MethodGet, "/health", ""); rec.Code != http.StatusOK {
		t.Errorf("GET /health without a session = %d, want 200", rec.Code)
	}

	// A wrong password must fail in the login handler itself ("wrong password"),
	// not get short-circuited by the auth middleware ("log in first") first.
	rec := sendRaw(h, http.MethodPost, "/login", `{"password":"wrong"}`)
	p := assertProblem(t, rec, http.StatusUnauthorized)
	if p.Detail != "wrong password" {
		t.Errorf("detail = %q, want %q (middleware may have intercepted /login)", p.Detail, "wrong password")
	}
}

func TestProtectedRouteWithoutSessionIsRejected(t *testing.T) {
	h := newTestRouter(t)

	rec := sendRaw(h, http.MethodGet, "/periods/2026-08/incomes", "")
	assertProblem(t, rec, http.StatusUnauthorized)
}

func TestProtectedRouteWithGarbageCookieIsRejected(t *testing.T) {
	h := newTestRouter(t)

	rec := sendRaw(h, http.MethodGet, "/periods/2026-08/incomes", "",
		&http.Cookie{Name: sessionCookie, Value: "not-a-real-token"})
	assertProblem(t, rec, http.StatusUnauthorized)
}

func TestLoginWrongPasswordIsRejected(t *testing.T) {
	h := newTestRouter(t)

	rec := sendRaw(h, http.MethodPost, "/login", `{"password":"nope"}`)
	assertProblem(t, rec, http.StatusUnauthorized)
	if rec.Header().Get("Set-Cookie") != "" {
		t.Error("a failed login must not set a session cookie")
	}
}

func TestLoginThenAccessProtectedRoute(t *testing.T) {
	h := newTestRouter(t)

	loginRec := sendRaw(h, http.MethodPost, "/login", `{"password":"`+testPassword+`"}`)
	if loginRec.Code != http.StatusNoContent {
		t.Fatalf("POST /login = %d, want 204 (body: %s)", loginRec.Code, loginRec.Body)
	}
	cookies := loginRec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != sessionCookie {
		t.Fatalf("login cookies = %+v, want exactly one %q cookie", cookies, sessionCookie)
	}
	if !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Errorf("session cookie attributes = %+v, want HttpOnly, Secure, SameSite=Strict", cookies[0])
	}

	rec := sendRaw(h, http.MethodGet, "/periods/2026-08/incomes", "", cookies[0])
	if rec.Code != http.StatusOK {
		t.Errorf("GET with the issued session = %d, want 200 (body: %s)", rec.Code, rec.Body)
	}
}

func TestLoginIsRateLimited(t *testing.T) {
	h := newTestRouter(t)

	var last *httptest.ResponseRecorder
	for i := 0; i < loginBurst+1; i++ {
		last = sendRaw(h, http.MethodPost, "/login", `{"password":"wrong"}`)
	}
	assertProblem(t, last, http.StatusTooManyRequests)
}

func TestLogoutClearsTheCookieAndWorksWithoutASession(t *testing.T) {
	h := newTestRouter(t)

	rec := sendRaw(h, http.MethodPost, "/logout", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("POST /logout without a session = %d, want 204", rec.Code)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].MaxAge >= 0 {
		t.Errorf("logout cookies = %+v, want one cookie with a negative MaxAge", cookies)
	}
}
