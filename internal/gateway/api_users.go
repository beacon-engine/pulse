package gateway

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Ports gateway/src/api/users.rs — user management (admin only).

func validRole(role string) bool { return role == "admin" || role == "viewer" }

// requireAdmin writes 403 and returns false unless the caller is an admin.
func requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if u, ok := userFrom(r.Context()); ok && u.IsAdmin() {
		return true
	}
	writeErr(w, http.StatusForbidden, "admin role required")
	return false
}

func (st *State) handleListUsers(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	users, err := st.DB.ListUsers(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, orEmpty(users))
}

type createUserBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

func (st *State) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var body createUserBody
	_ = json.NewDecoder(r.Body).Decode(&body)

	if strings.TrimSpace(body.Username) == "" {
		writeErr(w, http.StatusBadRequest, "username is required")
		return
	}
	if len(body.Password) < 8 {
		writeErr(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	if !validRole(body.Role) {
		writeErr(w, http.StatusBadRequest, "role must be 'admin' or 'viewer'")
		return
	}

	existing, err := st.DB.GetUserByUsername(r.Context(), body.Username)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if existing != nil {
		writeErr(w, http.StatusConflict, "username already taken")
		return
	}

	hash, err := hashPassword(body.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	id, err := st.DB.CreateUser(r.Context(), body.Username, hash, body.Role)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	created, err := st.DB.GetUser(r.Context(), id)
	if err != nil || created == nil {
		writeErr(w, http.StatusInternalServerError, "not found after insert")
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

type updateUserBody struct {
	Role     *string `json:"role"`
	Password *string `json:"password"`
}

func (st *State) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := r.PathValue("id")
	target, err := st.DB.GetUser(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if target == nil {
		writeErr(w, http.StatusNotFound, "user "+id+" not found")
		return
	}

	var body updateUserBody
	_ = json.NewDecoder(r.Body).Decode(&body)

	if body.Role != nil {
		if !validRole(*body.Role) {
			writeErr(w, http.StatusBadRequest, "role must be 'admin' or 'viewer'")
			return
		}
		if target.Role == "admin" && *body.Role != "admin" {
			admins, err := st.DB.CountAdmins(r.Context())
			if err != nil {
				writeErr(w, http.StatusInternalServerError, err.Error())
				return
			}
			if admins <= 1 {
				writeErr(w, http.StatusBadRequest, "cannot demote the last admin")
				return
			}
		}
		if err := st.DB.UpdateUserRole(r.Context(), id, *body.Role); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	if body.Password != nil {
		if len(*body.Password) < 8 {
			writeErr(w, http.StatusBadRequest, "password must be at least 8 characters")
			return
		}
		hash, err := hashPassword(*body.Password)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := st.DB.UpdateUserPassword(r.Context(), id, hash); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		// Changing a password invalidates existing sessions for that user.
		_ = st.DB.DeleteUserSessions(r.Context(), id)
	}

	updated, err := st.DB.GetUser(r.Context(), id)
	if err != nil || updated == nil {
		writeErr(w, http.StatusNotFound, "user "+id+" not found")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (st *State) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := r.PathValue("id")
	if u, _ := userFrom(r.Context()); u.ID == id {
		writeErr(w, http.StatusBadRequest, "you cannot delete your own account")
		return
	}
	target, err := st.DB.GetUser(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if target == nil {
		writeErr(w, http.StatusNotFound, "user "+id+" not found")
		return
	}
	if target.Role == "admin" {
		admins, err := st.DB.CountAdmins(r.Context())
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if admins <= 1 {
			writeErr(w, http.StatusBadRequest, "cannot delete the last admin")
			return
		}
	}
	if err := st.DB.DeleteUser(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
