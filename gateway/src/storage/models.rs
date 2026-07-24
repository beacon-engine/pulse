//! Database record types.
//!
//! These are shared with the gateway via the `protocol` crate; re-exported here
//! so existing `storage::models::*` references keep resolving.

pub use protocol::data::container::ContainerEvent;
pub use protocol::host::{Environment, SavedStack};
