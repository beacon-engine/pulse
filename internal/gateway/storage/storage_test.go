package storage

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
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
	id, err := repo.CreateEnvironment(ctx, "prod", "unix:///var/run/docker.sock", &agent)
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
