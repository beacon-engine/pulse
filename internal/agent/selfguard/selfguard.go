// Package selfguard recognises Pulse's own containers and pods, so the agent
// refuses actions that would cut off its own connection.
package selfguard

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// Self-protection: Pulse must not be able to stop the workloads it runs on.
// Stopping the agent from the UI kills the very session the UI talks through
// (and stopping the gateway kills the UI), so the agent recognises them and
// refuses disruptive actions on them.
//
// A workload is protected when it is this agent itself (detected, no setup
// needed), or when it carries LabelRole (set on Pulse's own deployments) or
// LabelProtected="true" (opt-in for anything else you never want touched).
const (
	LabelRole      = "pulse/role"      // "agent" | "gateway"
	LabelProtected = "pulse/protected" // "true"
)

// Self identifies the workload this agent is running as.
type Self struct {
	// ContainerID is the agent's own container id when it runs under Docker or
	// Podman ("" when bare-metal or undetectable).
	ContainerID string
	// Hostname is the agent's hostname: a container's short id by default, and
	// the pod name in Kubernetes.
	Hostname string
	// PodNamespace is set when running in a Kubernetes pod.
	PodNamespace string
}

const saNamespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

// Detect works out which container/pod this process is.
func Detect() Self {
	s := Self{ContainerID: selfContainerID("/proc/self/mountinfo")}
	s.Hostname, _ = os.Hostname()
	if ns, err := os.ReadFile(saNamespaceFile); err == nil {
		s.PodNamespace = strings.TrimSpace(string(ns))
	}
	return s
}

var containerIDRe = regexp.MustCompile(`[0-9a-f]{64}`)

// selfContainerID reads the container id from the mount that backs
// /etc/hostname. Docker (…/containers/<id>/hostname) and Podman
// (…/overlay-containers/<id>/userdata/hostname) both put it in that mount's
// source path. Only that one line is trusted: with the host bind-mounted
// (HOST_ROOT) other containers' paths also appear in mountinfo.
func selfContainerID(mountinfo string) string {
	f, err := os.Open(mountinfo)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		// mountinfo: id parent major:minor root mountpoint …
		if len(fields) < 5 || fields[4] != "/etc/hostname" {
			continue
		}
		if id := containerIDRe.FindString(fields[3]); id != "" {
			return id
		}
	}
	return ""
}

// ContainerProtection returns why a container is protected, or "".
func (s Self) ContainerProtection(id string, labels map[string]string) string {
	if s.ContainerID != "" && id == s.ContainerID {
		return "this is the Pulse agent serving this environment"
	}
	// Fallback when mountinfo is unavailable: Docker's default hostname is the
	// container's 12-char short id.
	if s.ContainerID == "" && len(s.Hostname) == 12 && strings.HasPrefix(id, s.Hostname) {
		return "this is the Pulse agent serving this environment"
	}
	return LabelProtection(labels)
}

// PodProtection returns why a pod is protected, or "".
func (s Self) PodProtection(namespace, name string, labels map[string]string) string {
	if s.PodNamespace != "" && namespace == s.PodNamespace && name == s.Hostname {
		return "this is the Pulse agent serving this environment"
	}
	return LabelProtection(labels)
}

func LabelProtection(labels map[string]string) string {
	switch labels[LabelRole] {
	case "agent":
		return "this is a Pulse agent"
	case "gateway":
		return "this is the Pulse gateway"
	}
	if strings.EqualFold(labels[LabelProtected], "true") {
		return "marked " + LabelProtected + "=true"
	}
	return ""
}

// Labels the kubelet's Docker shim (dockershim / cri-dockerd) puts on the
// containers it creates.
const (
	labelK8sNamespace = "io.kubernetes.pod.namespace"
	labelK8sPod       = "io.kubernetes.pod.name"
	labelK8sContainer = "io.kubernetes.container.name"
)

// KubeletRef reports the pod a container was created for, or nil when the
// container isn't managed by a kubelet.
func KubeletRef(labels map[string]string) *protocol.KubeletRef {
	pod, ns := labels[labelK8sPod], labels[labelK8sNamespace]
	if pod == "" || ns == "" {
		return nil
	}
	c := labels[labelK8sContainer]
	if c == "" {
		c = "POD" // the sandbox container carries no container name
	}
	return &protocol.KubeletRef{Namespace: ns, Pod: pod, Container: c}
}

func ProtectedError(name, reason string) protocol.Response {
	return protocol.RespError(403, fmt.Sprintf("%s is protected (%s): Pulse blocks actions that could cut off its own connection", name, reason))
}
