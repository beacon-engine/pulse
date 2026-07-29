use async_trait::async_trait;
use chrono::Utc;
use sqlx::{sqlite::SqliteRow, Row, SqlitePool};
use uuid::Uuid;

use crate::storage::{
    models::{ContainerEvent, Environment, SavedStack, Session, User},
    repository::Repository,
};

pub struct SqliteRepository {
    pool: SqlitePool,
}

impl SqliteRepository {
    pub fn new(pool: SqlitePool) -> Self {
        Self { pool }
    }
}

#[async_trait]
impl Repository for SqliteRepository {
    async fn record_event(
        &self,
        container_id: &str,
        container_name: &str,
        event: &str,
    ) -> Result<(), sqlx::Error> {
        let id = Uuid::new_v4().to_string();
        sqlx::query(
            "INSERT INTO container_events (id, container_id, container_name, event, occurred_at)
             VALUES (?, ?, ?, ?, ?)",
        )
        .bind(&id)
        .bind(container_id)
        .bind(container_name)
        .bind(event)
        .bind(Utc::now().to_rfc3339())
        .execute(&self.pool)
        .await?;
        Ok(())
    }

    async fn get_recent_events(&self, limit: i64) -> Result<Vec<ContainerEvent>, sqlx::Error> {
        let rows = sqlx::query(
            "SELECT id, container_id, container_name, event, occurred_at
             FROM container_events ORDER BY id DESC LIMIT ?",
        )
        .bind(limit)
        .fetch_all(&self.pool)
        .await?;

        Ok(rows
            .iter()
            .map(|r| ContainerEvent {
                id: r.get("id"),
                container_id: r.get("container_id"),
                container_name: r.get("container_name"),
                event: r.get("event"),
                occurred_at: r.get("occurred_at"),
            })
            .collect())
    }

    async fn list_environments(&self) -> Result<Vec<Environment>, sqlx::Error> {
        let rows = sqlx::query(
            "SELECT id, name, docker_host, is_local, created_at FROM environments ORDER BY created_at ASC",
        )
        .fetch_all(&self.pool)
        .await?;

        Ok(rows.iter().map(row_to_env).collect())
    }

    async fn list_environments_page(
        &self,
        search: Option<&str>,
        limit: i64,
        offset: i64,
    ) -> Result<Vec<Environment>, sqlx::Error> {
        // Local host first, then newest; LIKE is case-insensitive for ASCII.
        let pattern = format!("%{}%", search.unwrap_or(""));
        let rows = sqlx::query(
            "SELECT id, name, docker_host, is_local, created_at FROM environments
             WHERE name LIKE ?
             ORDER BY is_local DESC, created_at ASC
             LIMIT ? OFFSET ?",
        )
        .bind(pattern)
        .bind(limit)
        .bind(offset)
        .fetch_all(&self.pool)
        .await?;
        Ok(rows.iter().map(row_to_env).collect())
    }

    async fn count_environments(&self, search: Option<&str>) -> Result<i64, sqlx::Error> {
        let pattern = format!("%{}%", search.unwrap_or(""));
        let row = sqlx::query("SELECT COUNT(*) AS n FROM environments WHERE name LIKE ?")
            .bind(pattern)
            .fetch_one(&self.pool)
            .await?;
        Ok(row.get::<i64, _>("n"))
    }

    async fn get_environment(&self, id: &str) -> Result<Option<Environment>, sqlx::Error> {
        let row = sqlx::query(
            "SELECT id, name, docker_host, is_local, created_at FROM environments WHERE id = ?",
        )
        .bind(id)
        .fetch_optional(&self.pool)
        .await?;

        Ok(row.as_ref().map(row_to_env))
    }

    async fn create_environment(
        &self,
        name: &str,
        docker_host: &str,
    ) -> Result<String, sqlx::Error> {
        let id = Uuid::new_v4().to_string();
        sqlx::query(
            "INSERT INTO environments (id, name, docker_host, is_local) VALUES (?, ?, ?, 0)",
        )
        .bind(&id)
        .bind(name)
        .bind(docker_host)
        .execute(&self.pool)
        .await?;
        Ok(id)
    }

    async fn rename_environment(&self, id: &str, name: &str) -> Result<(), sqlx::Error> {
        sqlx::query("UPDATE environments SET name = ? WHERE id = ?")
            .bind(name)
            .bind(id)
            .execute(&self.pool)
            .await?;
        Ok(())
    }

    async fn delete_environment(&self, id: &str) -> Result<(), sqlx::Error> {
        sqlx::query("DELETE FROM environments WHERE id = ?")
            .bind(id)
            .execute(&self.pool)
            .await?;
        Ok(())
    }

    async fn list_stacks(&self, host_id: &str) -> Result<Vec<SavedStack>, sqlx::Error> {
        let rows = sqlx::query(
            "SELECT id, host_id, name, content, env_vars, webhook_token, status, created_at, updated_at
             FROM stacks WHERE host_id = ? ORDER BY updated_at DESC",
        )
        .bind(host_id)
        .fetch_all(&self.pool)
        .await?;

        Ok(rows.iter().map(row_to_stack).collect())
    }

