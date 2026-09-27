package storage

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func tempRepo(t *testing.T) Repository {
	t.Helper()
	url := "sqlite://" + filepath.Join(t.TempDir(), "test.db")
	repo, err := Init(context.Background(), url, false)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	return repo
}

func TestMigrationsAndCRUD(t *testing.T) {
	ctx := context.Background()
	repo := tempRepo(t)

	// Migration 0003 seeds a default local host, so start from that baseline.
	base, _ := repo.CountEnvironments(ctx, "")

	agent := "prod-k8s"
	id, err := repo.CreateEnvironment(ctx, "prod", "unix:///var/run/docker.sock", &agent, nil)
	if err != nil {
		t.Fatalf("CreateEnvironment: %v", err)
	}
	env, err := repo.GetEnvironment(ctx, id)
	if err != nil || env == nil {
		t.Fatalf("GetEnvironment: %v (env=%v)", err, env)
	}
	if env.Name != "prod" || env.AgentID == nil || *env.AgentID != "prod-k8s" {
		t.Errorf("env mapped wrong: %+v", env)
	}
	if n, _ := repo.CountEnvironments(ctx, ""); n != base+1 {
		t.Errorf("count = %d, want %d", n, base+1)
	}
	// Name search isolates the created host (the seeded local host is named "Local").
	if page, _ := repo.ListEnvironmentsPage(ctx, "prod", 10, 0); len(page) != 1 || page[0].Name != "prod" {
		t.Errorf("search page = %+v, want just [prod]", page)
	}

	// Stacks.
	sid, err := repo.CreateStack(ctx, id, "web", "services: {}")
	if err != nil {
		t.Fatalf("CreateStack: %v", err)
	}
	if err := repo.UpdateStackStatus(ctx, sid, "running"); err != nil {
		t.Fatalf("UpdateStackStatus: %v", err)
	}
	stacks, _ := repo.ListStacks(ctx, id)
	if len(stacks) != 1 || stacks[0].Status != "running" {
		t.Errorf("stacks wrong: %+v", stacks)
	}

	// Events.
	if err := repo.RecordEvent(ctx, "c1", "web", "crash"); err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}
	evs, _ := repo.GetRecentEvents(ctx, 10)
	if len(evs) != 1 || evs[0].Event != "crash" {
		t.Errorf("events wrong: %+v", evs)
	}

	// Users + sessions.
	uid, err := repo.CreateUser(ctx, "admin", "hash", "admin")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if n, _ := repo.CountAdmins(ctx); n != 1 {
		t.Errorf("admins = %d, want 1", n)
	}
	u, _ := repo.GetUserByUsername(ctx, "admin")
	if u == nil || u.ID != uid {
		t.Errorf("GetUserByUsername wrong: %+v", u)
	}
	if err := repo.CreateSession(ctx, "tok", uid, "2099-01-01T00:00:00Z"); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	sess, _ := repo.GetSession(ctx, "tok")
	if sess == nil || sess.UserID != uid {
		t.Errorf("GetSession wrong: %+v", sess)
	}

	// Missing lookups return (nil, nil).
	if got, err := repo.GetEnvironment(ctx, "nope"); err != nil || got != nil {
		t.Errorf("missing env = %v, %v; want nil, nil", got, err)
	}
}

