//! Transport to the agent(s).
//!
//! [`AgentClient`] is the **pull** transport: the gateway dials the agent, and
//! each call opens a short-lived connection carrying one framed [`Request`] /
//! [`Response`]. The long-lived event stream ([`AgentClient::watch_events`]) uses
//! its own connection.
//!
//! [`AgentRouter`] sits in front and picks a transport per call: a **push**
//! session (an agent that dialed *into* the gateway, registered by id) when one
//! is available, otherwise the default pull client. Host-agnostic calls (e.g.
//! cluster-wide Kubernetes reads) go straight to the default. This is the seam
//! multi-agent routing and the WSS push server build on.

use std::collections::HashMap;
use std::sync::Arc;

use async_trait::async_trait;
use axum::http::StatusCode;
use tokio::net::TcpStream;
use tokio::sync::RwLock;

use protocol::wire::{Request, Response, read_frame, write_frame};

/// One transport that can execute a request and return the agent's response.
/// Implemented by the pull client today, and by push sessions once agents dial
/// into the gateway.
#[async_trait]
pub trait AgentTransport: Send + Sync {
    async fn call(&self, req: Request) -> Response;
}

/// Routes requests to the right agent transport.
///
/// Holds the default pull client plus a registry of live push sessions keyed by
/// agent id. Exposes the same `call`/`watch_events` surface as [`AgentClient`]
/// so existing call sites route through it unchanged; [`AgentRouter::call_on`]
/// adds per-agent selection for multi-agent routing.
#[derive(Clone)]
pub struct AgentRouter {
    default: Arc<AgentClient>,
    sessions: Arc<RwLock<HashMap<String, Arc<dyn AgentTransport>>>>,
}

impl AgentRouter {
    pub fn new(default: AgentClient) -> Self {
        Self {
            default: Arc::new(default),
            sessions: Arc::new(RwLock::new(HashMap::new())),
        }
    }

    /// Routed call: prefer the named agent's live push session, else fall back to
    /// the default pull client. `agent_id` comes from the target host's record
    /// once host→agent association lands; today it is always `None`.
    pub async fn call_on(&self, agent_id: Option<&str>, req: Request) -> Response {
        if let Some(id) = agent_id
            && let Some(session) = self.sessions.read().await.get(id).cloned()
        {
            return session.call(req).await;
        }
        self.default.call(req).await
    }

    /// Opens the default agent's event stream (used by the watcher).
    pub async fn watch_events(&self, docker_host: String) -> std::io::Result<TcpStream> {
        self.default.watch_events(docker_host).await
    }

    /// Ids of the agents currently connected via a push session.
    pub async fn connected_agent_ids(&self) -> Vec<String> {
        let mut ids: Vec<String> = self.sessions.read().await.keys().cloned().collect();
        ids.sort();
        ids
    }

    /// Registers a live push session under `agent_id`, replacing any previous
    /// session for that id (an agent that reconnects supersedes its stale link).
    pub async fn register(&self, agent_id: String, transport: Arc<dyn AgentTransport>) {
        self.sessions.write().await.insert(agent_id, transport);
    }

    /// Removes the push session for `agent_id` (on disconnect).
    pub async fn unregister(&self, agent_id: &str) {
        self.sessions.write().await.remove(agent_id);
    }
}

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

// ── Push sessions ────────────────────────────────────────────────────────────
// An agent that dials into the gateway (push mode) holds one multiplexed
// WebSocket session. The gateway sends `RequestFrame`s and matches each reply by
// id; the agent also pushes Docker events on `EVENT_STREAM_ID`.

use std::sync::Mutex;
use std::sync::atomic::{AtomicU64, Ordering};
use std::time::Duration;

use axum::extract::ws::{Message, WebSocket};
use futures_util::{SinkExt, StreamExt};
use tokio::sync::{mpsc, oneshot};

use protocol::wire::{EVENT_STREAM_ID, RequestFrame, ResponseFrame};

use crate::state::AppState;

/// How long a routed request waits for its correlated reply before giving up.
const REQUEST_TIMEOUT: Duration = Duration::from_secs(30);

/// Correlates in-flight requests with their replies over one session.
#[derive(Default)]
struct Pending {
    next_id: AtomicU64,
    waiters: Mutex<HashMap<u64, oneshot::Sender<Response>>>,
}