    async fn get_stack(&self, id: &str) -> Result<Option<SavedStack>, sqlx::Error> {
        let row = sqlx::query(
            "SELECT id, host_id, name, content, env_vars, webhook_token, status, created_at, updated_at
             FROM stacks WHERE id = ?",
        )
        .bind(id)
        .fetch_optional(&self.pool)
        .await?;

        Ok(row.as_ref().map(row_to_stack))
    }

    async fn create_stack(
        &self,
        host_id: &str,
        name: &str,
        content: &str,
    ) -> Result<String, sqlx::Error> {
        let id = Uuid::new_v4().to_string();
        let now = Utc::now().to_rfc3339();
        sqlx::query(
            "INSERT INTO stacks (id, host_id, name, content, status, created_at, updated_at)
             VALUES (?, ?, ?, ?, ?, ?, ?)",
        )
        .bind(&id)
        .bind(host_id)
        .bind(name)
        .bind(content)
        .bind("created")
        .bind(&now)
        .bind(&now)
        .execute(&self.pool)
        .await?;
        Ok(id)
    }

    async fn update_stack(&self, id: &str, content: &str, env_vars: Option<&str>) -> Result<(), sqlx::Error> {
        sqlx::query(
            "UPDATE stacks SET content = ?, env_vars = ?, updated_at = ? WHERE id = ?",
        )
        .bind(content)
        .bind(env_vars)
        .bind(Utc::now().to_rfc3339())
        .bind(id)
        .execute(&self.pool)
        .await?;
        Ok(())
    }

    async fn set_stack_webhook_token(&self, id: &str, token: Option<&str>) -> Result<(), sqlx::Error> {
        sqlx::query("UPDATE stacks SET webhook_token = ?, updated_at = ? WHERE id = ?")
            .bind(token)
            .bind(Utc::now().to_rfc3339())
            .bind(id)
            .execute(&self.pool)
            .await?;
        Ok(())
    }

    async fn update_stack_status(&self, id: &str, status: &str) -> Result<(), sqlx::Error> {
        sqlx::query(
            "UPDATE stacks SET status = ?, updated_at = ? WHERE id = ?",
        )
        .bind(status)
        .bind(Utc::now().to_rfc3339())
        .bind(id)
        .execute(&self.pool)
        .await?;
        Ok(())
    }

    async fn delete_stack(&self, id: &str) -> Result<(), sqlx::Error> {
        sqlx::query("DELETE FROM stacks WHERE id = ?")
            .bind(id)
            .execute(&self.pool)
            .await?;
        Ok(())
    }

    async fn get_webhook_token(
        &self,
        entity_type: &str,
        entity_key: &str,
        host_id: &str,
    ) -> Result<Option<String>, sqlx::Error> {
        let row = sqlx::query(
            "SELECT token FROM webhook_tokens WHERE entity_type = ? AND entity_key = ? AND host_id = ?",
        )
        .bind(entity_type)
        .bind(entity_key)
        .bind(host_id)
        .fetch_optional(&self.pool)
        .await?;
        Ok(row.map(|r| r.get("token")))
    }

    async fn set_webhook_token(
        &self,
        entity_type: &str,
        entity_key: &str,
        host_id: &str,
        token: &str,
    ) -> Result<(), sqlx::Error> {
        let id = Uuid::new_v4().to_string();
        let now = Utc::now().to_rfc3339();
        sqlx::query(
            "INSERT INTO webhook_tokens (id, entity_type, entity_key, host_id, token, created_at)
             VALUES (?, ?, ?, ?, ?, ?)
             ON CONFLICT(entity_type, entity_key, host_id) DO UPDATE SET token = excluded.token, created_at = excluded.created_at",
        )
        .bind(&id)
        .bind(entity_type)
        .bind(entity_key)
        .bind(host_id)
        .bind(token)
        .bind(&now)
        .execute(&self.pool)
        .await?;
        Ok(())
    }

    async fn delete_webhook_token(
        &self,
        entity_type: &str,
        entity_key: &str,
        host_id: &str,
    ) -> Result<(), sqlx::Error> {
        sqlx::query(
            "DELETE FROM webhook_tokens WHERE entity_type = ? AND entity_key = ? AND host_id = ?",
        )
        .bind(entity_type)
        .bind(entity_key)
        .bind(host_id)
        .execute(&self.pool)
        .await?;
        Ok(())
    }

    // ── Users ─────────────────────────────────────────────────────────────────

    async fn count_users(&self) -> Result<i64, sqlx::Error> {
        let row = sqlx::query("SELECT COUNT(*) AS n FROM users")
            .fetch_one(&self.pool)
            .await?;
        Ok(row.get::<i64, _>("n"))
    }

    async fn count_admins(&self) -> Result<i64, sqlx::Error> {
        let row = sqlx::query("SELECT COUNT(*) AS n FROM users WHERE role = 'admin'")
            .fetch_one(&self.pool)
            .await?;
        Ok(row.get::<i64, _>("n"))
    }

