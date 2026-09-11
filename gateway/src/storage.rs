pub mod models;
pub mod repository;
pub mod sqlite;

pub use repository::Repository;

use sqlx::migrate::{MigrateDatabase, MigrateError};
use sqlx::sqlite::{SqliteConnectOptions, SqlitePool, SqlitePoolOptions};
use std::str::FromStr;
use std::sync::Arc;

pub async fn init(
    database_url: &str,
    reset_on_mismatch: bool,
) -> Result<Arc<dyn Repository>, sqlx::Error> {
    let pool = connect(database_url).await?;

    match sqlx::migrate!("./migrations").run(&pool).await {
        Ok(()) => Ok(Arc::new(sqlite::SqliteRepository::new(pool))),

        // A migration-integrity failure: either an already-applied migration was
        // edited in place (`VersionMismatch`) or the database has a migration
        // this binary doesn't (`VersionMissing`). For a throwaway dev database,
        // `reset_on_mismatch` (from `PULSE_DB_RESET_ON_MISMATCH`, read in config)
        // recreates it instead of forcing a manual wipe. Production leaves it
        // false, so the error propagates.
        Err(e @ (MigrateError::VersionMismatch(_) | MigrateError::VersionMissing(_)))
            if reset_on_mismatch =>
        {
            eprintln!(
                "⚠️  Database migration mismatch ({e}); PULSE_DB_RESET_ON_MISMATCH is set — \
                 recreating the database from scratch. ALL DATA WILL BE LOST."
            );
            pool.close().await;
            reset_database(database_url).await?;

            let pool = connect(database_url).await?;
            sqlx::migrate!("./migrations").run(&pool).await?;
            Ok(Arc::new(sqlite::SqliteRepository::new(pool)))
        }

        Err(e) => Err(e.into()),
    }
}

/// Opens the connection pool, creating the database file if it does not exist.
async fn connect(database_url: &str) -> Result<SqlitePool, sqlx::Error> {
    let options = SqliteConnectOptions::from_str(database_url)?.create_if_missing(true);
    SqlitePoolOptions::new()
        .max_connections(5)
        .connect_with(options)
        .await
}

/// Drops and recreates the database at `database_url` (dev reset path). Destroys
/// all data — only reached when `PULSE_DB_RESET_ON_MISMATCH` is truthy.
async fn reset_database(database_url: &str) -> Result<(), sqlx::Error> {
    if sqlx::Sqlite::database_exists(database_url).await.unwrap_or(false) {
        sqlx::Sqlite::drop_database(database_url).await?;
    }
    sqlx::Sqlite::create_database(database_url).await
}

// To switch to PostgreSQL:
// 1. Change sqlx features in Cargo.toml: replace "sqlite" with "postgres"
// 2. Write src/storage/postgres.rs implementing Repository
// 3. Replace SqliteRepository::new above with PostgresRepository::new
// Nothing else in the codebase changes.