impl Pending {
    /// Allocates a fresh id (≥ 1, so it never collides with [`EVENT_STREAM_ID`])
    /// and returns the receiver that resolves when its reply arrives.
    fn begin(&self) -> (u64, oneshot::Receiver<Response>) {
        let id = self.next_id.fetch_add(1, Ordering::Relaxed) + 1;
        let (tx, rx) = oneshot::channel();
        self.waiters.lock().unwrap().insert(id, tx);
        (id, rx)
    }

    /// Delivers a reply to its waiter, if one is still registered.
    fn complete(&self, id: u64, resp: Response) {
        if let Some(tx) = self.waiters.lock().unwrap().remove(&id) {
            let _ = tx.send(resp);
        }
    }

    /// Drops every waiter; their `call`s resolve to a closed-session error.
    fn fail_all(&self) {
        self.waiters.lock().unwrap().clear();
    }
}

/// The push transport: queues a request onto the session and awaits its reply.
pub struct SessionTransport {
    pending: Arc<Pending>,
    outbound: mpsc::UnboundedSender<RequestFrame>,
}

#[async_trait]
impl AgentTransport for SessionTransport {
    async fn call(&self, req: Request) -> Response {
        let (id, rx) = self.pending.begin();
        if self.outbound.send(RequestFrame { id, req }).is_err() {
            return session_error("agent session closed");
        }
        match tokio::time::timeout(REQUEST_TIMEOUT, rx).await {
            Ok(Ok(resp)) => resp,
            Ok(Err(_)) => session_error("agent session closed"),
            Err(_) => Response::Error {
                status: 504,
                message: "agent request timed out".to_string(),
            },
        }
    }
}

fn session_error(msg: &str) -> Response {
    Response::Error {
        status: 503,
        message: msg.to_string(),
    }
}

/// Drives one push session for its lifetime: registers the transport, pumps
/// requests out and replies/events in, and cleans up on disconnect.
pub async fn run_session(socket: WebSocket, state: AppState, agent_id: String) {
    let pending = Arc::new(Pending::default());
    let (out_tx, mut out_rx) = mpsc::unbounded_channel::<RequestFrame>();

    let transport = Arc::new(SessionTransport {
        pending: pending.clone(),
        outbound: out_tx,
    });
    state.agent.register(agent_id.clone(), transport).await;
    println!("Agent '{agent_id}' connected (push session)");

    let (mut sink, mut stream) = socket.split();

    // Outbound: serialize queued request frames to the socket.
    let writer = tokio::spawn(async move {
        while let Some(frame) = out_rx.recv().await {
            let Ok(json) = serde_json::to_string(&frame) else {
                continue;
            };
            if sink.send(Message::Text(json)).await.is_err() {
                break;
            }
        }
    });

    // Inbound: route replies to their waiters, pushed events to the watcher.
    while let Some(Ok(msg)) = stream.next().await {
        match msg {
            Message::Text(text) => match serde_json::from_str::<ResponseFrame>(&text) {
                Ok(frame) if frame.id == EVENT_STREAM_ID => {
                    if let Response::Event(event) = frame.resp {
                        crate::watcher::handle_event(&state, event).await;
                    }
                }
                Ok(frame) => pending.complete(frame.id, frame.resp),
                Err(e) => eprintln!("Malformed frame from agent '{agent_id}': {e}"),
            },
            Message::Close(_) => break,
            _ => {}
        }
    }

    // Teardown: stop routing to this agent and unblock any pending calls.
    state.agent.unregister(&agent_id).await;
    pending.fail_all();
    writer.abort();
    println!("Agent '{agent_id}' disconnected");
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn pending_correlates_reply_to_waiter() {
        let pending = Pending::default();
        let (id1, rx1) = pending.begin();
        let (id2, rx2) = pending.begin();
        assert_ne!(id1, id2);
        assert!(
            id1 >= 1 && id2 >= 1,
            "ids must not collide with EVENT_STREAM_ID"
        );

        pending.complete(id2, Response::Pong);
        pending.complete(id1, Response::Ok);

        assert!(matches!(rx1.await, Ok(Response::Ok)));
        assert!(matches!(rx2.await, Ok(Response::Pong)));
    }

    #[tokio::test]
    async fn fail_all_closes_pending_calls() {
        let pending = Pending::default();
        let (_id, rx) = pending.begin();
        pending.fail_all();
        assert!(
            rx.await.is_err(),
            "waiter should observe the dropped sender"
        );
    }

    #[test]
    fn unknown_completion_is_ignored() {
        let pending = Pending::default();
        pending.complete(999, Response::Ok); // no waiter — must not panic
    }
}