    async fn create_user(
        &self,
        username: &str,
        password_hash: &str,
        role: &str,
    ) -> Result<String, sqlx::Error> {
        let id = Uuid::new_v4().to_string();
        sqlx::query(
            "INSERT INTO users (id, username, password_hash, role, created_at)
             VALUES (?, ?, ?, ?, ?)",
        )
        .bind(&id)
        .bind(username)
        .bind(password_hash)
        .bind(role)
        .bind(Utc::now().to_rfc3339())
        .execute(&self.pool)
        .await?;
        Ok(id)
    }

    async fn get_user(&self, id: &str) -> Result<Option<User>, sqlx::Error> {
        let row = sqlx::query(
            "SELECT id, username, password_hash, role, created_at FROM users WHERE id = ?",
        )
        .bind(id)
        .fetch_optional(&self.pool)
        .await?;
        Ok(row.as_ref().map(row_to_user))
    }

    async fn get_user_by_username(&self, username: &str) -> Result<Option<User>, sqlx::Error> {
        let row = sqlx::query(
            "SELECT id, username, password_hash, role, created_at FROM users WHERE username = ?",
        )
        .bind(username)
        .fetch_optional(&self.pool)
        .await?;
        Ok(row.as_ref().map(row_to_user))
    }

    async fn list_users(&self) -> Result<Vec<User>, sqlx::Error> {
        let rows = sqlx::query(
            "SELECT id, username, password_hash, role, created_at FROM users ORDER BY created_at ASC",
        )
        .fetch_all(&self.pool)
        .await?;
        Ok(rows.iter().map(row_to_user).collect())
    }

    async fn update_user_role(&self, id: &str, role: &str) -> Result<(), sqlx::Error> {
        sqlx::query("UPDATE users SET role = ? WHERE id = ?")
            .bind(role)
            .bind(id)
            .execute(&self.pool)
            .await?;
        Ok(())
    }

    async fn update_user_password(&self, id: &str, password_hash: &str) -> Result<(), sqlx::Error> {
        sqlx::query("UPDATE users SET password_hash = ? WHERE id = ?")
            .bind(password_hash)
            .bind(id)
            .execute(&self.pool)
            .await?;
        Ok(())
    }

    async fn delete_user(&self, id: &str) -> Result<(), sqlx::Error> {
        // Sessions cascade via the foreign key.
        sqlx::query("DELETE FROM users WHERE id = ?")
            .bind(id)
            .execute(&self.pool)
            .await?;
        Ok(())
    }

    // ── Sessions ────────────────────────────────────────────────────────────────

    async fn create_session(
        &self,
        token: &str,
        user_id: &str,
        expires_at: &str,
    ) -> Result<(), sqlx::Error> {
        sqlx::query(
            "INSERT INTO sessions (token, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)",
        )
        .bind(token)
        .bind(user_id)
        .bind(Utc::now().to_rfc3339())
        .bind(expires_at)
        .execute(&self.pool)
        .await?;
        Ok(())
    }

    async fn get_session(&self, token: &str) -> Result<Option<Session>, sqlx::Error> {
        let row = sqlx::query(
            "SELECT token, user_id, created_at, expires_at FROM sessions WHERE token = ?",
        )
        .bind(token)
        .fetch_optional(&self.pool)
        .await?;
        Ok(row.map(|r| Session {
            token: r.get("token"),
            user_id: r.get("user_id"),
            created_at: r.get("created_at"),
            expires_at: r.get("expires_at"),
        }))
    }

    async fn delete_session(&self, token: &str) -> Result<(), sqlx::Error> {
        sqlx::query("DELETE FROM sessions WHERE token = ?")
            .bind(token)
            .execute(&self.pool)
            .await?;
        Ok(())
    }

    async fn delete_user_sessions(&self, user_id: &str) -> Result<(), sqlx::Error> {
        sqlx::query("DELETE FROM sessions WHERE user_id = ?")
            .bind(user_id)
            .execute(&self.pool)
            .await?;
        Ok(())
    }
}

fn row_to_user(r: &SqliteRow) -> User {
    User {
        id: r.get("id"),
        username: r.get("username"),
        password_hash: r.get("password_hash"),
        role: r.get("role"),
        created_at: r.get("created_at"),
    }
}

fn row_to_env(r: &SqliteRow) -> Environment {
    Environment {
        id: r.get("id"),
        name: r.get("name"),
        docker_host: r.get("docker_host"),
        is_local: r.get::<i64, _>("is_local") != 0,
        created_at: r.get("created_at"),
    }
}

fn row_to_stack(r: &SqliteRow) -> SavedStack {
    SavedStack {
        id: r.get("id"),
        host_id: r.get("host_id"),
        name: r.get("name"),
        content: r.get("content"),
        env_vars: r.get("env_vars"),
        webhook_token: r.get("webhook_token"),
        status: r.get("status"),
        created_at: r.get("created_at"),
        updated_at: r.get("updated_at"),
    }
}
