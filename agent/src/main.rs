//! Pulse agent — a stateless executor.
//!
//! The agent owns Docker, Docker Swarm and Kubernetes access. It listens on a
//! TCP socket and answers [`Request`]s from the gateway with [`Response`]s. It
//! keeps no database of its own: the gateway resolves each host to a
//! `docker_host` connection string and passes it on every request.

mod compose;
mod config;
mod docker;
mod events;
mod kubernetes;
mod manager;
mod swarm;

use std::fmt::Display;
use std::sync::Arc;

use tokio::net::{TcpListener, TcpStream};
use tokio::sync::RwLock;

use protocol::wire::{read_frame, write_frame, Request, Response};

struct AppState {
    docker: Arc<manager::DockerManager>,
    /// Cached Kubernetes client, connected lazily so a kubeconfig that appears
    /// after startup is picked up without restarting the agent.
    k8s: Arc<RwLock<Option<kube::Client>>>,
}

/// Returns a Kubernetes client, connecting (and caching) on first use or if a
/// previous startup attempt failed. `None` when no cluster is reachable.
async fn k8s_client(state: &AppState) -> Option<kube::Client> {
    if let Some(c) = state.k8s.read().await.as_ref() {
        return Some(c.clone());
    }
    match kubernetes::connect().await {
        Ok(c) => {
            *state.k8s.write().await = Some(c.clone());
            Some(c)
        }
        Err(_) => None,
    }
}

#[tokio::main]
async fn main() {
    println!("Starting Pulse agent v0.1.0");

    let cfg = config::Config::from_env();

    // Best-effort initial connect; if it fails we retry lazily per request.
    let k8s = Arc::new(RwLock::new(None));
    match kubernetes::connect().await {
        Ok(client) => {
            println!("Kubernetes connected");
            *k8s.write().await = Some(client);
        }
        Err(e) => println!("Kubernetes not connected yet ({e}); will retry on demand"),
    }

    let state = Arc::new(AppState {
        docker: Arc::new(manager::DockerManager::new()),
        k8s,
    });

    let addr = format!("0.0.0.0:{}", cfg.port);
    let listener = TcpListener::bind(&addr)
        .await
        .unwrap_or_else(|_| panic!("Failed to bind to TCP port {}", cfg.port));

    println!("Pulse agent listening on tcp://{addr}");

    loop {
        let (socket, peer) = match listener.accept().await {
            Ok(v) => v,
            Err(e) => {
                eprintln!("accept error: {e}");
                continue;
            }
        };

        let state = state.clone();
        tokio::spawn(async move {
            if let Err(e) = handle_conn(socket, state).await {
                match e.kind() {
                    // Peer closed cleanly between frames — normal.
                    std::io::ErrorKind::UnexpectedEof => {}
                    // Bytes that aren't our binary protocol (e.g. a browser or
                    // health-check speaking HTTP to the agent port). Drop quietly
                    // with a hint rather than spamming the log.
                    std::io::ErrorKind::InvalidData => {
                        eprintln!(
                            "ignoring non-protocol connection from {peer} \
                             (the agent speaks the binary gateway protocol on this port, not HTTP)"
                        );
                    }
                    _ => eprintln!("connection error from {peer}: {e}"),
                }
            }
        });
    }
}

/// Reads framed requests from one connection until EOF, answering each in turn.
/// A [`Request::WatchEvents`] takes over the connection and streams events.
async fn handle_conn(mut socket: TcpStream, state: Arc<AppState>) -> std::io::Result<()> {
    let (mut reader, mut writer) = socket.split();

    loop {
        let req: Request = read_frame(&mut reader).await?;

        if let Request::WatchEvents { docker_host } = req {
            match state.docker.client_for(&docker_host).await {
                Ok(d) => events::watch(&d, &mut writer).await?,
                Err(e) => {
                    write_frame(&mut writer, &conn_error(e)).await?;
                }
            }
            return Ok(());
        }

        let resp = dispatch(&state, req).await;
        write_frame(&mut writer, &resp).await?;
    }
}

fn conn_error(message: String) -> Response {
    Response::Error { status: 502, message: format!("cannot connect to Docker: {message}") }
}

fn exec_error<E: Display>(e: E) -> Response {
    Response::Error { status: 500, message: e.to_string() }
}

