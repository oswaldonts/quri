package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

// sessionCookie is the name of the cookie holding the session token.
// It is HttpOnly (JavaScript can never read it, so an XSS can't steal it) and
// Secure (browsers treat localhost/127.0.0.1 as a secure origin, so this also
// works in development; it needs real TLS anywhere else).
const sessionCookie = "session"

// AuthService checks the operator's password and issues/verifies session tokens.
type AuthService interface {
	CheckPassword(password string) error
	IssueToken() (token string, expiresAt time.Time)
	VerifyToken(token string) bool
}

// LoginInput is the request of POST /login.
type LoginInput struct {
	Body struct {
		Password string `json:"password"`
	}
}

// LoginOutput sets the session cookie; the body carries nothing worth reading.
type LoginOutput struct {
	SetCookie string `header:"Set-Cookie"`
}

func registerAuth(api huma.API, svc AuthService) {
	huma.Register(api, huma.Operation{
		OperationID: "login",
		Method:      http.MethodPost,
		Path:        "/login",
		Summary:     "Log in",
		Description: "Exchanges the operator password for a session cookie. There is a single " +
			"operator: no username, no accounts.",
		Tags:          []string{tagAuth},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnauthorized},
	}, func(ctx context.Context, in *LoginInput) (*LoginOutput, error) {
		if err := svc.CheckPassword(in.Body.Password); err != nil {
			return nil, huma.Error401Unauthorized("wrong password")
		}
		token, expiresAt := svc.IssueToken()
		return &LoginOutput{SetCookie: newSessionCookie(token, expiresAt).String()}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "logout",
		Method:        http.MethodPost,
		Path:          "/logout",
		Summary:       "Log out",
		Description:   "Clears the session cookie. Always succeeds, even without a valid session.",
		Tags:          []string{tagAuth},
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *struct{}) (*LoginOutput, error) {
		return &LoginOutput{SetCookie: clearedSessionCookie().String()}, nil
	})
}

func newSessionCookie(token string, expiresAt time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	}
}

func clearedSessionCookie() *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	}
}

// publicOperations don't require a session: logging in obviously can't require
// one already, health is polled by the orchestrator, and logout must clear the
// cookie even if the session already expired.
var publicOperations = map[string]bool{
	"login":  true,
	"logout": true,
	"health": true,
}

// requireSession rejects any request without a valid session cookie, except
// for publicOperations. It runs as Huma middleware (rather than chi/net-http
// middleware) so it can inspect the matched operation and reuse huma.WriteErr
// for a consistent RFC 9457 response.
func requireSession(api huma.API, svc AuthService) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		if publicOperations[ctx.Operation().OperationID] {
			next(ctx)
			return
		}
		token, ok := readCookie(ctx, sessionCookie)
		if !ok || !svc.VerifyToken(token) {
			huma.WriteErr(api, ctx, http.StatusUnauthorized, "log in first")
			return
		}
		next(ctx)
	}
}

// readCookie extracts a cookie from the request's raw Cookie header.
// Huma's Context has no cookie jar of its own, so this reuses net/http's parser.
func readCookie(ctx huma.Context, name string) (string, bool) {
	req := http.Request{Header: http.Header{"Cookie": {ctx.Header("Cookie")}}}
	cookie, err := req.Cookie(name)
	if err != nil {
		return "", false
	}
	return cookie.Value, true
}
