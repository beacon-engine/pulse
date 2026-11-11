package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSelfContainerIDFromHostnameMount(t *testing.T) {
	self := "0f3c7e2a9b1d4c5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6"
	other := "aaaabbbbccccddddeeeeffff00001111222233334444555566667777888899990"[:64]
	// With HOST_ROOT bind-mounted, other containers' paths appear too; only the
	// /etc/hostname line identifies us.
	mountinfo := "" +
		"1 0 0:1 / / rw - overlay overlay rw\n" +
		"2 1 0:2 /var/lib/docker/containers/" + other + "/mounts/shm /host/var/lib/docker/containers/" + other + "/mounts/shm rw - tmpfs shm rw\n" +
		"3 1 254:1 /docker/containers/" + self + "/hostname /etc/hostname rw - ext4 /dev/vda1 rw\n"
	path := filepath.Join(t.TempDir(), "mountinfo")
	if err := os.WriteFile(path, []byte(mountinfo), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := selfContainerID(path); got != self {
		t.Errorf("selfContainerID = %q, want %q", got, self)
	}

	podman := "3 1 0:3 /containers/storage/overlay-containers/" + self + "/userdata/hostname /etc/hostname rw - tmpfs tmpfs rw\n"
	if err := os.WriteFile(path, []byte(podman), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := selfContainerID(path); got != self {
		t.Errorf("podman selfContainerID = %q, want %q", got, self)
	}
}

func TestProtection(t *testing.T) {
	id := "0f3c7e2a9b1d4c5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6"
	s := Self{ContainerID: id, Hostname: "pulse-agent-7cc85d8b6f-85d84", PodNamespace: "pulse"}

	cases := []struct {
		name      string
		protected bool
		reason    string
	}{
		{"self container", true, s.containerProtection(id, nil)},
		{"gateway label", true, s.containerProtection("other", map[string]string{LabelRole: "gateway"})},
		{"opt-in label", true, s.containerProtection("other", map[string]string{LabelProtected: "true"})},
		{"ordinary container", false, s.containerProtection("other", map[string]string{"app": "web"})},
		{"self pod", true, s.podProtection("pulse", "pulse-agent-7cc85d8b6f-85d84", nil)},
		{"same name, other namespace", false, s.podProtection("demo", "pulse-agent-7cc85d8b6f-85d84", nil)},
		{"agent pod label", true, s.podProtection("other", "x", map[string]string{LabelRole: "agent"})},
		{"ordinary pod", false, s.podProtection("demo", "nginx", nil)},
	}
	for _, c := range cases {
		if (c.reason != "") != c.protected {
			t.Errorf("%s: protected = %t (reason %q), want %t", c.name, c.reason != "", c.reason, c.protected)
		}
	}

	// Bare-metal fallback: a 12-char hostname that prefixes the id.
	bare := Self{Hostname: id[:12]}
	if bare.containerProtection(id, nil) == "" {
		t.Errorf("short-id hostname fallback should protect the agent's container")
	}
}

func TestKubeletRef(t *testing.T) {
	if ref := kubeletRef(map[string]string{"com.docker.compose.project": "pulse"}); ref != nil {
		t.Errorf("plain container: %+v, want nil", ref)
	}
	ref := kubeletRef(map[string]string{
		labelK8sNamespace: "demo", labelK8sPod: "nginx-1", labelK8sContainer: "nginx",
	})
	if ref == nil || ref.Namespace != "demo" || ref.Pod != "nginx-1" || ref.Container != "nginx" {
		t.Errorf("app container: %+v", ref)
	}
	if ref := kubeletRef(map[string]string{labelK8sNamespace: "demo", labelK8sPod: "nginx-1"}); ref == nil || ref.Container != "POD" {
		t.Errorf("sandbox container: %+v, want Container=POD", ref)
	}
}
