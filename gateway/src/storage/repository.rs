use async_trait::async_trait;

use crate::storage::models::{ContainerEvent, Environment, SavedStack, Session, User};

/// Implement this trait to swap the database backend.
/// The rest of the app depends only on this interface.
#[async_trait]
pub trait Repository: Send + Sync {
    async fn record_event(
        &self,
        container_id: &str,
        container_name: &str,
        event: &str,
    ) -> Result<(), sqlx::Error>;

    async fn get_recent_events(&self, limit: i64) -> Result<Vec<ContainerEvent>, sqlx::Error>;

    // async fn list_environments(&self) -> Result<Vec<Environment>, sqlx::Error>;

    /// A page of hosts, optionally filtered by a name search (case-insensitive).
    async fn list_environments_page(
        &self,
        search: Option<&str>,
        limit: i64,
        offset: i64,
    ) -> Result<Vec<Environment>, sqlx::Error>;

    /// Total host count matching an optional name search.
    async fn count_environments(&self, search: Option<&str>) -> Result<i64, sqlx::Error>;

    async fn get_environment(&self, id: &str) -> Result<Option<Environment>, sqlx::Error>;

    async fn create_environment(
        &self,
        name: &str,
        docker_host: &str,
        agent_id: Option<&str>,
    ) -> Result<String, sqlx::Error>;

    async fn rename_environment(&self, id: &str, name: &str) -> Result<(), sqlx::Error>;

    async fn delete_environment(&self, id: &str) -> Result<(), sqlx::Error>;

    async fn list_stacks(&self, host_id: &str) -> Result<Vec<SavedStack>, sqlx::Error>;

    async fn get_stack(&self, id: &str) -> Result<Option<SavedStack>, sqlx::Error>;

    async fn create_stack(
        &self,
        host_id: &str,
        name: &str,
        content: &str,
    ) -> Result<String, sqlx::Error>;

    async fn update_stack(
        &self,
        id: &str,
        content: &str,
        env_vars: Option<&str>,
    ) -> Result<(), sqlx::Error>;

    async fn update_stack_status(&self, id: &str, status: &str) -> Result<(), sqlx::Error>;

    async fn set_stack_webhook_token(
        &self,
        id: &str,
        token: Option<&str>,
    ) -> Result<(), sqlx::Error>;

    async fn delete_stack(&self, id: &str) -> Result<(), sqlx::Error>;

    // ── Generic webhook token management ──────────────────────────────────────
    async fn get_webhook_token(
        &self,
        entity_type: &str,
        entity_key: &str,
        host_id: &str,
    ) -> Result<Option<String>, sqlx::Error>;

    async fn set_webhook_token(
        &self,
        entity_type: &str,
        entity_key: &str,
        host_id: &str,
        token: &str,
    ) -> Result<(), sqlx::Error>;

    async fn delete_webhook_token(
        &self,
        entity_type: &str,
        entity_key: &str,
        host_id: &str,
    ) -> Result<(), sqlx::Error>;

    // ── Users ─────────────────────────────────────────────────────────────────
    async fn count_users(&self) -> Result<i64, sqlx::Error>;

    async fn count_admins(&self) -> Result<i64, sqlx::Error>;

    async fn create_user(
        &self,
        username: &str,
        password_hash: &str,
        role: &str,
    ) -> Result<String, sqlx::Error>;

    async fn get_user(&self, id: &str) -> Result<Option<User>, sqlx::Error>;

    async fn get_user_by_username(&self, username: &str) -> Result<Option<User>, sqlx::Error>;

    async fn list_users(&self) -> Result<Vec<User>, sqlx::Error>;

    async fn update_user_role(&self, id: &str, role: &str) -> Result<(), sqlx::Error>;

    async fn update_user_password(&self, id: &str, password_hash: &str) -> Result<(), sqlx::Error>;

    async fn delete_user(&self, id: &str) -> Result<(), sqlx::Error>;

    // ── Sessions ────────────────────────────────────────────────────────────────
    async fn create_session(
        &self,
        token: &str,
        user_id: &str,
        expires_at: &str,
    ) -> Result<(), sqlx::Error>;

    async fn get_session(&self, token: &str) -> Result<Option<Session>, sqlx::Error>;

    async fn delete_session(&self, token: &str) -> Result<(), sqlx::Error>;

    async fn delete_user_sessions(&self, user_id: &str) -> Result<(), sqlx::Error>;
}