/// Executes a single request and produces its response.
async fn dispatch(state: &AppState, req: Request) -> Response {
    // Resolve the Docker client for host-scoped requests once, up front.
    macro_rules! docker {
        ($host:expr) => {
            match state.docker.client_for(&$host).await {
                Ok(d) => d,
                Err(e) => return conn_error(e),
            }
        };
    }

    match req {
        Request::Ping => Response::Pong,

        // ── Containers ─────────────────────────────────────────────────────────
        Request::ListContainers { docker_host } => {
            let d = docker!(docker_host);
            match docker::list(&d).await {
                Ok(v) => Response::Containers(v),
                Err(e) => exec_error(e),
            }
        }
        Request::InspectContainer { docker_host, id } => {
            let d = docker!(docker_host);
            match docker::inspect(&d, &id).await {
                Ok(v) => Response::ContainerDetail(v),
                Err(e) => exec_error(e),
            }
        }
        Request::InspectRawContainer { docker_host, id } => {
            let d = docker!(docker_host);
            match docker::inspect_raw(&d, &id).await {
                Ok(v) => Response::Raw(v),
                Err(e) => exec_error(e),
            }
        }
        Request::RecreateContainer { docker_host, id, ports } => {
            let d = docker!(docker_host);
            match docker::recreate(&d, &id, &ports).await {
                Ok(new_id) => Response::Id(new_id),
                Err(e) => exec_error(e),
            }
        }
        Request::RecreateLatest { docker_host, id, pull } => {
            let d = docker!(docker_host);
            match docker::recreate_latest(&d, &id, pull).await {
                Ok(new_id) => Response::Id(new_id),
                Err(e) => exec_error(e),
            }
        }
        Request::UpdateContainerEnv { docker_host, id, env } => {
            let d = docker!(docker_host);
            match docker::update_env(&d, &id, &env).await {
                Ok(new_id) => Response::Id(new_id),
                Err(e) => exec_error(e),
            }
        }
        Request::ExecContainer { docker_host, id, cmd } => {
            let d = docker!(docker_host);
            match docker::exec_command(&d, &id, cmd).await {
                Ok(out) => Response::Text(out),
                Err(e) => exec_error(e),
            }
        }
        Request::StartContainer { docker_host, id } => {
            let d = docker!(docker_host);
            match docker::start(&d, &id).await {
                Ok(()) => Response::Ok,
                Err(e) => exec_error(e),
            }
        }
        Request::StopContainer { docker_host, id } => {
            let d = docker!(docker_host);
            match docker::stop(&d, &id).await {
                Ok(()) => Response::Ok,
                Err(e) => exec_error(e),
            }
        }
        Request::RestartContainer { docker_host, id } => {
            let d = docker!(docker_host);
            match docker::restart(&d, &id).await {
                Ok(()) => Response::Ok,
                Err(e) => exec_error(e),
            }
        }
        Request::ContainerLogs { docker_host, id, tail } => {
            let d = docker!(docker_host);
            match docker::logs(&d, &id, tail).await {
                Ok(out) => Response::Text(out),
                Err(e) => exec_error(e),
            }
        }
        Request::ContainerNetworks { docker_host, id } => {
            let d = docker!(docker_host);
            match docker::get_container_networks(&d, &id).await {
                Ok(v) => Response::ContainerNetworks(v),
                Err(e) => exec_error(e),
            }
        }

        // ── System / hardware ────────────────────────────────────────────────────
        Request::SystemInfo { docker_host } => {
            let d = docker!(docker_host);
            match docker::system_info(&d).await {
                Ok(v) => Response::SystemInfo(v),
                Err(e) => exec_error(e),
            }
        }
        Request::HardwareMetrics => Response::HardwareMetrics(docker::hardware_metrics()),

        // ── Images ─────────────────────────────────────────────────────────────
        Request::ListImages { docker_host } => {
            let d = docker!(docker_host);
            match docker::list_images(&d).await {
                Ok(v) => Response::Images(v),
                Err(e) => exec_error(e),
            }
        }
        Request::DeleteImage { docker_host, id } => {
            let d = docker!(docker_host);
            match docker::delete_image(&d, &id).await {
                Ok(()) => Response::Ok,
                Err(e) => exec_error(e),
            }
        }
        Request::PruneImages { docker_host } => {
            let d = docker!(docker_host);
            match docker::prune_images(&d).await {
                Ok(n) => Response::Count(n),
                Err(e) => exec_error(e),
            }
        }

        // ── Volumes ────────────────────────────────────────────────────────────
        Request::ListVolumes { docker_host } => {
            let d = docker!(docker_host);
            match docker::list_volumes(&d).await {
                Ok(v) => Response::Volumes(v),
                Err(e) => exec_error(e),
            }
        }
        Request::DeleteVolume { docker_host, name } => {
            let d = docker!(docker_host);
            match docker::delete_volume(&d, &name).await {
                Ok(()) => Response::Ok,
                Err(e) => exec_error(e),
            }
        }
        Request::PruneVolumes { docker_host } => {
            let d = docker!(docker_host);
            match docker::prune_volumes(&d).await {
                Ok(n) => Response::Count(n),
                Err(e) => exec_error(e),
            }
        }

        // ── Networks ───────────────────────────────────────────────────────────
        Request::ListHostNetworks { docker_host } => {
            let d = docker!(docker_host);
            match docker::list_host_networks(&d).await {
                Ok(v) => Response::HostNetworks(v),
                Err(e) => exec_error(e),
            }
        }
        Request::NetworkConnect { docker_host, network_id, container_id } => {
            let d = docker!(docker_host);
            match docker::connect_to_network(&d, &network_id, &container_id).await {
                Ok(()) => Response::Ok,
                Err(e) => exec_error(e),
            }
        }
        Request::NetworkDisconnect { docker_host, network_id, container_id } => {
            let d = docker!(docker_host);
            match docker::disconnect_from_network(&d, &network_id, &container_id).await {
                Ok(()) => Response::Ok,
                Err(e) => exec_error(e),
            }
        }

        // ── Detected stacks ──────────────────────────────────────────────────────
        Request::ListDetectedStacks { docker_host } => {
            let d = docker!(docker_host);
            match docker::list_stacks(&d).await {
                Ok(v) => Response::DetectedStacks(v),
                Err(e) => exec_error(e),
            }
        }

        // ── Compose ──────────────────────────────────────────────────────────────
        Request::ComposeUp { docker_host, stack } => match compose::up(&docker_host, &stack).await {
            Ok(out) => Response::Text(out),
            Err(e) => exec_error(e),
        },
        Request::ComposeDown { docker_host, stack } => {
            match compose::down(&docker_host, &stack).await {
                Ok(out) => Response::Text(out),
                Err(e) => exec_error(e),
            }
        }
        Request::ComposePull { docker_host, stack } => {
            match compose::pull(&docker_host, &stack).await {
                Ok(out) => Response::Text(out),
                Err(e) => exec_error(e),
            }
        }

        // ── Swarm ──────────────────────────────────────────────────────────────
        Request::ListSwarmServices { docker_host } => {
            let d = docker!(docker_host);
            match swarm::services(&d).await {
                Ok(v) => Response::SwarmServices(v),
                Err(e) => exec_error(e),
            }
        }
        Request::ListSwarmNodes { docker_host } => {
            let d = docker!(docker_host);
            match swarm::nodes(&d).await {
                Ok(v) => Response::SwarmNodes(v),
                Err(e) => exec_error(e),
            }
        }
        Request::ListSwarmTasks { docker_host, service_id } => {
            let d = docker!(docker_host);
            match swarm::tasks(&d, &service_id).await {
                Ok(v) => Response::SwarmTasks(v),
                Err(e) => exec_error(e),
            }
        }
        Request::UpdateSwarmService { docker_host, service_id } => {
            let d = docker!(docker_host);
            match swarm::force_update(&d, &service_id).await {
                Ok(()) => Response::Ok,
                Err(e) => exec_error(e),
            }
        }

        // ── Kubernetes ───────────────────────────────────────────────────────────
        Request::K8sPods => match k8s_client(state).await {
            None => k8s_unavailable(),
            Some(c) => match kubernetes::list_pods(&c).await {
                Ok(v) => Response::Pods(v),
                Err(e) => exec_error(e),
            },
        },
        Request::K8sNamespaces => match k8s_client(state).await {
            None => k8s_unavailable(),
            Some(c) => match kubernetes::list_namespaces(&c).await {
                Ok(v) => Response::Namespaces(v),
                Err(e) => exec_error(e),
            },
        },
        Request::K8sRollout { namespace, deployment } => match k8s_client(state).await {
            None => k8s_unavailable(),
            Some(c) => match kubernetes::rollout(&c, &namespace, &deployment).await {
                Ok(()) => Response::Ok,
                Err(e) => exec_error(e),
            },
        },
        Request::K8sDeletePod { namespace, name } => match k8s_client(state).await {
            None => k8s_unavailable(),
            Some(c) => match kubernetes::delete_pod(&c, &namespace, &name).await {
                Ok(()) => Response::Ok,
                Err(e) => exec_error(e),
            },
        },

        // Handled before dispatch — see handle_conn.
        Request::WatchEvents { .. } => Response::Error {
            status: 400,
            message: "WatchEvents must be sent on a dedicated connection".into(),
        },
    }
}

fn k8s_unavailable() -> Response {
    Response::Error { status: 503, message: "Kubernetes not connected".into() }
}
