use std::fs;
use std::path::PathBuf;
use std::process::Stdio;

use tokio::process::Command;

use protocol::wire::ComposeStack;

fn project_dir(stack: &ComposeStack) -> PathBuf {
    let mut dir = std::env::temp_dir();
    dir.push("pulse-stacks");
    dir.push(&stack.id);
    dir
}

fn write_compose_file(stack: &ComposeStack) -> Result<PathBuf, String> {
    let dir = project_dir(stack);
    fs::create_dir_all(&dir).map_err(|e| format!("create stack dir: {e}"))?;

    // Write .env so docker compose can substitute ${VAR} references
    if let Some(env_json) = &stack.env_vars {
        if let Ok(vars) = serde_json::from_str::<Vec<serde_json::Value>>(env_json) {
            let content: String = vars
                .iter()
                .filter_map(|v| {
                    let k = v["key"].as_str().filter(|s| !s.is_empty())?;
                    let val = v["value"].as_str().unwrap_or("");
                    Some(format!("{k}={val}\n"))
                })
                .collect();
            fs::write(dir.join(".env"), content).map_err(|e| format!("write .env file: {e}"))?;
        }
    }

    let path = dir.join("docker-compose.yml");
    fs::write(&path, &stack.content).map_err(|e| format!("write compose file: {e}"))?;
    Ok(path)
}

/// Builds the environment for the `docker compose` child process, pointing it at
/// the target daemon via `DOCKER_HOST` when the host is remote.
fn docker_env(docker_host: &str) -> Vec<(String, String)> {
    let mut envs: Vec<(String, String)> = std::env::vars().collect();
    if !docker_host.is_empty()
        && docker_host != "unix:///var/run/docker.sock"
        && !docker_host.starts_with("unix://")
    {
        envs.push(("DOCKER_HOST".into(), docker_host.to_string()));
    }
    envs
}

async fn run_compose(
    docker_host: &str,
    stack: &ComposeStack,
    args: &[&str],
) -> Result<String, String> {
    let path = write_compose_file(stack)?;
    let dir = project_dir(stack);
    let envs = docker_env(docker_host);

    let mut cmd = Command::new("docker");
    cmd.arg("compose")
        .arg("-f")
        .arg(&path)
        .arg("-p")
        .arg(&stack.name)
        .args(args)
        .current_dir(&dir)
        .envs(envs)
        .stdout(Stdio::piped())
        .stderr(Stdio::piped());

    let output = cmd
        .output()
        .await
        .map_err(|e| format!("failed to run docker compose: {e}"))?;

    let mut text = String::from_utf8_lossy(&output.stdout).to_string();
    if !output.stderr.is_empty() {
        text.push('\n');
        text.push_str(&String::from_utf8_lossy(&output.stderr));
    }

    if !output.status.success() {
        return Err(text);
    }

    Ok(text)
}

pub async fn up(docker_host: &str, stack: &ComposeStack) -> Result<String, String> {
    run_compose(
        docker_host,
        stack,
        &["up", "-d", "--build", "--remove-orphans"],
    )
    .await
}

pub async fn down(docker_host: &str, stack: &ComposeStack) -> Result<String, String> {
    run_compose(docker_host, stack, &["down", "--remove-orphans"]).await
}

pub async fn pull(docker_host: &str, stack: &ComposeStack) -> Result<String, String> {
    run_compose(docker_host, stack, &["pull"]).await
}
