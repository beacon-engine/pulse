//! Authentication primitives: password hashing, session tokens, the session
//! cookie, and the `require_auth` middleware that gates protected routes.

use argon2::password_hash::rand_core::{OsRng, RngCore};
use argon2::password_hash::{PasswordHash, PasswordHasher, PasswordVerifier, SaltString};
use argon2::Argon2;
use axum::{
    extract::{Request, State},
    http::{Method, StatusCode},
    middleware::Next,
    response::Response,
    Extension,
};
use axum_extra::extract::cookie::{Cookie, CookieJar, SameSite};
use chrono::{Duration, Utc};

use crate::state::AppState;

pub const SESSION_COOKIE: &str = "pulse_session";
pub const SESSION_TTL_DAYS: i64 = 7;

/// The authenticated user, injected into request extensions by [`require_auth`]
/// and read by handlers via `Extension<AuthUser>`.
#[derive(Debug, Clone)]
pub struct AuthUser {
    pub id: String,
    pub username: String,
    pub role: String,
}

impl AuthUser {
    pub fn is_admin(&self) -> bool {
        self.role == "admin"
    }
}

/// Rejects the request unless the caller has the `admin` role.
pub fn require_admin(user: &AuthUser) -> Result<(), (StatusCode, String)> {
    if user.is_admin() {
        Ok(())
    } else {
        Err((StatusCode::FORBIDDEN, "admin role required".into()))
    }
}

pub fn hash_password(password: &str) -> Result<String, String> {
    let salt = SaltString::generate(&mut OsRng);
    Argon2::default()
        .hash_password(password.as_bytes(), &salt)
        .map(|h| h.to_string())
        .map_err(|e| e.to_string())
}

pub fn verify_password(password: &str, hash: &str) -> bool {
    match PasswordHash::new(hash) {
        Ok(parsed) => Argon2::default()
            .verify_password(password.as_bytes(), &parsed)
            .is_ok(),
        Err(_) => false,
    }
}

/// Generates a 256-bit random session token, hex-encoded.
pub fn generate_token() -> String {
    let mut bytes = [0u8; 32];
    OsRng.fill_bytes(&mut bytes);
    bytes.iter().map(|b| format!("{b:02x}")).collect()
}

/// The RFC3339 expiry timestamp for a newly created session.
pub fn session_expiry() -> String {
    (Utc::now() + Duration::days(SESSION_TTL_DAYS)).to_rfc3339()
}

/// Builds the session cookie. Not marked `Secure` so it works over plain HTTP in
/// local/dev; put the gateway behind TLS (or a TLS-terminating proxy) in prod.
pub fn session_cookie(token: String) -> Cookie<'static> {
    let mut cookie = Cookie::new(SESSION_COOKIE, token);
    cookie.set_http_only(true);
    cookie.set_same_site(SameSite::Lax);
    cookie.set_path("/");
    // cookie.set_secure(false);
    cookie.set_max_age(time::Duration::days(SESSION_TTL_DAYS));
    cookie
}

/// A cookie that clears the session on the client.
pub fn clear_cookie() -> Cookie<'static> {
    let mut cookie = Cookie::new(SESSION_COOKIE, "");
    cookie.set_http_only(true);
    // cookie.set_secure(false); // This is for TLS. Commented out for local development
    cookie.set_same_site(SameSite::Lax);
    cookie.set_path("/");
    cookie.set_max_age(time::Duration::seconds(0));
    cookie
}

/// Middleware for protected routes: validates the session cookie, loads the
/// user, and injects [`AuthUser`] into request extensions. Returns 401 when the
/// caller is not authenticated.
pub async fn require_auth(
    State(state): State<AppState>,
    jar: CookieJar,
    mut req: Request,
    next: Next,
) -> Result<Response, (StatusCode, String)> {
    let token = match jar.get(SESSION_COOKIE) {
        Some(c) => c.value().to_string(),
        None => return Err((StatusCode::UNAUTHORIZED, "not authenticated".into())),
    };

    let session = state
        .db
        .get_session(&token)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?
        .ok_or((StatusCode::UNAUTHORIZED, "invalid session".to_string()))?;

    if let Ok(exp) = chrono::DateTime::parse_from_rfc3339(&session.expires_at) {
        if exp.with_timezone(&Utc) < Utc::now() {
            let _ = state.db.delete_session(&token).await;
            return Err((StatusCode::UNAUTHORIZED, "session expired".into()));
        }
    }

    let user = state
        .db
        .get_user(&session.user_id)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?
        .ok_or((StatusCode::UNAUTHORIZED, "user no longer exists".to_string()))?;

    req.extensions_mut().insert(AuthUser {
        id: user.id,
        username: user.username,
        role: user.role,
    });

    Ok(next.run(req).await)
}

/// Runs after [`require_auth`]. Viewers may only make safe (read) requests;
/// any state-changing request requires the `admin` role. Logout is always
/// allowed so a viewer can end their own session.
pub async fn enforce_role(
    Extension(user): Extension<AuthUser>,
    req: Request,
    next: Next,
) -> Result<Response, (StatusCode, String)> {
    let safe = matches!(*req.method(), Method::GET | Method::HEAD | Method::OPTIONS);
    let always_allowed = req.uri().path() == "/api/auth/logout";

    if !safe && !always_allowed && !user.is_admin() {
        return Err((
            StatusCode::FORBIDDEN,
            "read-only account: this action requires an admin".into(),
        ));
    }

    Ok(next.run(req).await)
}
