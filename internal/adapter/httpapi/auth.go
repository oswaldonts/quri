package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"golang.org/x/time/rate"
)

// sessionCookie is the name of the cookie holding the session token.
// It is HttpOnly (JavaScript can never read it, so an XSS can't steal it) and
// Secure (browsers treat localhost/127.0.0.1 as a secure origin, so this also
// works in development; it needs real TLS anywhere else). The desktop build, which serves
// no network port, turns Secure off with Services.InsecureCookies.
const sessionCookie = "session"

// AuthService checks the operator's password and issues/verifies session tokens.
type AuthService interface {
	CheckPassword(password string) error
	IssueToken() (token string, expiresAt time.Time)
	VerifyToken(token string) bool
	// Configured reports whether a password has been set. false only in the
	// AUTH_LOCAL_CREDENTIALS setup flow, before the first POST /setup.
	Configured() bool
	// Configure sets the operator password; see auth.Service.Configure.
	Configure(password string) error
}

// LoginInput is the request of POST /login.
type LoginInput struct {
	Body struct {
		Password string `json:"password"`
	}
}

// SetupInput is the request of POST /setup. The 8-character minimum isn't a
// real password policy, just enough to keep "1" from being called done.
type SetupInput struct {
	Body struct {
		Password string `json:"password" minLength:"8"`
	}
}

// LoginOutput sets the session cookie; the body carries nothing worth reading.
type LoginOutput struct {
	SetCookie string `header:"Set-Cookie"`
}

// loginRateLimit and loginBurst throttle POST /login regardless of outcome, so
// guessing the operator password by brute force takes years even scripted.
// There's a single operator, so one shared bucket (not per-IP) is enough and
// avoids keeping a map of limiters alive for the life of the process.
const (
	loginRateLimit = 1 * time.Minute / 30 // ~30/min sustained
	loginBurst     = 5
)

func registerAuth(api huma.API, svc AuthService, secure bool) {
	limiter := rate.NewLimiter(rate.Every(loginRateLimit), loginBurst)

	huma.Register(api, huma.Operation{
		OperationID: "setup",
		Method:      http.MethodPost,
		Path:        "/setup",
		Summary:     "Set the operator password",
		Description: "Only for the AUTH_LOCAL_CREDENTIALS mode (see cmd/hashpw / cmd/api docs): works " +
			"exactly once, the first time this instance runs with no password configured yet. Logs the " +
			"operator in immediately, like POST /login would. Returns 409 once a password already exists.",
		Tags:          []string{tagAuth},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusConflict, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *SetupInput) (*LoginOutput, error) {
		if err := svc.Configure(in.Body.Password); err != nil {
			return nil, toHTTPError(ctx, "setup", err)
		}
		token, expiresAt := svc.IssueToken()
		return &LoginOutput{SetCookie: newSessionCookie(token, expiresAt, secure).String()}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "login",
		Method:      http.MethodPost,
		Path:        "/login",
		Summary:     "Log in",
		Description: "Exchanges the operator password for a session cookie. There is a single " +
			"operator: no username, no accounts. Rate limited to slow down password guessing.",
		Tags:          []string{tagAuth},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnauthorized, http.StatusTooManyRequests, http.StatusPreconditionRequired},
	}, func(ctx context.Context, in *LoginInput) (*LoginOutput, error) {
		if !limiter.Allow() {
			return nil, huma.Error429TooManyRequests("too many login attempts, slow down")
		}
		if err := svc.CheckPassword(in.Body.Password); err != nil {
			return nil, toHTTPError(ctx, "login", err)
		}
		token, expiresAt := svc.IssueToken()
		return &LoginOutput{SetCookie: newSessionCookie(token, expiresAt, secure).String()}, nil
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
		return &LoginOutput{SetCookie: clearedSessionCookie(secure).String()}, nil
	})
}

func newSessionCookie(token string, expiresAt time.Time, secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	}
}

func clearedSessionCookie(secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	}
}

// publicOperations don't require a session: logging in and setting up
// obviously can't require one already, health is polled by the orchestrator,
// and logout must clear the cookie even if the session already expired.
var publicOperations = map[string]bool{
	"setup":  true,
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
		if !svc.Configured() {
			huma.WriteErr(api, ctx, http.StatusPreconditionRequired, "not set up yet: POST /setup first")
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
