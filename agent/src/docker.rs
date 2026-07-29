use bollard::container::LogOutput;
use bollard::models::ContainerCreateBody;
use bollard::query_parameters::{
    CreateContainerOptions, CreateImageOptions, InspectContainerOptions, ListContainersOptions,
    LogsOptions, RemoveContainerOptions, RestartContainerOptions, StartContainerOptions,
    StopContainerOptions,
};
use bollard::Docker;
use futures_util::StreamExt;
use std::collections::HashMap;

// ── Shared wire types (defined in the `protocol` crate) ──────────────────────
//
// These describe what the agent returns to the gateway; they live in `protocol`
// so the gateway can deserialize them without duplicating the definitions.

pub use protocol::data::container::{
    ContainerDetail, ContainerInfo, ContainerMount, ContainerNetworkInfo, ContainerPort,
    HostNetwork, ImageInfo, PortMap, Stack, VolumeInfo,
};
pub use protocol::data::system::{DiskInfo, HardwareMetrics, SystemInfo};

// ── List ──────────────────────────────────────────────────────────────────────

pub async fn list(docker: &Docker) -> Result<Vec<ContainerInfo>, bollard::errors::Error> {
    let mut filters: HashMap<String, Vec<String>> = HashMap::new();
    filters.insert(
        "status".to_string(),
        vec!["running".to_string(), "exited".to_string(), "paused".to_string(), "created".to_string()],
    );

    let containers = docker
        .list_containers(Some(ListContainersOptions { all: true, filters: Some(filters), ..Default::default() }))
        .await?;

    let mut result: Vec<ContainerInfo> = containers
        .into_iter()
        .map(|c| {
            let stack = c.labels.as_ref().and_then(|l| {
                l.get("com.docker.compose.project")
                    .or_else(|| l.get("com.docker.stack.namespace"))
                    .cloned()
            });
            ContainerInfo {
                id: c.id.unwrap_or_default(),
                name: c
                    .names
                    .unwrap_or_default()
                    .first()
                    .cloned()
                    .unwrap_or_default()
                    .trim_start_matches('/')
                    .to_string(),
                image: c.image.unwrap_or_default(),
                status: c.status.unwrap_or_default(),
                state: c.state.as_ref().map(|s| s.to_string()).unwrap_or_default(),
                stack,
            }
        })
        .collect();

    // Stable ordering so 10s UI refreshes don't reshuffle rows.
    result.sort_by(|a, b| a.name.cmp(&b.name));
    Ok(result)
}

// ── Inspect ───────────────────────────────────────────────────────────────────

pub async fn inspect(docker: &Docker, id: &str) -> Result<ContainerDetail, bollard::errors::Error> {
    let info = docker.inspect_container(id, None::<InspectContainerOptions>).await?;

    let config = info.config.unwrap_or_default();
    let state = info.state.unwrap_or_default();
    let host_config = info.host_config.unwrap_or_default();

    let ports = info
        .network_settings
        .and_then(|ns| ns.ports)
        .map(|port_map| {
            let mut result: Vec<ContainerPort> = port_map
                .into_iter()
                .flat_map(|(port_proto, bindings)| {
                    let mut parts = port_proto.splitn(2, '/');
                    let cport = parts.next().unwrap_or("").to_string();
                    let proto = parts.next().unwrap_or("tcp").to_string();
                    bindings
                        .unwrap_or_default()
                        .into_iter()
                        .map(move |b| ContainerPort {
                            container_port: cport.clone(),
                            protocol: proto.clone(),
                            host_ip: b.host_ip.unwrap_or_else(|| "0.0.0.0".to_string()),
                            host_port: b.host_port.unwrap_or_default(),
                        })
                })
                .collect();
            result.sort_by(|a, b| a.container_port.cmp(&b.container_port));
            result
        })
        .unwrap_or_default();

    let mounts = info
        .mounts
        .unwrap_or_default()
        .into_iter()
        .map(|m| ContainerMount {
            source: m.source.unwrap_or_default(),
            destination: m.destination.unwrap_or_default(),
            mode: m.mode.unwrap_or_default(),
            rw: m.rw.unwrap_or(false),
        })
        .collect();

    let restart_policy = host_config
        .restart_policy
        .and_then(|rp| rp.name)
        .map(|n| n.to_string())
        .unwrap_or_default();

    Ok(ContainerDetail {
        id: info.id.unwrap_or_default(),
        name: info.name.unwrap_or_default().trim_start_matches('/').to_string(),
        image: config.image.unwrap_or_default(),
        created: info.created.unwrap_or_default(),
        state: state.status.as_ref().map(|s| s.to_string()).unwrap_or_default(),
        started_at: state.started_at.unwrap_or_default(),
        finished_at: state.finished_at.unwrap_or_default(),
        restart_count: info.restart_count.unwrap_or(0),
        ports,
        mounts,
        env: config.env.unwrap_or_default(),
        cmd: config.cmd.unwrap_or_default(),
        network_mode: host_config.network_mode.unwrap_or_default(),
        restart_policy,
    })
}

