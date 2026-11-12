package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/beacon-engine/pulse/internal/gateway/liveness"
)

// Runtime-tunable gateway settings. Currently these are the auto-registered agent
// liveness thresholds consumed by the sweeper; keeping them in the DB (rather
// than env) lets operators tune them at runtime and, later, from a settings page.
// GET is available to any authenticated user; PUT is admin-only.

func (st *State) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := st.DB.ListSettings(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if settings == nil {
		settings = map[string]string{}
	}
	writeJSON(w, http.StatusOK, settings)
}

func (st *State) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var body map[string]string
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	// Validate everything before writing anything.
	for k, v := range body {
		if !allowedSetting(k) {
			writeErr(w, http.StatusBadRequest, "unknown setting: "+k)
			return
		}
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err != nil || n < 0 {
			writeErr(w, http.StatusBadRequest, "setting "+k+" must be a non-negative integer")
			return
		}
	}
	for k, v := range body {
		if err := st.DB.SetSetting(r.Context(), k, strings.TrimSpace(v)); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	settings, err := st.DB.ListSettings(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

// allowedSetting whitelists the keys the API may write, so the settings table
// can't be used as arbitrary key/value storage.
func allowedSetting(key string) bool {
	switch key {
	case liveness.SettingOfflineThreshold, liveness.SettingAlertGrace, liveness.SettingRetentionHours:
		return true
	default:
		return false
	}
}