// Exercises the auto-registered environment lifecycle: idempotent upsert
// (restart/reconnect can't duplicate), offline marking, alert eligibility, revive
// on reconnect, and retention deletion.
func TestAutoEnvironmentLifecycle(t *testing.T) {
	ctx := context.Background()
	repo := tempRepo(t)

	const key = "prod:inst-1"
	id1, err := repo.UpsertAutoEnvironment(ctx, key, "web-1", "unix:///var/run/docker.sock")
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Idempotent: same identity upserts the SAME row (a restart / reconnect),
	// updating the name rather than creating a duplicate.
	id2, err := repo.UpsertAutoEnvironment(ctx, key, "web-1-renamed", "unix:///var/run/docker.sock")
	if err != nil {
		t.Fatalf("upsert 2: %v", err)
	}
	if id1 != id2 {
		t.Fatalf("upsert not idempotent: %s != %s", id1, id2)
	}
	env, _ := repo.GetEnvironment(ctx, id1)
	if env == nil || env.Source != "auto" || env.Status != "online" || env.Name != "web-1-renamed" {
		t.Fatalf("unexpected env after upsert: %+v", env)
	}

	// Abrupt disconnect → offline with offline_since; alert-eligible.
	if err := repo.MarkEnvironmentOffline(ctx, key, false); err != nil {
		t.Fatalf("offline: %v", err)
	}
	env, _ = repo.GetEnvironment(ctx, id1)
	if env.Status != "offline" || env.OfflineSince == nil {
		t.Fatalf("expected offline w/ offline_since: %+v", env)
	}
	pending, err := repo.ListEnvironmentsPendingAlert(ctx, 0)
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if len(pending) != 1 || pending[0].ID != id1 {
		t.Fatalf("expected 1 pending alert, got %+v", pending)
	}
	// Once alerted, it doesn't re-fire.
	if err := repo.MarkEnvironmentAlerted(ctx, id1); err != nil {
		t.Fatalf("alerted: %v", err)
	}
	if p, _ := repo.ListEnvironmentsPendingAlert(ctx, 0); len(p) != 0 {
		t.Fatalf("expected no pending after alerted, got %+v", p)
	}

	// Reconnect revives: online again, offline_since cleared, alert reset.
	if _, err := repo.UpsertAutoEnvironment(ctx, key, "web-1", "unix:///var/run/docker.sock"); err != nil {
		t.Fatalf("revive: %v", err)
	}
	env, _ = repo.GetEnvironment(ctx, id1)
	if env.Status != "online" || env.OfflineSince != nil {
		t.Fatalf("expected revived online: %+v", env)
	}

	// Graceful disconnect (scale-down) → offline but NOT alert-eligible.
	if err := repo.MarkEnvironmentOffline(ctx, key, true); err != nil {
		t.Fatalf("graceful offline: %v", err)
	}
	if p, _ := repo.ListEnvironmentsPendingAlert(ctx, 0); len(p) != 0 {
		t.Fatalf("graceful disconnect should not alert, got %+v", p)
	}

	// Retention: after the offline window elapses, the row is hard-deleted.
	// offline_since has second precision, so sleep past a clean 2s boundary to
	// avoid an equal-timestamp (non-strict) comparison.
	time.Sleep(2100 * time.Millisecond)
	n, err := repo.DeleteExpiredEnvironments(ctx, 0)
	if err != nil {
		t.Fatalf("delete expired: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 deleted, got %d", n)
	}
	if got, _ := repo.GetEnvironment(ctx, id1); got != nil {
		t.Fatalf("expected env gone, got %+v", got)
	}
}

// Manual environments (the API path / seeded Local host) must never be touched by
// the auto lifecycle sweeps.
func TestManualEnvironmentUntouchedByLifecycle(t *testing.T) {
	ctx := context.Background()
	repo := tempRepo(t)

	id, err := repo.CreateEnvironment(ctx, "manual-host", "unix:///var/run/docker.sock", nil, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.MarkStaleOffline(ctx, 0); err != nil {
		t.Fatalf("mark stale: %v", err)
	}
	if _, err := repo.DeleteExpiredEnvironments(ctx, 0); err != nil {
		t.Fatalf("delete expired: %v", err)
	}
	env, _ := repo.GetEnvironment(ctx, id)
	if env == nil || env.Source != "manual" || env.Status != "online" {
		t.Fatalf("manual env should be untouched: %+v", env)
	}
}

func TestSettingsDefaultsAndUpdate(t *testing.T) {
	ctx := context.Background()
	repo := tempRepo(t)

	all, err := repo.ListSettings(ctx)
	if err != nil {
		t.Fatalf("list settings: %v", err)
	}
	if all["agent_retention_hours"] != "6" {
		t.Fatalf("expected seeded retention default 6, got %q", all["agent_retention_hours"])
	}
	if err := repo.SetSetting(ctx, "agent_retention_hours", "12"); err != nil {
		t.Fatalf("set: %v", err)
	}
	v, _ := repo.GetSetting(ctx, "agent_retention_hours")
	if v == nil || *v != "12" {
		t.Fatalf("expected 12 after update, got %v", v)
	}
}

func TestMigrationMismatch(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "ahead.db")
	url := "sqlite://" + path

	// Migrate normally, then fake a future migration the binary doesn't have.
	if _, err := Init(ctx, url, false); err != nil {
		t.Fatalf("initial Init: %v", err)
	}
	db, _ := sql.Open("sqlite", path)
	if _, err := db.Exec("INSERT INTO schema_migrations (version, applied_at) VALUES (9999, 'x')"); err != nil {
		t.Fatalf("seed future migration: %v", err)
	}
	db.Close()

	// Without reset → MismatchError.
	if _, err := Init(ctx, url, false); err == nil {
		t.Error("expected a mismatch error for a DB ahead of the binary")
	}

	// With reset → recreated and healthy.
	repo, err := Init(ctx, url, true)
	if err != nil {
		t.Fatalf("reset Init: %v", err)
	}
	if n, _ := repo.CountUsers(ctx); n != 0 {
		t.Errorf("after reset users = %d, want 0 (fresh db)", n)
	}
}
