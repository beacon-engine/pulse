//! End-to-end tests for the **push** transport over a real WebSocket.
//!
//! These drive the actual `/agent/ws` axum route (auth + upgrade + [`run_session`])
//! on a loopback listener, with a real WS client ([`protocol::transport::connect`])
//! standing in for a dialing agent. Unlike the `Pending` unit tests in
//! [`crate::agent`], the request/reply and event frames here cross a live socket,
//! so they exercise upgrade auth, session registration, id correlation over the
//! wire, event delivery on the reserved id, and disconnect teardown together.
//!
//! This lives in-crate rather than under `tests/` because `gateway` is a binary
//! (no lib target), so `run_session`/`AgentRouter` are not reachable as an
//! external API. The client half of the transport comes from the `protocol`
//! crate, matching what the real agent uses in `dial` mode.

use std::collections::HashMap;
use std::net::SocketAddr;
use std::sync::Arc;
use std::time::Duration;

use axum::{Router, routing::get};
use futures_util::{SinkExt, StreamExt};
use tokio::sync::mpsc;
use tokio::time::timeout;

use protocol::transport::{Message, TlsMode, connect};
use protocol::wire::{EVENT_STREAM_ID, EventFrame, Request, RequestFrame, Response, ResponseFrame};

use crate::agent::{AgentClient, AgentRouter};
use crate::observability::websocket;
use crate::state::AppState;

/// Spins up a gateway exposing only `/agent/ws` on an ephemeral loopback port,
/// with the given `agent_id -> token` pairs accepted for push auth. Returns the
/// bound address and the shared state (so tests can call the router and observe
/// the session registry / event bus).
async fn spawn_gateway(tokens: &[(&str, &str)]) -> (SocketAddr, AppState) {
    let db = crate::storage::init("sqlite::memory:", false)
        .await
        .expect("in-memory db init");

    let mut token_map = HashMap::new();
    for (agent_id, token) in tokens {
        token_map.insert((*token).to_string(), (*agent_id).to_string());
    }

    let state = AppState {
        // The default pull client points at a dead port; push tests never hit it,
        // and the disconnect test relies on that fallback failing.
        agent: AgentRouter::new(AgentClient::new("127.0.0.1:1")),
        agent_tokens: Arc::new(token_map),
        db,
        events: Arc::new(websocket::new_channel()),
        slack_webhook: None,
    };

    let app = Router::new()
        .route("/agent/ws", get(crate::api::agent_ws))
        .with_state(state.clone());

    let listener = tokio::net::TcpListener::bind("127.0.0.1:0")
        .await
        .expect("bind loopback");
    let addr = listener.local_addr().expect("local addr");
    tokio::spawn(async move {
        let _ = axum::serve(listener, app).await;
    });

    (addr, state)
}

/// Polls until `agent_id` is (or, if `present` is false, is no longer) in the
/// live-session registry, or panics after ~2s.
async fn wait_registered(state: &AppState, agent_id: &str, present: bool) {
    for _ in 0..100 {
        let connected = state
            .agent
            .connected_agent_ids()
            .await
            .iter()
            .any(|id| id == agent_id);
        if connected == present {
            return;
        }
        tokio::time::sleep(Duration::from_millis(20)).await;
    }
    panic!("agent '{agent_id}' registration != {present} within timeout");
}

/// Splits a client socket into a request-serving loop that mimics the agent's
/// `dispatch`: it replies to each [`RequestFrame`] with the matching canned
/// [`Response`], echoing the correlation id. `Ping` replies are deliberately
/// **delayed** so a later, faster request answers first — proving the gateway
/// correlates by id, not by arrival order. Replies go through an mpsc so delayed
/// ones don't block reading the next request.
fn spawn_fake_agent(ws: protocol::transport::WsStream) {
    let (mut sink, mut stream) = ws.split();
    let (tx, mut rx) = mpsc::unbounded_channel::<String>();

    // Writer: drain queued replies onto the socket.
    tokio::spawn(async move {
        while let Some(json) = rx.recv().await {
            if sink.send(Message::Text(json)).await.is_err() {
                break;
            }
        }
    });

    // Reader: turn each request into a (delayed) reply on the same id.
    tokio::spawn(async move {
        while let Some(Ok(Message::Text(text))) = stream.next().await {
            let Ok(frame) = serde_json::from_str::<RequestFrame>(&text) else {
                continue;
            };
            let (delay, resp) = match frame.req {
                Request::Ping => (Duration::from_millis(150), Response::Pong),
                Request::K8sRollout { .. } => (Duration::ZERO, Response::Ok),
                _ => (Duration::ZERO, Response::Ok),
            };
            let tx = tx.clone();
            tokio::spawn(async move {
                if !delay.is_zero() {
                    tokio::time::sleep(delay).await;
                }
                let reply = ResponseFrame { id: frame.id, resp };
                let _ = tx.send(serde_json::to_string(&reply).unwrap());
            });
        }
    });
}

