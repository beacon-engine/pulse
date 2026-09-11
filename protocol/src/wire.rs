//! The gateway <-> agent TCP wire protocol.
//!
//! The gateway owns the database (hosts, stacks, webhook tokens, events) and the
//! public HTTP API. The agent is a stateless executor that owns Docker, Docker
//! Swarm and Kubernetes access. For every operation the gateway resolves the
//! target host to a `docker_host` connection string and sends a [`Request`]; the
//! agent performs the work and replies with a [`Response`].
//!
//! Frames are length-prefixed JSON: a big-endian `u32` byte length followed by
//! that many bytes of JSON. See [`write_frame`] / [`read_frame`].

use serde::{Deserialize, Serialize};
use tokio::io::{AsyncReadExt, AsyncWriteExt};

use crate::data::container::{
    ContainerDetail, ContainerInfo, ContainerNetworkInfo, HostNetwork, ImageInfo, PortMap, Stack,
    VolumeInfo,
};
use crate::data::orchestration::{NamespaceInfo, PodInfo, SwarmNode, SwarmService, SwarmTask};
use crate::data::system::{HardwareMetrics, SystemInfo};

/// Everything the agent needs to run `docker compose` for a stack without a
/// database of its own. The gateway loads the persisted stack and forwards it.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ComposeStack {
    pub id: String,
    pub name: String,
    pub content: String,
    /// JSON-encoded `Vec<{key,value}>`, written to a `.env` file by the agent.
    pub env_vars: Option<String>,
}

/// A request the gateway forwards to the agent.
///
/// Docker-scoped variants carry `docker_host` — the connection string the agent
/// uses to reach the right daemon (`unix://…`, `tcp://…`, or empty/local).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum Request {
    // ── Liveness ──────────────────────────────────────────────────────────────
    Ping,

    // ── Containers ──────────────────────────────────────────────────────────────
    ListContainers {
        docker_host: String,
    },
    InspectContainer {
        docker_host: String,
        id: String,
    },
    InspectRawContainer {
        docker_host: String,
        id: String,
    },
    RecreateContainer {
        docker_host: String,
        id: String,
        ports: Vec<PortMap>,
    },
    /// Pull (optionally) the current image tag and recreate — used by webhooks.
    RecreateLatest {
        docker_host: String,
        id: String,
        pull: bool,
    },
    UpdateContainerEnv {
        docker_host: String,
        id: String,
        env: Vec<String>,
    },
    ExecContainer {
        docker_host: String,
        id: String,
        cmd: Vec<String>,
    },
    StartContainer {
        docker_host: String,
        id: String,
    },
    StopContainer {
        docker_host: String,
        id: String,
    },
    RestartContainer {
        docker_host: String,
        id: String,
    },
    ContainerLogs {
        docker_host: String,
        id: String,
        tail: u32,
    },
    ContainerNetworks {
        docker_host: String,
        id: String,
    },

    // ── System / hardware ────────────────────────────────────────────────────────
    SystemInfo {
        docker_host: String,
    },
    /// Hardware metrics for the machine the agent runs on (independent of host).
    HardwareMetrics,

    // ── Images ────────────────────────────────────────────────────────────────────
    ListImages {
        docker_host: String,
    },
    DeleteImage {
        docker_host: String,
        id: String,
    },
    PruneImages {
        docker_host: String,
    },

    // ── Volumes ─────────────────────────────────────────────────────────────────
    ListVolumes {
        docker_host: String,
    },
    DeleteVolume {
        docker_host: String,
        name: String,
    },
    PruneVolumes {
        docker_host: String,
    },

    // ── Networks ──────────────────────────────────────────────────────────────────
    ListHostNetworks {
        docker_host: String,
    },
    NetworkConnect {
        docker_host: String,
        network_id: String,
        container_id: String,
    },
    NetworkDisconnect {
        docker_host: String,
        network_id: String,
        container_id: String,
    },

    // ── Detected stacks (from container labels) ───────────────────────────────────
    ListDetectedStacks {
        docker_host: String,
    },

    // ── Compose ─────────────────────────────────────────────────────────────────
    ComposeUp {
        docker_host: String,
        stack: ComposeStack,
    },
    ComposeDown {
        docker_host: String,
        stack: ComposeStack,
    },
    ComposePull {
        docker_host: String,
        stack: ComposeStack,
    },

    // ── Swarm ─────────────────────────────────────────────────────────────────────
    ListSwarmServices {
        docker_host: String,
    },
    ListSwarmNodes {
        docker_host: String,
    },
    /// Tasks (containers) belonging to a swarm service.
    ListSwarmTasks {
        docker_host: String,
        service_id: String,
    },
    /// Force a rolling update of a swarm service (bump force_update).
    UpdateSwarmService {
        docker_host: String,
        service_id: String,
    },
    ScaleSwarmService {
        docker_host: String,
        service_id: String,
        target_replicas: u64,
    },

    // ── Kubernetes ────────────────────────────────────────────────────────────────
    K8sPods,
    K8sNamespaces,
    K8sRollout {
        namespace: String,
        deployment: String,
    },
    K8sDeletePod {
        namespace: String,
        name: String,
    },

    // ── Event stream ──────────────────────────────────────────────────────────────
    /// Opens a long-lived stream: the agent replies with a sequence of
    /// [`Response::Event`] frames (one per Docker event) until the connection
    /// closes. Use a dedicated connection for this.
    WatchEvents {
        docker_host: String,
    },
}

