package gateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/alexedwards/argon2id"
)

// Ports gateway/src/auth.rs — password hashing (argon2id, PHC format, so it
// verifies hashes created by the old Rust gateway), session tokens/cookies, and
// the require_auth / enforce_role middleware.

const sessionCookie = "pulse_session"
const sessionTTL = 7 * 24 * time.Hour

// AuthUser is the authenticated user, injected into the request context by
// requireAuth and read by handlers via userFrom.
type AuthUser struct {
	ID       string
	Username string
	Role     string
}

func (u AuthUser) IsAdmin() bool { return u.Role == "admin" }

type ctxKey int

const userCtxKey ctxKey = iota

func userFrom(ctx context.Context) (AuthUser, bool) {
	u, ok := ctx.Value(userCtxKey).(AuthUser)
	return u, ok
}

func hashPassword(password string) (string, error) {
	return argon2id.CreateHash(password, argon2id.DefaultParams)
}

func verifyPassword(password, hash string) bool {
	match, err := argon2id.ComparePasswordAndHash(password, hash)
	return err == nil && match
}

// generateToken returns a 256-bit random session token, hex-encoded.
func generateToken() string {
	var b [32]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func sessionExpiry() string {
	return time.Now().UTC().Add(sessionTTL).Format(time.RFC3339)
}

// setSessionCookie writes the session cookie. Not marked Secure so it works over
// plain HTTP in dev; put the gateway behind TLS in production.
func setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL / time.Second),
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// requireAuth validates the session cookie, loads the user, and injects AuthUser
// into the request context. Responds 401 when not authenticated.
func (st *State) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "not authenticated")
			return
		}
		sess, err := st.DB.GetSession(r.Context(), c.Value)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if sess == nil {
			writeErr(w, http.StatusUnauthorized, "invalid session")
			return
		}
		if exp, err := time.Parse(time.RFC3339, sess.ExpiresAt); err == nil && exp.Before(time.Now()) {
			_ = st.DB.DeleteSession(r.Context(), c.Value)
			writeErr(w, http.StatusUnauthorized, "session expired")
			return
		}
		u, err := st.DB.GetUser(r.Context(), sess.UserID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if u == nil {
			writeErr(w, http.StatusUnauthorized, "user no longer exists")
			return
		}
		ctx := context.WithValue(r.Context(), userCtxKey, AuthUser{ID: u.ID, Username: u.Username, Role: u.Role})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// enforceRole runs after requireAuth: viewers may only make safe (read)
// requests; any state-changing request requires admin. Logout is always allowed.
func (st *State) enforceRole(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _ := userFrom(r.Context())
		safe := r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions
		alwaysAllowed := r.URL.Path == "/api/auth/logout"
		if !safe && !alwaysAllowed && !u.IsAdmin() {
			writeErr(w, http.StatusForbidden, "read-only account: this action requires an admin")
			return
		}
		next.ServeHTTP(w, r)
	})
}