fn ws_url(addr: SocketAddr) -> String {
    format!("ws://{addr}/agent/ws")
}

#[tokio::test]
async fn upgrade_rejects_missing_or_bad_token() {
    let (addr, state) = spawn_gateway(&[("prod", "s3cret")]).await;
    let url = ws_url(addr);

    // No token → 401, no session.
    assert!(
        connect(&url, TlsMode::Verify, None).await.is_err(),
        "unauthenticated upgrade must be refused"
    );
    // Wrong token → 401, no session.
    assert!(
        connect(&url, TlsMode::Verify, Some("nope")).await.is_err(),
        "invalid token must be refused"
    );

    // Neither attempt should have registered a session.
    assert!(state.agent.connected_agent_ids().await.is_empty());
}

#[tokio::test]
async fn valid_token_registers_session_and_correlates_replies() {
    let (addr, state) = spawn_gateway(&[("prod", "s3cret")]).await;

    let ws = connect(&ws_url(addr), TlsMode::Verify, Some("s3cret"))
        .await
        .expect("authenticated upgrade succeeds");
    spawn_fake_agent(ws);
    wait_registered(&state, "prod", true).await;

    // Fire two requests concurrently. `Ping` (→ Pong) is delayed in the fake
    // agent, so `K8sRollout` (→ Ok) answers first; each call must still resolve
    // to *its own* reply, which only holds if ids are matched to waiters.
    let (ping, rollout) = tokio::join!(
        state.agent.call_on(Some("prod"), Request::Ping),
        state.agent.call_on(
            Some("prod"),
            Request::K8sRollout {
                namespace: "ns".into(),
                deployment: "api".into(),
            },
        ),
    );

    assert!(
        matches!(ping, Response::Pong),
        "Ping call got the wrong reply: {ping:?}"
    );
    assert!(
        matches!(rollout, Response::Ok),
        "K8sRollout call got the wrong reply: {rollout:?}"
    );
}

#[tokio::test]
async fn events_on_reserved_id_reach_the_watcher() {
    let (addr, state) = spawn_gateway(&[("prod", "s3cret")]).await;

    let ws = connect(&ws_url(addr), TlsMode::Verify, Some("s3cret"))
        .await
        .expect("authenticated upgrade succeeds");
    let (mut sink, _stream) = ws.split();
    wait_registered(&state, "prod", true).await;

    // Subscribe to the gateway's client-facing event bus before pushing, so we
    // observe the broadcast the watcher emits for this event.
    let mut bus = state.events.subscribe();

    let event = ResponseFrame {
        id: EVENT_STREAM_ID,
        resp: Response::Event(EventFrame {
            container_id: "c1".into(),
            container_name: "web".into(),
            action: "die".into(),
            exit_code: Some("1".into()),
        }),
    };
    sink.send(Message::Text(serde_json::to_string(&event).unwrap()))
        .await
        .expect("push event frame");

    let broadcast = timeout(Duration::from_secs(2), bus.recv())
        .await
        .expect("watcher broadcast within timeout")
        .expect("event bus not closed");

    assert!(
        broadcast.contains("crash") && broadcast.contains("web"),
        "unexpected broadcast payload: {broadcast}"
    );
}

#[tokio::test]
async fn disconnect_unregisters_the_session() {
    let (addr, state) = spawn_gateway(&[("prod", "s3cret")]).await;

    let ws = connect(&ws_url(addr), TlsMode::Verify, Some("s3cret"))
        .await
        .expect("authenticated upgrade succeeds");
    let (sink, stream) = ws.split();
    wait_registered(&state, "prod", true).await;

    // Close the client socket. The gateway's read loop sees EOF and runs teardown.
    drop(sink);
    drop(stream);

    // Teardown removes the push session from the registry.
    wait_registered(&state, "prod", false).await;
    assert!(state.agent.connected_agent_ids().await.is_empty());

    // With the session gone, a routed call no longer reaches it: it falls back to
    // the default pull client (a dead port here), so it fails fast rather than
    // hanging on a stale session — the caller always gets a `Response`.
    let resp = timeout(
        Duration::from_secs(5),
        state.agent.call_on(Some("prod"), Request::Ping),
    )
    .await
    .expect("routed call resolves promptly after disconnect");
    assert!(
        matches!(resp, Response::Error { status: 502, .. }),
        "expected 502 fallback after session teardown, got {resp:?}"
    );
}
