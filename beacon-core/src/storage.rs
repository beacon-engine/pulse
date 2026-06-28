pub mod models;
pub mod repository;
pub mod sqlite;

pub use repository::Repository;

use sqlx::sqlite::{SqliteConnectOptions, SqlitePoolOptions};
use std::str::FromStr;
use std::sync::Arc;

pub async fn init(database_url: &str) -> Result<Arc<dyn Repository>, sqlx::Error> {
    let options = SqliteConnectOptions::from_str(database_url)?.create_if_missing(true);

    let pool = SqlitePoolOptions::new()
        .max_connections(5)
        .connect_with(options)
        .await?;

    sqlx::migrate!("./migrations").run(&pool).await?;

    Ok(Arc::new(sqlite::SqliteRepository::new(pool)))
}

// To switch to PostgreSQL:
// 1. Change sqlx features in Cargo.toml: replace "sqlite" with "postgres"
// 2. Write src/storage/postgres.rs implementing Repository
// 3. Replace SqliteRepository::new above with PostgresRepository::new
// Nothing else in the codebase changes.
