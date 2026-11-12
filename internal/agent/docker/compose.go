package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// Ports agent/src/compose.rs — runs `docker compose` for a stack. The agent is
// stateless: the gateway forwards the persisted stack content and env, which we
// materialize to a temp project dir and shell out to the compose CLI.

func composeProjectDir(stack *protocol.ComposeStack) string {
	return filepath.Join(os.TempDir(), "pulse-stacks", stack.ID)
}

// writeComposeFile materializes the compose file (and a .env for ${VAR}
// substitution) into the project dir, returning the compose file path.
func writeComposeFile(stack *protocol.ComposeStack) (string, error) {
	dir := composeProjectDir(stack)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create stack dir: %w", err)
	}

	// Write .env so `docker compose` can substitute ${VAR} references.
	if stack.EnvVars != nil {
		var vars []map[string]any
		if err := json.Unmarshal([]byte(*stack.EnvVars), &vars); err == nil {
			var b strings.Builder
			for _, v := range vars {
				key, _ := v["key"].(string)
				if key == "" {
					continue
				}
				val, _ := v["value"].(string)
				fmt.Fprintf(&b, "%s=%s\n", key, val)
			}
			if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(b.String()), 0o644); err != nil {
				return "", fmt.Errorf("write .env file: %w", err)
			}
		}
	}

	path := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(path, []byte(stack.Content), 0o644); err != nil {
		return "", fmt.Errorf("write compose file: %w", err)
	}
	return path, nil
}

// composeEnv points the compose child at the target daemon via DOCKER_HOST when
// the host is remote, inheriting the rest of the environment.
func composeEnv(dockerHost string) []string {
	env := os.Environ()
	if dockerHost != "" && dockerHost != "unix:///var/run/docker.sock" && !strings.HasPrefix(dockerHost, "unix://") {
		env = append(env, "DOCKER_HOST="+dockerHost)
	}
	return env
}

func runCompose(ctx context.Context, dockerHost string, stack *protocol.ComposeStack, args ...string) (string, error) {
	path, err := writeComposeFile(stack)
	if err != nil {
		return "", err
	}
	dir := composeProjectDir(stack)

	full := append([]string{"compose", "-f", path, "-p", stack.Name}, args...)
	cmd := exec.CommandContext(ctx, "docker", full...)
	cmd.Dir = dir
	cmd.Env = composeEnv(dockerHost)

	out, err := cmd.CombinedOutput()
	text := string(out)
	if err != nil {
		// Non-zero exit → surface the combined output as the error.
		if text != "" {
			return "", fmt.Errorf("%s", text)
		}
		return "", err
	}
	return text, nil
}

func ComposeUp(ctx context.Context, dockerHost string, stack *protocol.ComposeStack) (string, error) {
	return runCompose(ctx, dockerHost, stack, "up", "-d", "--build", "--remove-orphans")
}

func ComposeDown(ctx context.Context, dockerHost string, stack *protocol.ComposeStack) (string, error) {
	return runCompose(ctx, dockerHost, stack, "down", "--remove-orphans")
}

func ComposePull(ctx context.Context, dockerHost string, stack *protocol.ComposeStack) (string, error) {
	return runCompose(ctx, dockerHost, stack, "pull")
}
