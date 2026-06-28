use async_trait::async_trait;

use crate::storage::models::{ContainerEvent, Environment, SavedStack};

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

    async fn list_environments(&self) -> Result<Vec<Environment>, sqlx::Error>;

    async fn get_environment(&self, id: &str) -> Result<Option<Environment>, sqlx::Error>;

    async fn create_environment(
        &self,
        name: &str,
        docker_host: &str,
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

    async fn update_stack(&self, id: &str, content: &str) -> Result<(), sqlx::Error>;

    async fn update_stack_status(&self, id: &str, status: &str) -> Result<(), sqlx::Error>;

    async fn delete_stack(&self, id: &str) -> Result<(), sqlx::Error>;
}
