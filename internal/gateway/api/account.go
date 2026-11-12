package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/beacon-engine/pulse/internal/gateway/auth"
)

// Ports gateway/src/api/auth.rs — first-run setup, login, logout, "who am I".

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func validateCredentials(username, password string) error {
	if strings.TrimSpace(username) == "" {
		return errors.New("username is required")
	}
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	return nil
}

// issueSession creates a session and sets the cookie. Returns false (having
// written the error) on failure.
func (st *State) issueSession(w http.ResponseWriter, r *http.Request, userID string) bool {
	token := auth.NewToken()
	if err := st.DB.CreateSession(r.Context(), token, userID, auth.SessionExpiry()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return false
	}
	auth.SetSessionCookie(w, token)
	return true
}

func (st *State) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	n, err := st.DB.CountUsers(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"needs_setup": n == 0})
}

func (st *State) handleSetup(w http.ResponseWriter, r *http.Request) {
	n, err := st.DB.CountUsers(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n > 0 {
		writeErr(w, http.StatusConflict, "setup already completed")
		return
	}
	var body credentials
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := validateCredentials(body.Username, body.Password); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	id, err := st.DB.CreateUser(r.Context(), body.Username, hash, "admin")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !st.issueSession(w, r, id) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "username": body.Username, "role": "admin"})
}

func (st *State) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body credentials
	_ = json.NewDecoder(r.Body).Decode(&body)

	u, err := st.DB.GetUserByUsername(r.Context(), body.Username)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if u == nil || !auth.VerifyPassword(body.Password, u.PasswordHash) {
		writeErr(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	if !st.issueSession(w, r, u.ID) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": u.ID, "username": u.Username, "role": u.Role})
}

func (st *State) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(auth.CookieName); err == nil {
		_ = st.DB.DeleteSession(r.Context(), c.Value)
	}
	auth.ClearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (st *State) handleMe(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": u.ID, "username": u.Username, "role": u.Role})
}