pub async fn inspect_raw(
    docker: &Docker,
    id: &str,
) -> Result<serde_json::Value, bollard::errors::Error> {
    let info = docker.inspect_container(id, None::<InspectContainerOptions>).await?;
    Ok(serde_json::to_value(info).unwrap_or_default())
}

// ── Recreate with new port mappings ──────────────────────────────────────────

pub async fn recreate(
    docker: &Docker,
    id: &str,
    new_ports: &[PortMap],
) -> Result<String, bollard::errors::Error> {
    let info = docker.inspect_container(id, None::<InspectContainerOptions>).await?;
    let name = info.name.unwrap_or_default().trim_start_matches('/').to_string();
    let config = info.config.unwrap_or_default();
    let mut host_config = info.host_config.unwrap_or_default();

    // Build new port bindings
    let mut bindings: HashMap<String, Option<Vec<bollard::models::PortBinding>>> = HashMap::new();
    for pm in new_ports {
        let key = format!("{}/{}", pm.container_port, pm.protocol);
        bindings.insert(
            key,
            Some(vec![bollard::models::PortBinding {
                host_ip: Some("0.0.0.0".to_string()),
                host_port: Some(pm.host_port.clone()),
            }]),
        );
    }
    host_config.port_bindings = Some(bindings);

    // Stop and remove old container (best effort stop)
    let _ = docker.stop_container(id, Some(StopContainerOptions { t: Some(5), ..Default::default() })).await;
    docker
        .remove_container(id, Some(RemoveContainerOptions { force: true, ..Default::default() }))
        .await?;

    // Create with same name and updated ports
    let created = docker
        .create_container(
            Some(CreateContainerOptions { name: Some(name.clone()), ..Default::default() }),
            ContainerCreateBody {
                image: config.image.clone(),
                cmd: config.cmd.clone(),
                env: config.env.clone(),
                labels: config.labels.clone(),
                host_config: Some(host_config),
                ..Default::default()
            },
        )
        .await?;

    docker.start_container(&created.id, None::<StartContainerOptions>).await?;
    Ok(created.id)
}

// ── Recreate a container (optionally pulling the latest image first) ───────

pub async fn recreate_latest(
    docker: &Docker,
    id: &str,
    pull: bool,
) -> Result<String, bollard::errors::Error> {
    let info = docker.inspect_container(id, None::<InspectContainerOptions>).await?;
    let name = info.name.unwrap_or_default().trim_start_matches('/').to_string();
    let config = info.config.unwrap_or_default();
    let host_config = info.host_config.unwrap_or_default();
    let image = config.image.clone().unwrap_or_default();

    let _ = docker.stop_container(id, Some(StopContainerOptions { t: Some(5), ..Default::default() })).await;
    docker
        .remove_container(id, Some(RemoveContainerOptions { force: true, ..Default::default() }))
        .await?;

    if pull && !image.is_empty() {
        let mut stream = docker.create_image(
            Some(CreateImageOptions {
                from_image: Some(image.clone()),
                ..Default::default()
            }),
            None,
            None,
        );
        while let Some(result) = stream.next().await {
            if let Err(e) = result {
                return Err(e);
            }
        }
    }

    let created = docker
        .create_container(
            Some(CreateContainerOptions { name: Some(name.clone()), ..Default::default() }),
            ContainerCreateBody {
                image: Some(image),
                cmd: config.cmd.clone(),
                env: config.env.clone(),
                labels: config.labels.clone(),
                host_config: Some(host_config),
                ..Default::default()
            },
        )
        .await?;

    docker.start_container(&created.id, None::<StartContainerOptions>).await?;
    Ok(created.id)
}

