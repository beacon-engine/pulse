// Package storage is the Go port of gateway/src/storage — the gateway's
// database (hosts, stacks, webhook tokens, events, users, sessions). The rest of
// the gateway depends only on the Repository interface, so the backend can be
// swapped. The implementation here is pure-Go SQLite (modernc.org/sqlite), so the
// gateway builds as a static binary with no cgo.
package storage

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// User is a dashboard user. PasswordHash is never serialized to API clients.
type User struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"`
	CreatedAt    string `json:"created_at"`
}

// Session backs a login session cookie.
type Session struct {
	Token     string
	UserID    string
	CreatedAt string
	ExpiresAt string
}

// Repository is the storage interface the gateway depends on. Docker/host record
// types (Environment, SavedStack, ContainerEvent) come from the protocol package.
type Repository interface {
	RecordEvent(ctx context.Context, containerID, containerName, event string) error
	GetRecentEvents(ctx context.Context, limit int64) ([]protocol.ContainerEvent, error)

	ListEnvironmentsPage(ctx context.Context, search string, limit, offset int64) ([]protocol.Environment, error)
	CountEnvironments(ctx context.Context, search string) (int64, error)
	GetEnvironment(ctx context.Context, id string) (*protocol.Environment, error)
	CreateEnvironment(ctx context.Context, name, dockerHost string, agentID, agentAddr *string) (string, error)
	RenameEnvironment(ctx context.Context, id, name string) error
	DeleteEnvironment(ctx context.Context, id string) error

	// Auto-registered environment lifecycle (self-registering dial-in agents).
	UpsertAutoEnvironment(ctx context.Context, agentKey, name, dockerHost string) (string, error)
	TouchEnvironment(ctx context.Context, agentKey string) error
	MarkEnvironmentOffline(ctx context.Context, agentKey string, graceful bool) error
	MarkStaleOffline(ctx context.Context, thresholdSeconds int) error
	ListEnvironmentsPendingAlert(ctx context.Context, graceSeconds int) ([]protocol.Environment, error)
	MarkEnvironmentAlerted(ctx context.Context, id string) error
	DeleteExpiredEnvironments(ctx context.Context, retentionSeconds int) (int64, error)

	// Pull agents (manual environments with an agent address): the gateway
	// probes them, since they never call home.
	ListPullEnvironments(ctx context.Context) ([]protocol.Environment, error)
	SetPullStatus(ctx context.Context, id string, online bool) error

	// Runtime-tunable settings (DB-backed).
	GetSetting(ctx context.Context, key string) (*string, error)
	SetSetting(ctx context.Context, key, value string) error
	ListSettings(ctx context.Context) (map[string]string, error)

	ListStacks(ctx context.Context, hostID string) ([]protocol.SavedStack, error)
	GetStack(ctx context.Context, id string) (*protocol.SavedStack, error)
	CreateStack(ctx context.Context, hostID, name, content string) (string, error)
	UpdateStack(ctx context.Context, id, content string, envVars *string) error
	UpdateStackStatus(ctx context.Context, id, status string) error
	SetStackWebhookToken(ctx context.Context, id string, token *string) error
	DeleteStack(ctx context.Context, id string) error

	GetWebhookToken(ctx context.Context, entityType, entityKey, hostID string) (*string, error)
	SetWebhookToken(ctx context.Context, entityType, entityKey, hostID, token string) error
	DeleteWebhookToken(ctx context.Context, entityType, entityKey, hostID string) error

	CountUsers(ctx context.Context) (int64, error)
	CountAdmins(ctx context.Context) (int64, error)
	CreateUser(ctx context.Context, username, passwordHash, role string) (string, error)
	GetUser(ctx context.Context, id string) (*User, error)
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	ListUsers(ctx context.Context) ([]User, error)
	UpdateUserRole(ctx context.Context, id, role string) error
	UpdateUserPassword(ctx context.Context, id, passwordHash string) error
	DeleteUser(ctx context.Context, id string) error

	CreateSession(ctx context.Context, token, userID, expiresAt string) error
	GetSession(ctx context.Context, token string) (*Session, error)
	DeleteSession(ctx context.Context, token string) error
	DeleteUserSessions(ctx context.Context, userID string) error
}

//go:embed migrations/*.sql
var migrationsFS embed.FS

