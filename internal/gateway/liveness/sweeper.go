// Package liveness keeps environments' online/offline status true: it marks
// silent push agents offline, pings pull agents, alerts on outages and
// removes long-gone self-registered environments.
package liveness

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/beacon-engine/pulse/internal/gateway/agents"
	"github.com/beacon-engine/pulse/internal/gateway/events"
	"github.com/beacon-engine/pulse/internal/gateway/storage"
)

// The liveness sweeper enforces the auto-registered environment lifecycle. Its
// thresholds are DB-backed settings (editable via /api/settings), so operators
// tune them at runtime instead of baking them into container env. The constants
// below are only fallbacks when a setting row is missing/malformed.

// sweepInterval is how often the sweeper runs. Kept well under the offline
// threshold so staleness is caught promptly.
const sweepInterval = 15 * time.Second

const (
	SettingOfflineThreshold = "agent_offline_threshold_seconds"
	SettingAlertGrace       = "agent_alert_grace_seconds"
	SettingRetentionHours   = "agent_retention_hours"

	defaultOfflineThreshold = 90 // seconds without a heartbeat → offline
	defaultAlertGrace       = 60 // seconds offline before alerting
	defaultRetentionHours   = 6  // hours offline before hard delete
)

// The sweeper periodically: (1) marks agents whose heartbeat went stale
// offline (the backstop for a half-open socket), (2) alerts on abrupt
// disconnects that stay down past the grace window (graceful scale-downs are
// skipped), and (3) hard-deletes environments offline past the retention window.
// Monitor holds what the liveness loops need.
type Monitor struct {
	DB       storage.Repository
	Router   *agents.Router
	Notifier *events.Notifier
}

// RunSweeper runs the sweeper until ctx ends.
func (m *Monitor) RunSweeper(ctx context.Context) {
	t := time.NewTicker(sweepInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.sweepAgents(ctx)
		}
	}
}

func (m *Monitor) sweepAgents(ctx context.Context) {
	offlineThreshold := m.settingInt(ctx, SettingOfflineThreshold, defaultOfflineThreshold)
	alertGrace := m.settingInt(ctx, SettingAlertGrace, defaultAlertGrace)
	retentionHours := m.settingInt(ctx, SettingRetentionHours, defaultRetentionHours)

	// 1. Heartbeat backstop.
	if err := m.DB.MarkStaleOffline(ctx, offlineThreshold); err != nil {
		log.Printf("sweeper: mark stale offline: %v", err)
	}

	// 2. Alert on abrupt disconnects still down past the grace window.
	pending, err := m.DB.ListEnvironmentsPendingAlert(ctx, alertGrace)
	if err != nil {
		log.Printf("sweeper: list pending alerts: %v", err)
	}
	for _, env := range pending {
		why := "no heartbeat"
		if env.Source == "manual" {
			why = "unreachable"
		}
		msg := fmt.Sprintf(":red_circle: *Pulse* — agent `%s` is down (%s)", env.Name, why)
		m.Notifier.Slack(ctx, msg)
		m.Notifier.Hub.Broadcast(fmt.Sprintf(`{"event":"agent_down","host":"%s"}`, env.Name))
		if err := m.DB.MarkEnvironmentAlerted(ctx, env.ID); err != nil {
			log.Printf("sweeper: mark alerted %s: %v", env.ID, err)
		}
	}

	// 3. Retention: hard-delete environments offline beyond the window.
	if n, err := m.DB.DeleteExpiredEnvironments(ctx, retentionHours*3600); err != nil {
		log.Printf("sweeper: delete expired: %v", err)
	} else if n > 0 {
		log.Printf("sweeper: removed %d expired offline environment(s)", n)
	}
}

// settingInt reads an integer setting, falling back to def when the row is
// missing, unreadable, or not a non-negative integer.
func (m *Monitor) settingInt(ctx context.Context, key string, def int) int {
	v, err := m.DB.GetSetting(ctx, key)
	if err != nil || v == nil {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(*v))
	if err != nil || n < 0 {
		return def
	}
	return n
}