// ── Update environment variables and restart ───────────────────────────────

pub async fn update_env(
    docker: &Docker,
    id: &str,
    env: &[String],
) -> Result<String, bollard::errors::Error> {
    let info = docker.inspect_container(id, None::<InspectContainerOptions>).await?;
    let name = info.name.unwrap_or_default().trim_start_matches('/').to_string();
    let config = info.config.unwrap_or_default();
    let host_config = info.host_config.unwrap_or_default();

    let _ = docker.stop_container(id, Some(StopContainerOptions { t: Some(5), ..Default::default() })).await;
    docker
        .remove_container(id, Some(RemoveContainerOptions { force: true, ..Default::default() }))
        .await?;

    let created = docker
        .create_container(
            Some(CreateContainerOptions { name: Some(name.clone()), ..Default::default() }),
            ContainerCreateBody {
                image: config.image.clone(),
                cmd: config.cmd.clone(),
                env: Some(env.to_vec()),
                labels: config.labels.clone(),
                host_config: Some(host_config),
                ..Default::default()
            },
        )
        .await?;

    docker.start_container(&created.id, None::<StartContainerOptions>).await?;
    Ok(created.id)
}

// ── Logs ──────────────────────────────────────────────────────────────────────

pub async fn logs(docker: &Docker, id: &str, tail: u32) -> Result<String, bollard::errors::Error> {
    let mut stream = docker.logs(
        id,
        Some(LogsOptions {
            stdout: true,
            stderr: true,
            tail: tail.to_string(),
            timestamps: true,
            ..Default::default()
        }),
    );
    let mut output = String::new();
    while let Some(chunk) = stream.next().await {
        match chunk? {
            LogOutput::StdOut { message } | LogOutput::StdErr { message } => {
                output.push_str(&String::from_utf8_lossy(&message));
            }
            _ => {}
        }
    }
    Ok(output)
}

// ── Stacks ────────────────────────────────────────────────────────────────────

pub async fn list_stacks(docker: &Docker) -> Result<Vec<Stack>, bollard::errors::Error> {
    let all = list(docker).await?;
    let mut groups: HashMap<String, Vec<ContainerInfo>> = HashMap::new();
    for c in all {
        if let Some(ref name) = c.stack {
            groups.entry(name.clone()).or_default().push(c);
        }
    }
    let mut stacks: Vec<Stack> = groups
        .into_iter()
        .map(|(name, services)| Stack { name, services })
        .collect();
    stacks.sort_by(|a, b| a.name.cmp(&b.name));
    Ok(stacks)
}

// ── System info ───────────────────────────────────────────────────────────────

pub async fn system_info(docker: &Docker) -> Result<SystemInfo, bollard::errors::Error> {
    let info = docker.info().await?;
    Ok(SystemInfo {
        docker_version: info.server_version.unwrap_or_default(),
        api_version: "1.43".into(),
        os: format!(
            "{} {}",
            info.operating_system.unwrap_or_default(),
            info.os_version.unwrap_or_default()
        )
        .trim()
        .to_string(),
        arch: info.architecture.unwrap_or_default(),
        kernel: info.kernel_version.unwrap_or_default(),
        total_memory: info.mem_total.unwrap_or(0),
        cpus: info.ncpu.unwrap_or(0),
        containers_running: info.containers_running.unwrap_or(0),
        containers_stopped: info.containers_paused.unwrap_or(0)
            + info.containers_stopped.unwrap_or(0),
        images: info.images.unwrap_or(0) as i64,
    })
}

// ── Images ────────────────────────────────────────────────────────────────────

pub async fn list_images(docker: &Docker) -> Result<Vec<ImageInfo>, bollard::errors::Error> {
    // `all: false` is the Docker default, so no options are needed.
    let images = docker
        .list_images(None::<bollard::query_parameters::ListImagesOptions>)
        .await?;
    Ok(images
        .into_iter()
        .map(|img| {
            let dangling = img.repo_tags.is_empty()
                || img.repo_tags.iter().all(|t| t == "<none>:<none>");
            ImageInfo {
                id: img.id,
                tags: img.repo_tags.into_iter().filter(|t| t != "<none>:<none>").collect(),
                size: img.size,
                created: img.created,
                dangling,
            }
        })
        .collect())
}

