//! Push (dial) uplink.
//!
//! When the agent runs in `Dial` mode it connects *out* to the gateway over WSS
//! and serves requests over that single session — the exact same [`dispatch`]
//! used by the TCP listener, just a different transport. This lets an agent that
//! can only make outbound connections (443/WSS) still be driven by the gateway.
//! The socket is bidirectional, so the gateway sends [`RequestFrame`]s down it
//! and the agent replies with [`ResponseFrame`]s carrying the same id.

use std::sync::Arc;
use std::time::Duration;

use futures_util::{SinkExt, StreamExt};

use protocol::transport::{connect, Message, TlsMode, WsStream};
use protocol::wire::{RequestFrame, ResponseFrame};

use crate::{dispatch, AppState};

/// Delay between reconnect attempts after the session drops or fails to open.
const RECONNECT_DELAY: Duration = Duration::from_secs(5);

/// Runs the uplink forever: (re)connect to `url`, serve until the session drops,
/// then back off and retry. `tls` is the server-certificate trust policy for the
/// `wss://` connection, resolved from config by the caller.
pub async fn run(state: Arc<AppState>, url: String, token: Option<String>, tls: TlsMode) {
    loop {
        match connect(&url, tls, token.as_deref()).await {
            Ok(ws) => {
                println!("Uplink connected to gateway {url}");
                serve(ws, &state).await;
                eprintln!(
                    "Uplink to {url} closed; reconnecting in {}s",
                    RECONNECT_DELAY.as_secs()
                );
            }
            Err(e) => eprintln!(
                "Uplink connect to {url} failed: {e}; retrying in {}s",
                RECONNECT_DELAY.as_secs()
            ),
        }
        tokio::time::sleep(RECONNECT_DELAY).await;
    }
}

/// Serves requests over one live session until it closes or errors.
async fn serve(ws: WsStream, state: &Arc<AppState>) {
    let (mut sink, mut stream) = ws.split();

    while let Some(msg) = stream.next().await {
        let msg = match msg {
            Ok(m) => m,
            Err(e) => {
                eprintln!("uplink read error: {e}");
                break;
            }
        };

        match msg {
            Message::Text(text) => {
                let frame: RequestFrame = match serde_json::from_str(&text) {
                    Ok(f) => f,
                    Err(e) => {
                        eprintln!("uplink: malformed request frame: {e}");
                        continue;
                    }
                };
                let resp = dispatch(state, frame.req).await;
                let reply = ResponseFrame { id: frame.id, resp };
                match serde_json::to_string(&reply) {
                    Ok(json) => {
                        if sink.send(Message::Text(json)).await.is_err() {
                            break;
                        }
                    }
                    Err(e) => eprintln!("uplink: failed to encode response: {e}"),
                }
            }
            // Answer keepalive pings so the gateway can detect a live session.
            Message::Ping(payload) => {
                let _ = sink.send(Message::Pong(payload)).await;
            }
            Message::Close(_) => break,
            _ => {}
        }
    }
}
