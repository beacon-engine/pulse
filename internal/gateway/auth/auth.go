// Package auth handles sign-in: password hashing, session tokens and cookies,
// and the middleware that requires a session and enforces roles.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/beacon-engine/pulse/internal/gateway/storage"
)

// Ports gateway/src/auth.rs — password hashing (argon2id, PHC format, so it
// verifies hashes created by the old Rust gateway), session tokens/cookies, and
// the require_auth / enforce_role middleware.

const CookieName = "pulse_session"
const sessionTTL = 7 * 24 * time.Hour

// User is the authenticated user, injected into the request context by
// requireAuth and read by handlers via UserFrom.
type User struct {
	ID       string
	Username string
	Role     string
}

func (u User) IsAdmin() bool { return u.Role == "admin" }

type ctxKey int

const userCtxKey ctxKey = iota

func UserFrom(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(userCtxKey).(User)
	return u, ok
}

func HashPassword(password string) (string, error) {
	return argon2id.CreateHash(password, argon2id.DefaultParams)
}

func VerifyPassword(password, hash string) bool {
	match, err := argon2id.ComparePasswordAndHash(password, hash)
	return err == nil && match
}

// NewToken returns a 256-bit random session token, hex-encoded.
func NewToken() string {
	var b [32]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func SessionExpiry() string {
	return time.Now().UTC().Add(sessionTTL).Format(time.RFC3339)
}

// SetSessionCookie writes the session cookie. Not marked Secure so it works over
// plain HTTP in dev; put the gateway behind TLS in production.
func SetSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL / time.Second),
	})
}

func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// requireAuth validates the session cookie, loads the user, and injects User
// into the request context. Responds 401 when not authenticated.
func RequireAuth(db storage.Repository, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(CookieName)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "not authenticated")
			return
		}
		sess, err := db.GetSession(r.Context(), c.Value)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if sess == nil {
			writeErr(w, http.StatusUnauthorized, "invalid session")
			return
		}
		if exp, err := time.Parse(time.RFC3339, sess.ExpiresAt); err == nil && exp.Before(time.Now()) {
			_ = db.DeleteSession(r.Context(), c.Value)
			writeErr(w, http.StatusUnauthorized, "session expired")
			return
		}
		u, err := db.GetUser(r.Context(), sess.UserID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if u == nil {
			writeErr(w, http.StatusUnauthorized, "user no longer exists")
			return
		}
		ctx := context.WithValue(r.Context(), userCtxKey, User{ID: u.ID, Username: u.Username, Role: u.Role})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// enforceRole runs after requireAuth: viewers may only make safe (read)
// requests; any state-changing request requires admin. Logout is always allowed.
func EnforceRole(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _ := UserFrom(r.Context())
		safe := r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions
		alwaysAllowed := r.URL.Path == "/api/auth/logout"
		if !safe && !alwaysAllowed && !u.IsAdmin() {
			writeErr(w, http.StatusForbidden, "read-only account: this action requires an admin")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	http.Error(w, msg, status)
}