pub async fn delete_image(docker: &Docker, id: &str) -> Result<(), bollard::errors::Error> {
    use bollard::query_parameters::RemoveImageOptionsBuilder;
    docker
        .remove_image(id, Some(RemoveImageOptionsBuilder::new().force(true).build()), None)
        .await?;
    Ok(())
}

pub async fn prune_images(docker: &Docker) -> Result<i64, bollard::errors::Error> {
    let result = docker
        .prune_images(None::<bollard::query_parameters::PruneImagesOptions>)
        .await?;
    Ok(result.space_reclaimed.unwrap_or(0))
}

// ── Volumes ───────────────────────────────────────────────────────────────────

pub async fn list_volumes(docker: &Docker) -> Result<Vec<VolumeInfo>, bollard::errors::Error> {
    let result = docker
        .list_volumes(None::<bollard::query_parameters::ListVolumesOptions>)
        .await?;
    Ok(result
        .volumes
        .unwrap_or_default()
        .into_iter()
        .map(|v| VolumeInfo {
            name: v.name,
            driver: v.driver,
            mountpoint: v.mountpoint,
            created: v.created_at.unwrap_or_default(),
        })
        .collect())
}

pub async fn delete_volume(docker: &Docker, name: &str) -> Result<(), bollard::errors::Error> {
    use bollard::query_parameters::RemoveVolumeOptionsBuilder;
    docker
        .remove_volume(name, Some(RemoveVolumeOptionsBuilder::new().force(true).build()))
        .await
}

pub async fn prune_volumes(docker: &Docker) -> Result<i64, bollard::errors::Error> {
    let result = docker
        .prune_volumes(None::<bollard::query_parameters::PruneVolumesOptions>)
        .await?;
    Ok(result.space_reclaimed.unwrap_or(0))
}

// ── Networks ──────────────────────────────────────────────────────────────────

pub async fn list_host_networks(docker: &Docker) -> Result<Vec<HostNetwork>, bollard::errors::Error> {
    let nets = docker.list_networks(None::<bollard::query_parameters::ListNetworksOptions>).await?;
    let mut result: Vec<HostNetwork> = nets
        .into_iter()
        .map(|n| HostNetwork {
            id: n.id.unwrap_or_default(),
            name: n.name.unwrap_or_default(),
            driver: n.driver.unwrap_or_default(),
            scope: n.scope.unwrap_or_default(),
            internal: n.internal.unwrap_or(false),
        })
        .collect();
    result.sort_by(|a, b| a.name.cmp(&b.name));
    Ok(result)
}

pub async fn get_container_networks(
    docker: &Docker,
    container_id: &str,
) -> Result<Vec<ContainerNetworkInfo>, bollard::errors::Error> {
    let info = docker.inspect_container(container_id, None::<InspectContainerOptions>).await?;
    let map = info
        .network_settings
        .and_then(|ns| ns.networks)
        .unwrap_or_default();
    Ok(map
        .into_iter()
        .map(|(name, ep)| ContainerNetworkInfo {
            id: ep.network_id.unwrap_or_default(),
            name,
            ip_address: ep.ip_address.unwrap_or_default(),
            gateway: ep.gateway.unwrap_or_default(),
            mac_address: ep.mac_address.unwrap_or_default(),
        })
        .collect())
}

pub async fn connect_to_network(
    docker: &Docker,
    network_id: &str,
    container_id: &str,
) -> Result<(), bollard::errors::Error> {
    docker
        .connect_network(
            network_id,
            bollard::models::NetworkConnectRequest {
                container: Some(container_id.to_string()),
                endpoint_config: Some(bollard::models::EndpointSettings::default()),
            },
        )
        .await
}

pub async fn disconnect_from_network(
    docker: &Docker,
    network_id: &str,
    container_id: &str,
) -> Result<(), bollard::errors::Error> {
    docker
        .disconnect_network(
            network_id,
            bollard::models::NetworkDisconnectRequest {
                container: Some(container_id.to_string()),
                force: Some(false),
            },
        )
        .await
}

// ── Exec a command inside a container ─────────────────────────────────────────

