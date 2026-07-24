//! User management (admin only). The first user is created via `/api/auth/setup`;
//! everything here requires an authenticated admin.

use axum::{
    extract::{Path, State},
    http::StatusCode,
    Extension, Json,
};
use serde::Deserialize;

use crate::auth::{hash_password, require_admin, AuthUser};
use crate::state::AppState;
use crate::storage::models::User;

type ApiError = (StatusCode, String);

const ROLES: [&str; 2] = ["admin", "viewer"];

fn valid_role(role: &str) -> bool {
    ROLES.contains(&role)
}

pub async fn list(
    State(state): State<AppState>,
    Extension(user): Extension<AuthUser>,
) -> Result<Json<Vec<User>>, ApiError> {
    require_admin(&user)?;
    state
        .db
        .list_users()
        .await
        .map(Json)
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))
}

#[derive(Deserialize)]
pub struct CreateUserBody {
    pub username: String,
    pub password: String,
    pub role: String,
}

pub async fn create(
    State(state): State<AppState>,
    Extension(user): Extension<AuthUser>,
    Json(body): Json<CreateUserBody>,
) -> Result<(StatusCode, Json<User>), ApiError> {
    require_admin(&user)?;

    if body.username.trim().is_empty() {
        return Err((StatusCode::BAD_REQUEST, "username is required".into()));
    }
    if body.password.len() < 8 {
        return Err((StatusCode::BAD_REQUEST, "password must be at least 8 characters".into()));
    }
    if !valid_role(&body.role) {
        return Err((StatusCode::BAD_REQUEST, "role must be 'admin' or 'viewer'".into()));
    }

    if state
        .db
        .get_user_by_username(&body.username)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?
        .is_some()
    {
        return Err((StatusCode::CONFLICT, "username already taken".into()));
    }

    let hash = hash_password(&body.password).map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e))?;
    let id = state
        .db
        .create_user(&body.username, &hash, &body.role)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;

    let created = state
        .db
        .get_user(&id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?
        .ok_or((StatusCode::INTERNAL_SERVER_ERROR, "not found after insert".to_string()))?;

    Ok((StatusCode::CREATED, Json(created)))
}

#[derive(Deserialize)]
pub struct UpdateUserBody {
    pub role: Option<String>,
    pub password: Option<String>,
}

pub async fn update(
    State(state): State<AppState>,
    Extension(user): Extension<AuthUser>,
    Path(id): Path<String>,
    Json(body): Json<UpdateUserBody>,
) -> Result<Json<User>, ApiError> {
    require_admin(&user)?;

    let target = state
        .db
        .get_user(&id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?
        .ok_or((StatusCode::NOT_FOUND, format!("user {id} not found")))?;

    if let Some(role) = &body.role {
        if !valid_role(role) {
            return Err((StatusCode::BAD_REQUEST, "role must be 'admin' or 'viewer'".into()));
        }
        // Don't allow demoting the last remaining admin.
        if target.role == "admin" && role != "admin" {
            let admins = state
                .db
                .count_admins()
                .await
                .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
            if admins <= 1 {
                return Err((StatusCode::BAD_REQUEST, "cannot demote the last admin".into()));
            }
        }
        state
            .db
            .update_user_role(&id, role)
            .await
            .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
    }

    if let Some(password) = &body.password {
        if password.len() < 8 {
            return Err((StatusCode::BAD_REQUEST, "password must be at least 8 characters".into()));
        }
        let hash = hash_password(password).map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e))?;
        state
            .db
            .update_user_password(&id, &hash)
            .await
            .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
        // Changing a password invalidates existing sessions for that user.
        let _ = state.db.delete_user_sessions(&id).await;
    }

    state
        .db
        .get_user(&id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?
        .ok_or((StatusCode::NOT_FOUND, format!("user {id} not found")))
        .map(Json)
}

pub async fn delete(
    State(state): State<AppState>,
    Extension(user): Extension<AuthUser>,
    Path(id): Path<String>,
) -> Result<StatusCode, ApiError> {
    require_admin(&user)?;

    if id == user.id {
        return Err((StatusCode::BAD_REQUEST, "you cannot delete your own account".into()));
    }

    let target = state
        .db
        .get_user(&id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?
        .ok_or((StatusCode::NOT_FOUND, format!("user {id} not found")))?;

    if target.role == "admin" {
        let admins = state
            .db
            .count_admins()
            .await
            .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
        if admins <= 1 {
            return Err((StatusCode::BAD_REQUEST, "cannot delete the last admin".into()));
        }
    }

    state
        .db
        .delete_user(&id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;

    Ok(StatusCode::NO_CONTENT)
}
