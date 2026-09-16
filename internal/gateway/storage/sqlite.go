package storage

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// SqliteRepository is the modernc.org/sqlite-backed Repository (ports
// gateway/src/storage/sqlite.rs).
type SqliteRepository struct {
	db *sql.DB
}

func newID() string      { return uuid.NewString() }
func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }

// ptrArg turns a *string into a NULL-or-value SQL argument.
func ptrArg(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

// nsToPtr turns a scanned NullString into a *string.
func nsToPtr(n sql.NullString) *string {
	if n.Valid {
		v := n.String
		return &v
	}
	return nil
}

// ── Events ───────────────────────────────────────────────────────────────────

func (r *SqliteRepository) RecordEvent(ctx context.Context, containerID, containerName, event string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO container_events (id, container_id, container_name, event, occurred_at)
		 VALUES (?, ?, ?, ?, ?)`,
		newID(), containerID, containerName, event, nowRFC3339())
	return err
}

func (r *SqliteRepository) GetRecentEvents(ctx context.Context, limit int64) ([]protocol.ContainerEvent, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, container_id, container_name, event, occurred_at
		 FROM container_events ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []protocol.ContainerEvent
	for rows.Next() {
		var e protocol.ContainerEvent
		if err := rows.Scan(&e.ID, &e.ContainerID, &e.ContainerName, &e.Event, &e.OccurredAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ── Environments (hosts) ─────────────────────────────────────────────────────

func (r *SqliteRepository) ListEnvironmentsPage(ctx context.Context, search string, limit, offset int64) ([]protocol.Environment, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, docker_host, is_local, created_at, agent_id FROM environments
		 WHERE name LIKE ?
		 ORDER BY is_local DESC, created_at ASC
		 LIMIT ? OFFSET ?`,
		"%"+search+"%", limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []protocol.Environment
	for rows.Next() {
		env, err := scanEnv(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, env)
	}
	return out, rows.Err()
}

func (r *SqliteRepository) CountEnvironments(ctx context.Context, search string) (int64, error) {
	var n int64
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM environments WHERE name LIKE ?", "%"+search+"%").Scan(&n)
	return n, err
}

func (r *SqliteRepository) GetEnvironment(ctx context.Context, id string) (*protocol.Environment, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, docker_host, is_local, created_at, agent_id FROM environments WHERE id = ?`, id)
	env, err := scanEnv(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &env, nil
}

func (r *SqliteRepository) CreateEnvironment(ctx context.Context, name, dockerHost string, agentID *string) (string, error) {
	id := newID()
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO environments (id, name, docker_host, is_local, agent_id) VALUES (?, ?, ?, 0, ?)`,
		id, name, dockerHost, ptrArg(agentID))
	return id, err
}

func (r *SqliteRepository) RenameEnvironment(ctx context.Context, id, name string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE environments SET name = ? WHERE id = ?", name, id)
	return err
}

func (r *SqliteRepository) DeleteEnvironment(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM environments WHERE id = ?", id)
	return err
}

// ── Stacks ───────────────────────────────────────────────────────────────────

const stackCols = `id, host_id, name, content, env_vars, webhook_token, status, created_at, updated_at`

func (r *SqliteRepository) ListStacks(ctx context.Context, hostID string) ([]protocol.SavedStack, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+stackCols+` FROM stacks WHERE host_id = ? ORDER BY updated_at DESC`, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []protocol.SavedStack
	for rows.Next() {
		s, err := scanStack(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *SqliteRepository) GetStack(ctx context.Context, id string) (*protocol.SavedStack, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+stackCols+` FROM stacks WHERE id = ?`, id)
	s, err := scanStack(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SqliteRepository) CreateStack(ctx context.Context, hostID, name, content string) (string, error) {
	id, now := newID(), nowRFC3339()
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO stacks (id, host_id, name, content, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, 'created', ?, ?)`,
		id, hostID, name, content, now, now)
	return id, err
}

func (r *SqliteRepository) UpdateStack(ctx context.Context, id, content string, envVars *string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE stacks SET content = ?, env_vars = ?, updated_at = ? WHERE id = ?",
		content, ptrArg(envVars), nowRFC3339(), id)
	return err
}

func (r *SqliteRepository) UpdateStackStatus(ctx context.Context, id, status string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE stacks SET status = ?, updated_at = ? WHERE id = ?", status, nowRFC3339(), id)
	return err
}

func (r *SqliteRepository) SetStackWebhookToken(ctx context.Context, id string, token *string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE stacks SET webhook_token = ?, updated_at = ? WHERE id = ?", ptrArg(token), nowRFC3339(), id)
	return err
}

func (r *SqliteRepository) DeleteStack(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM stacks WHERE id = ?", id)
	return err
}

// ── Generic webhook tokens ───────────────────────────────────────────────────

func (r *SqliteRepository) GetWebhookToken(ctx context.Context, entityType, entityKey, hostID string) (*string, error) {
	var token string
	err := r.db.QueryRowContext(ctx,
		"SELECT token FROM webhook_tokens WHERE entity_type = ? AND entity_key = ? AND host_id = ?",
		entityType, entityKey, hostID).Scan(&token)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func (r *SqliteRepository) SetWebhookToken(ctx context.Context, entityType, entityKey, hostID, token string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO webhook_tokens (id, entity_type, entity_key, host_id, token, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(entity_type, entity_key, host_id) DO UPDATE SET token = excluded.token, created_at = excluded.created_at`,
		newID(), entityType, entityKey, hostID, token, nowRFC3339())
	return err
}

func (r *SqliteRepository) DeleteWebhookToken(ctx context.Context, entityType, entityKey, hostID string) error {
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM webhook_tokens WHERE entity_type = ? AND entity_key = ? AND host_id = ?",
		entityType, entityKey, hostID)
	return err
}

