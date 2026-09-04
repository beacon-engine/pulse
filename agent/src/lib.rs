//! Library surface for the Pulse agent.
//!
//! Every agent module is declared here so the `agent` binary (`main.rs`) and the
//! integration tests under `tests/` compile against one shared, public code
//! base. Keeping the modules in the library (rather than private to the binary)
//! is what lets tests live in a separate folder instead of inside `src/`.

pub mod compose;
pub mod config;
pub mod docker;
pub mod events;
pub mod kubernetes;
pub mod manager;
pub mod swarm;
