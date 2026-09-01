//! TCP client to the agent.
//!
//! Each call opens a short-lived connection, writes one framed [`Request`] and
//! reads one framed [`Response`]. Connections are cheap on localhost and this
//! keeps request handling free of shared-connection interleaving. The long-lived
//! event stream ([`AgentClient::watch_events`]) uses its own connection.

use axum::http::StatusCode;
use tokio::net::TcpStream;

use protocol::wire::{Request, Response, read_frame, write_frame};

#[derive(Clone)]
pub struct AgentClient {
    addr: String,
}

impl AgentClient {
    pub fn new(addr: impl Into<String>) -> Self {
        Self { addr: addr.into() }
    }

    /// Sends a request and returns the agent's response. Transport failures are
    /// surfaced as [`Response::Error`] with a `502` status so callers only ever
    /// match on `Response`.
    pub async fn call(&self, req: Request) -> Response {
        match self.try_call(req).await {
            Ok(resp) => resp,
            Err(e) => Response::Error {
                status: 502,
                message: format!("agent unreachable: {e}"),
            },
        }
    }

    async fn try_call(&self, req: Request) -> std::io::Result<Response> {
        let mut stream = TcpStream::connect(&self.addr).await?;
        let (mut reader, mut writer) = stream.split();
        write_frame(&mut writer, &req).await?;
        read_frame(&mut reader).await
    }

    /// Opens an event-stream connection. The caller reads [`Response::Event`]
    /// frames from the returned stream until it closes.
    pub async fn watch_events(&self, docker_host: String) -> std::io::Result<TcpStream> {
        let mut stream = TcpStream::connect(&self.addr).await?;
        write_frame(&mut stream, &Request::WatchEvents { docker_host }).await?;
        Ok(stream)
    }
}

/// Maps a wire status code onto an axum [`StatusCode`], defaulting to 500.
pub fn to_status(code: u16) -> StatusCode {
    StatusCode::from_u16(code).unwrap_or(StatusCode::INTERNAL_SERVER_ERROR)
}

/// Turns an unexpected response variant into a 500 error tuple.
pub fn unexpected(resp: Response) -> (StatusCode, String) {
    match resp {
        Response::Error { status, message } => (to_status(status), message),
        other => (
            StatusCode::INTERNAL_SERVER_ERROR,
            format!("unexpected agent response: {other:?}"),
        ),
    }
}