// ── Users ────────────────────────────────────────────────────────────────────

func (r *SqliteRepository) CountUsers(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&n)
	return n, err
}

func (r *SqliteRepository) CountAdmins(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&n)
	return n, err
}

func (r *SqliteRepository) CreateUser(ctx context.Context, username, passwordHash, role string) (string, error) {
	id := newID()
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO users (id, username, password_hash, role, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, username, passwordHash, role, nowRFC3339())
	return id, err
}

const userCols = `id, username, password_hash, role, created_at`

func (r *SqliteRepository) GetUser(ctx context.Context, id string) (*User, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE id = ?`, id)
	return scanUserPtr(row)
}

func (r *SqliteRepository) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE username = ?`, username)
	return scanUserPtr(row)
}

func (r *SqliteRepository) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+userCols+` FROM users ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (r *SqliteRepository) UpdateUserRole(ctx context.Context, id, role string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE users SET role = ? WHERE id = ?", role, id)
	return err
}

func (r *SqliteRepository) UpdateUserPassword(ctx context.Context, id, passwordHash string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE users SET password_hash = ? WHERE id = ?", passwordHash, id)
	return err
}

func (r *SqliteRepository) DeleteUser(ctx context.Context, id string) error {
	// Sessions cascade via the foreign key.
	_, err := r.db.ExecContext(ctx, "DELETE FROM users WHERE id = ?", id)
	return err
}

// ── Sessions ─────────────────────────────────────────────────────────────────

func (r *SqliteRepository) CreateSession(ctx context.Context, token, userID, expiresAt string) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO sessions (token, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)",
		token, userID, nowRFC3339(), expiresAt)
	return err
}

func (r *SqliteRepository) GetSession(ctx context.Context, token string) (*Session, error) {
	var s Session
	err := r.db.QueryRowContext(ctx,
		"SELECT token, user_id, created_at, expires_at FROM sessions WHERE token = ?", token).
		Scan(&s.Token, &s.UserID, &s.CreatedAt, &s.ExpiresAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SqliteRepository) DeleteSession(ctx context.Context, token string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM sessions WHERE token = ?", token)
	return err
}

func (r *SqliteRepository) DeleteUserSessions(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM sessions WHERE user_id = ?", userID)
	return err
}

// ── Row scanners ─────────────────────────────────────────────────────────────

type scanner interface {
	Scan(dest ...any) error
}

func scanEnv(s scanner) (protocol.Environment, error) {
	var e protocol.Environment
	var isLocal int64
	var agentID sql.NullString
	err := s.Scan(&e.ID, &e.Name, &e.DockerHost, &isLocal, &e.CreatedAt, &agentID)
	if err != nil {
		return e, err
	}
	e.IsLocal = isLocal != 0
	e.AgentID = nsToPtr(agentID)
	return e, nil
}

func scanStack(s scanner) (protocol.SavedStack, error) {
	var st protocol.SavedStack
	var envVars, webhookToken sql.NullString
	err := s.Scan(&st.ID, &st.HostID, &st.Name, &st.Content, &envVars, &webhookToken,
		&st.Status, &st.CreatedAt, &st.UpdatedAt)
	if err != nil {
		return st, err
	}
	st.EnvVars = nsToPtr(envVars)
	st.WebhookToken = nsToPtr(webhookToken)
	return st, nil
}

func scanUserPtr(s scanner) (*User, error) {
	var u User
	err := s.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}
