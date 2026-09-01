use axum::extract::ws::{Message, WebSocket};
use futures_util::{SinkExt, StreamExt};
use std::sync::Arc;
use tokio::sync::broadcast;
use tokio::sync::broadcast::error::RecvError;

pub type EventSender = broadcast::Sender<String>;

pub fn new_channel() -> EventSender {
    broadcast::channel::<String>(64).0
}

pub async fn handle(socket: WebSocket, tx: Arc<EventSender>) {
    let mut rx = tx.subscribe();
    let (mut sender, mut receiver) = socket.split();

    let mut send_task = tokio::spawn(async move {
        loop {
            match rx.recv().await {
                Ok(msg) => {
                    if sender.send(Message::Text(msg.into())).await.is_err() {
                        break;
                    }
                }
                Err(RecvError::Lagged(_)) => continue,
                Err(RecvError::Closed) => break,
            }
        }
    });

    let mut recv_task = tokio::spawn(async move {
        while let Some(Ok(msg)) = receiver.next().await {
            if matches!(msg, Message::Close(_)) {
                break;
            }
        }
    });

    tokio::select! {
        _ = &mut send_task => recv_task.abort(),
        _ = &mut recv_task => send_task.abort(),
    }
}

pub fn broadcast_event(tx: &EventSender, event: impl Into<String>) {
    let _ = tx.send(event.into());
}
