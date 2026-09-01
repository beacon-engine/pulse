//! First-run setup, login, logout and "who am I" endpoints.

use axum::{Extension, Json, extract::State, http::StatusCode};
use axum_extra::extract::CookieJar;
use serde::Deserialize;
use serde_json::{Value, json};

use crate::auth::{
    AuthUser, SESSION_COOKIE, clear_cookie, generate_token, hash_password, session_cookie,
    session_expiry, verify_password,
};
use crate::state::AppState;

type ApiError = (StatusCode, String);

#[derive(Deserialize)]
pub struct Credentials {
    pub username: String,
    pub password: String,
}

fn validate(username: &str, password: &str) -> Result<(), ApiError> {
    if username.trim().is_empty() {
        return Err((StatusCode::BAD_REQUEST, "username is required".into()));
    }
    if password.len() < 8 {
        return Err((
            StatusCode::BAD_REQUEST,
            "password must be at least 8 characters".into(),
        ));
    }
    Ok(())
}

async fn issue_session(
    state: &AppState,
    jar: CookieJar,
    user_id: &str,
) -> Result<CookieJar, ApiError> {
    let token = generate_token();
    state
        .db
        .create_session(&token, user_id, &session_expiry())
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
    Ok(jar.add(session_cookie(token)))
}

/// Whether the gateway still needs its first (admin) account created.
pub async fn setup_status(State(state): State<AppState>) -> Result<Json<Value>, ApiError> {
    let count = state
        .db
        .count_users()
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
    Ok(Json(json!({ "needs_setup": count == 0 })))
}

/// Creates the first user (an admin). Only works while no users exist.
pub async fn setup(
    State(state): State<AppState>,
    jar: CookieJar,
    Json(body): Json<Credentials>,
) -> Result<(CookieJar, Json<Value>), ApiError> {
    let count = state
        .db
        .count_users()
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
    if count > 0 {
        return Err((StatusCode::CONFLICT, "setup already completed".into()));
    }

    validate(&body.username, &body.password)?;

    let hash = hash_password(&body.password).map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e))?;
    let id = state
        .db
        .create_user(&body.username, &hash, "admin")
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;

    let jar = issue_session(&state, jar, &id).await?;
    Ok((
        jar,
        Json(json!({ "id": id, "username": body.username, "role": "admin" })),
    ))
}

pub async fn login(
    State(state): State<AppState>,
    jar: CookieJar,
    Json(body): Json<Credentials>,
) -> Result<(CookieJar, Json<Value>), ApiError> {
    let user = state
        .db
        .get_user_by_username(&body.username)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;

    // Verify against the stored hash; run verification even on a missing user is
    // not necessary here since usernames aren't secret, but avoid leaking which
    // half failed.
    let user = match user {
        Some(u) if verify_password(&body.password, &u.password_hash) => u,
        _ => {
            return Err((
                StatusCode::UNAUTHORIZED,
                "invalid username or password".into(),
            ));
        }
    };

    let jar = issue_session(&state, jar, &user.id).await?;
    Ok((
        jar,
        Json(json!({ "id": user.id, "username": user.username, "role": user.role })),
    ))
}

pub async fn logout(
    State(state): State<AppState>,
    jar: CookieJar,
) -> Result<(CookieJar, StatusCode), ApiError> {
    if let Some(c) = jar.get(SESSION_COOKIE) {
        let _ = state.db.delete_session(c.value()).await;
    }
    Ok((jar.add(clear_cookie()), StatusCode::NO_CONTENT))
}

pub async fn me(Extension(user): Extension<AuthUser>) -> Json<Value> {
    Json(json!({ "id": user.id, "username": user.username, "role": user.role }))
}
