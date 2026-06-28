use std::fs;
use std::path::PathBuf;
use std::process::Stdio;

use tokio::process::Command;

use crate::state::AppState;
use crate::storage::models::SavedStack;

fn project_dir(stack: &SavedStack) -> PathBuf {
    let mut dir = std::env::temp_dir();
    dir.push("beacon-stacks");
    dir.push(&stack.id);
    dir
}

fn write_compose_file(stack: &SavedStack) -> Result<PathBuf, String> {
    let dir = project_dir(stack);
    fs::create_dir_all(&dir).map_err(|e| format!("create stack dir: {e}"))?;
    let path = dir.join("docker-compose.yml");
    fs::write(&path, &stack.content).map_err(|e| format!("write compose file: {e}"))?;
    Ok(path)
}

async fn docker_env(state: &AppState, stack: &SavedStack) -> Result<Vec<(String, String)>, String> {
    let mut envs: Vec<(String, String)> = std::env::vars().collect();

    let host = state
        .db
        .get_environment(&stack.host_id)
        .await
        .map_err(|e| format!("load host: {e}"))?
        .ok_or_else(|| format!("host {} not found", stack.host_id))?;

    if !host.docker_host.is_empty() && host.docker_host != "unix:///var/run/docker.sock" {
        envs.push(("DOCKER_HOST".into(), host.docker_host));
    }

    Ok(envs)
}

async fn run_compose(
    state: &AppState,
    stack: &SavedStack,
    args: &[&str],
) -> Result<String, String> {
    let path = write_compose_file(stack)?;
    let dir = project_dir(stack);

    let envs = docker_env(state, stack).await?;

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

pub async fn up(state: &AppState, stack: &SavedStack) -> Result<String, String> {
    run_compose(state, stack, &["up", "-d", "--build", "--remove-orphans"]).await
}

pub async fn down(state: &AppState, stack: &SavedStack) -> Result<String, String> {
    run_compose(state, stack, &["down", "--remove-orphans"]).await
}

pub async fn pull(state: &AppState, stack: &SavedStack) -> Result<String, String> {
    run_compose(state, stack, &["pull"]).await
}