/// A single Docker lifecycle event streamed in response to [`Request::WatchEvents`].
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EventFrame {
    pub container_id: String,
    pub container_name: String,
    /// Raw Docker action: `die`, `oom`, `start`, `stop`, `restart`, …
    pub action: String,
    pub exit_code: Option<String>,
}

/// The agent's reply to a [`Request`].
#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum Response {
    /// Operation succeeded with no payload (start/stop/delete/prune-with-no-value).
    Ok,
    Pong,
    Containers(Vec<ContainerInfo>),
    ContainerDetail(ContainerDetail),
    /// Raw JSON (e.g. full container inspect).
    Raw(serde_json::Value),
    /// A resource id (e.g. the id of a recreated container).
    Id(String),
    /// Free text (logs, exec output, compose output).
    Text(String),
    /// A numeric result (e.g. bytes reclaimed by prune).
    Count(i64),
    SystemInfo(SystemInfo),
    HardwareMetrics(HardwareMetrics),
    Images(Vec<ImageInfo>),
    Volumes(Vec<VolumeInfo>),
    HostNetworks(Vec<HostNetwork>),
    ContainerNetworks(Vec<ContainerNetworkInfo>),
    DetectedStacks(Vec<Stack>),
    SwarmServices(Vec<SwarmService>),
    SwarmNodes(Vec<SwarmNode>),
    SwarmTasks(Vec<SwarmTask>),
    Pods(Vec<PodInfo>),
    Namespaces(Vec<NamespaceInfo>),
    /// One frame of a [`Request::WatchEvents`] stream.
    Event(EventFrame),
    /// The operation failed. `status` is an HTTP status code the gateway can relay.
    Error {
        status: u16,
        message: String,
    },
}

/// Reserved correlation id for unsolicited event pushes (the live Docker event
/// feed). Events are emitted by the agent rather than answering a request, so
/// they never collide with a real request id (which start at 1).
pub const EVENT_STREAM_ID: u64 = 0;

/// A gateway→agent request tagged with a correlation id.
///
/// The single WebSocket session multiplexes many concurrent exchanges plus the
/// event feed, so — unlike the legacy one-connection-per-request TCP path — each
/// message carries an `id` the reply echoes back. See [`ResponseFrame`].
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RequestFrame {
    pub id: u64,
    pub req: Request,
}

/// An agent→gateway message. For a reply, `id` echoes the originating
/// [`RequestFrame::id`]; for a pushed event it is [`EVENT_STREAM_ID`].
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseFrame {
    pub id: u64,
    pub resp: Response,
}

/// Largest frame we will read — guards against a corrupt/hostile length prefix.
const MAX_FRAME: usize = 64 * 1024 * 1024;

/// Writes a length-prefixed JSON frame.
pub async fn write_frame<W, T>(w: &mut W, value: &T) -> std::io::Result<()>
where
    W: AsyncWriteExt + Unpin,
    T: Serialize,
{
    let bytes = serde_json::to_vec(value)
        .map_err(|e| std::io::Error::new(std::io::ErrorKind::InvalidData, e))?;
    if bytes.len() > MAX_FRAME {
        return Err(std::io::Error::new(
            std::io::ErrorKind::InvalidData,
            "frame exceeds maximum size",
        ));
    }
    w.write_all(&(bytes.len() as u32).to_be_bytes()).await?;
    w.write_all(&bytes).await?;
    w.flush().await?;
    Ok(())
}

/// Reads a single length-prefixed JSON frame.
///
/// Returns [`std::io::ErrorKind::UnexpectedEof`] when the peer closes the
/// connection cleanly between frames.
pub async fn read_frame<R, T>(r: &mut R) -> std::io::Result<T>
where
    R: AsyncReadExt + Unpin,
    T: for<'de> Deserialize<'de>,
{
    let mut len_buf = [0u8; 4];
    r.read_exact(&mut len_buf).await?;
    let len = u32::from_be_bytes(len_buf) as usize;
    if len > MAX_FRAME {
        return Err(std::io::Error::new(
            std::io::ErrorKind::InvalidData,
            "frame exceeds maximum size",
        ));
    }
    let mut buf = vec![0u8; len];
    r.read_exact(&mut buf).await?;
    serde_json::from_slice(&buf)
        .map_err(|e| std::io::Error::new(std::io::ErrorKind::InvalidData, e))
}