pub async fn exec_command(
    docker: &Docker,
    container_id: &str,
    cmd: Vec<String>,
) -> Result<String, bollard::errors::Error> {
    use bollard::exec::{CreateExecOptions, StartExecResults};

    let exec = docker
        .create_exec(
            container_id,
            CreateExecOptions {
                attach_stdout: Some(true),
                attach_stderr: Some(true),
                cmd: Some(cmd),
                ..Default::default()
            },
        )
        .await?;

    let mut out = String::new();
    if let StartExecResults::Attached { mut output, .. } =
        docker.start_exec(&exec.id, None).await?
    {
        while let Some(Ok(msg)) = output.next().await {
            match msg {
                LogOutput::StdOut { message } | LogOutput::StdErr { message } => {
                    out.push_str(&String::from_utf8_lossy(&message));
                }
                _ => {}
            }
        }
    }
    Ok(out)
}

// ── Hardware metrics (local machine) ──────────────────────────────────────────

pub fn hardware_metrics() -> HardwareMetrics {
    use sysinfo::System;

    let mut sys = System::new();
    sys.refresh_memory();

    let mem_total = sys.total_memory();
    let mem_used = sys.used_memory();
    let mem_pct = if mem_total > 0 { mem_used as f32 / mem_total as f32 * 100.0 } else { 0.0 };

    HardwareMetrics { mem_used, mem_total, mem_pct, disks: disk_metrics() }
}

/// Chooses the disk collector honestly:
///
/// - containerized **with** `HOST_ROOT` → read the host's real disks through the
///   bind-mount;
/// - containerized **without** `HOST_ROOT` → we can only see the container's own
///   overlay filesystem, which is misleading, so report nothing;
/// - **not** containerized (bare-metal) → `sysinfo` sees the real machine.
///
/// Note: on Docker Desktop (Windows/macOS) the "host" is the Docker Linux VM, not
/// the physical machine. To monitor a Windows/macOS host, run the agent natively.
fn disk_metrics() -> Vec<DiskInfo> {
    let host_root = std::env::var("HOST_ROOT").unwrap_or_default();

    if !host_root.is_empty() && host_root != "/" {
        return host_disk_metrics(&host_root);
    }

    if in_container() {
        eprintln!(
            "host disk metrics unavailable: running in a container without HOST_ROOT. \
             Bind-mount the host (-v /:/host:ro) and set HOST_ROOT=/host to enable them."
        );
        return Vec::new();
    }

    local_disk_metrics()
}

/// Best-effort detection of whether we're running inside a container.
fn in_container() -> bool {
    std::path::Path::new("/.dockerenv").exists()
        || std::path::Path::new("/run/.containerenv").exists()
}

/// Reads the host's mounts (via `HOST_ROOT/proc/*/mounts`) and `statvfs`'s each
/// real filesystem through the bind-mount. Used when the agent runs in a
/// container with the host mounted read-only at `HOST_ROOT` (e.g. `-v /:/host:ro`).
fn host_disk_metrics(root: &str) -> Vec<DiskInfo> {
    use std::collections::HashMap;

    // Pseudo/virtual filesystems we never want to report.
    const SKIP_FS: &[&str] = &[
        "proc", "sysfs", "tmpfs", "devtmpfs", "devpts", "cgroup", "cgroup2", "overlay",
        "squashfs", "mqueue", "fusectl", "debugfs", "tracefs", "bpf", "configfs", "pstore",
        "securityfs", "ramfs", "autofs", "binfmt_misc", "nsfs", "hugetlbfs", "fuse.lxcfs",
    ];

    // The host mount table. Prefer PID 1's table (the host mount namespace);
    // fall back to the generic file, then to the local collector.
    let mounts = std::fs::read_to_string(format!("{root}/proc/1/mounts"))
        .or_else(|_| std::fs::read_to_string(format!("{root}/proc/mounts")));
    let Ok(mounts) = mounts else { return local_disk_metrics() };

    // Deduplicate by (total, available) — bind mounts report identical stats.
    let mut best: HashMap<(u64, u64), (i32, DiskInfo)> = HashMap::new();

    for line in mounts.lines() {
        let f: Vec<&str> = line.split_whitespace().collect();
        if f.len() < 3 {
            continue;
        }
        let device = f[0];
        let mount = f[1].replace("\\040", " "); // octal-escaped spaces in /proc/mounts
        let fstype = f[2];

        if SKIP_FS.contains(&fstype) {
            continue;
        }
        // Real block devices, plus common network/pooled filesystems.
        let is_real = device.starts_with("/dev/")
            || matches!(fstype, "zfs" | "nfs" | "nfs4" | "cifs" | "ceph");
        if !is_real {
            continue;
        }

        // The host path as seen from inside the container: HOST_ROOT + mount.
        let path = if mount == "/" { root.to_string() } else { format!("{root}{mount}") };
        let Some((total, available)) = statvfs_bytes(&path) else { continue };
        if total < 1_000_000_000 {
            continue; // skip < 1 GB
        }

        let used = total.saturating_sub(available);
        let pct = if total > 0 { used as f32 / total as f32 * 100.0 } else { 0.0 };
        let info = DiskInfo { name: device.to_string(), mount: mount.clone(), total, used, pct };

        let priority: i32 = if mount == "/" { 100 } else { 10 };
        let key = (total, available);
        let entry = best.entry(key).or_insert((priority, info.clone()));
        if priority > entry.0 {
            *entry = (priority, info);
        }
    }

    let mut result: Vec<DiskInfo> = best.into_values().map(|(_, d)| d).collect();
    result.sort_by(|a, b| b.total.cmp(&a.total));
    result
}

