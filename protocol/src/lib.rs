//! Shared wire types for the Pulse gateway <-> agent protocol.
//!
//! The gateway forwards operator requests to an agent and deserializes the
//! agent's responses. Both sides link this crate so the request/response
//! vocabulary is defined exactly once.
//!
//! - [`data`] — domain models the agent produces (containers, images, hosts,
//!   system + hardware info, swarm/kubernetes objects).
//! - [`host`] — persisted host/stack records.
//! - [`protocol`] — request bodies the gateway sends and response envelopes the
//!   agent returns.

pub mod data;
pub mod host;
pub mod protocol;
pub mod transport;
pub mod wire;