// MismatchError signals the database has a migration this binary doesn't (the DB
// is ahead of the binary — a stale build). Mirrors sqlx's VersionMissing.
type MismatchError struct{ Version int }

func (e *MismatchError) Error() string {
	return fmt.Sprintf("database has migration %d applied but this binary doesn't include it (stale build?)", e.Version)
}

// Init opens the database, runs migrations, and returns a Repository. On a
// migration mismatch it either recreates the database (resetOnMismatch, dev only)
// or returns a MismatchError.
func Init(ctx context.Context, databaseURL string, resetOnMismatch bool) (Repository, error) {
	path := sqlitePath(databaseURL)
	db, err := openDB(path)
	if err != nil {
		return nil, err
	}

	if err := runMigrations(ctx, db); err != nil {
		var mm *MismatchError
		if errors.As(err, &mm) && resetOnMismatch && path != ":memory:" {
			fmt.Fprintf(os.Stderr, "⚠️  %v; PULSE_DB_RESET_ON_MISMATCH is set — recreating the database. ALL DATA WILL BE LOST.\n", err)
			_ = db.Close()
			removeDBFiles(path)
			if db, err = openDB(path); err != nil {
				return nil, err
			}
			if err := runMigrations(ctx, db); err != nil {
				return nil, err
			}
		} else {
			_ = db.Close()
			return nil, err
		}
	}
	return &SqliteRepository{db: db}, nil
}

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// A single writer avoids "database is locked" on SQLite; also required so an
	// in-memory DB is one shared database across the pool.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// sqlitePath turns a DATABASE_URL (sqlite://file, sqlite:///abs, sqlite::memory:)
// into a modernc.org/sqlite DSN path.
func sqlitePath(url string) string {
	switch {
	case strings.HasPrefix(url, "sqlite://"):
		return strings.TrimPrefix(url, "sqlite://")
	case strings.HasPrefix(url, "sqlite:"):
		return strings.TrimPrefix(url, "sqlite:") // sqlite::memory: -> :memory:
	default:
		return url
	}
}

func removeDBFiles(path string) {
	for _, suffix := range []string{"", "-wal", "-shm"} {
		_ = os.Remove(path + suffix)
	}
}

type migration struct {
	version int
	name    string
	sql     string
}

func loadMigrations() ([]migration, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, err
	}
	var migs []migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		num, _, ok := strings.Cut(e.Name(), "_")
		if !ok {
			continue
		}
		v, err := strconv.Atoi(num)
		if err != nil {
			continue
		}
		body, err := migrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			return nil, err
		}
		migs = append(migs, migration{version: v, name: e.Name(), sql: string(body)})
	}
	sort.Slice(migs, func(i, j int) bool { return migs[i].version < migs[j].version })
	return migs, nil
}

// runMigrations applies embedded migrations not yet recorded. It seeds the
// applied set from a pre-existing sqlx `_sqlx_migrations` table too, so a
// database migrated by the old Rust gateway is picked up without re-running.
func runMigrations(ctx context.Context, db *sql.DB) error {
	migs, err := loadMigrations()
	if err != nil {
		return err
	}
	if len(migs) == 0 {
		return nil
	}
	maxEmbedded := migs[len(migs)-1].version

	if _, err := db.ExecContext(ctx,
		"CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT)"); err != nil {
		return err
	}

	applied := map[int]bool{}
	for _, table := range []string{"schema_migrations", "_sqlx_migrations"} {
		versions, err := appliedVersions(ctx, db, table)
		if err != nil {
			return err
		}
		for _, v := range versions {
			applied[v] = true
		}
	}

	// DB ahead of binary → mismatch.
	for v := range applied {
		if v > maxEmbedded {
			return &MismatchError{Version: v}
		}
	}

	for _, m := range migs {
		if applied[m.version] {
			continue
		}
		if _, err := db.ExecContext(ctx, m.sql); err != nil {
			return fmt.Errorf("migration %s: %w", m.name, err)
		}
		if _, err := db.ExecContext(ctx,
			"INSERT INTO schema_migrations (version, applied_at) VALUES (?, datetime('now'))", m.version); err != nil {
			return err
		}
	}
	return nil
}

// appliedVersions reads the version column of a migrations table, returning nil
// if the table doesn't exist.
func appliedVersions(ctx context.Context, db *sql.DB, table string) ([]int, error) {
	var exists string
	err := db.QueryRowContext(ctx,
		"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&exists)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, "SELECT version FROM "+table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