/// `statvfs(path)` → `(total_bytes, available_bytes)`, or `None` on error.
fn statvfs_bytes(path: &str) -> Option<(u64, u64)> {
    let c_path = std::ffi::CString::new(path).ok()?;
    // SAFETY: `c_path` is a valid NUL-terminated string; `stat` is zeroed and
    // only read after a successful (0) return.
    unsafe {
        let mut stat: libc::statvfs = std::mem::zeroed();
        if libc::statvfs(c_path.as_ptr(), &mut stat) != 0 {
            return None;
        }
        let frsize = stat.f_frsize as u64;
        let total = (stat.f_blocks as u64).saturating_mul(frsize);
        let available = (stat.f_bavail as u64).saturating_mul(frsize);
        Some((total, available))
    }
}

fn local_disk_metrics() -> Vec<DiskInfo> {
    use sysinfo::Disks;
    use std::collections::HashMap;

    let list = Disks::new_with_refreshed_list();

    // On macOS, multiple APFS logical volumes share the same physical container
    // and report identical (total, available) stats. Deduplicate by those values
    // and keep the entry with the highest mount priority.
    let mut best: HashMap<(u64, u64), (i32, DiskInfo)> = HashMap::new();

    for d in list.iter() {
        let total = d.total_space();
        // Skip tiny/virtual volumes (< 1 GB)
        if total < 1_000_000_000 { continue; }

        let mount = d.mount_point().to_string_lossy().to_string();

        // Skip noisy/internal macOS paths
        if mount.contains("AppTranslocation") { continue; }
        if mount.contains("CoreSimulator") { continue; }
        if mount.starts_with("/private/var/") { continue; }
        if mount.starts_with("/System/Volumes/") { continue; }

        let priority: i32 = if mount == "/" { 100 }
            else if mount.starts_with("/Volumes/") { 50 } // external drives
            else { 10 };

        let available = d.available_space();
        let used = total.saturating_sub(available);
        let pct = if total > 0 { used as f32 / total as f32 * 100.0 } else { 0.0 };

        let info = DiskInfo {
            name: d.name().to_string_lossy().to_string(),
            mount: mount.clone(),
            total, used, pct,
        };

        let key = (total, available);
        let entry = best.entry(key).or_insert((priority, info.clone()));
        if priority > entry.0 {
            *entry = (priority, info);
        }
    }

    let mut result: Vec<DiskInfo> = best.into_values().map(|(_, d)| d).collect();
    result.sort_by(|a, b| b.total.cmp(&a.total));
    result
}

// ── Basic actions ─────────────────────────────────────────────────────────────

pub async fn start(docker: &Docker, id: &str) -> Result<(), bollard::errors::Error> {
    docker.start_container(id, None::<StartContainerOptions>).await
}

pub async fn stop(docker: &Docker, id: &str) -> Result<(), bollard::errors::Error> {
    docker.stop_container(id, Some(StopContainerOptions { t: Some(10), ..Default::default() })).await
}

pub async fn restart(docker: &Docker, id: &str) -> Result<(), bollard::errors::Error> {
    docker.restart_container(id, Some(RestartContainerOptions { t: Some(10), ..Default::default() })).await
}
