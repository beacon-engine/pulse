package agent

import (
	"context"
	"errors"
	"testing"
)

func TestResolveIdentityPrecedence(t *testing.T) {
	ok := func(v string) func(context.Context) (string, error) {
		return func(context.Context) (string, error) { return v, nil }
	}
	down := func(context.Context) (string, error) { return "", errors.New("unreachable") }

	cases := []struct {
		name   string
		src    identitySources
		want   string
		stable bool
	}{
		{"explicit wins", identitySources{explicit: "pinned", k8sID: ok("u"), dockerID: ok("d")}, "pinned", true},
		// In a pod the agent represents the cluster, even if a daemon is reachable.
		{"in-cluster prefers cluster", identitySources{inCluster: true, k8sID: ok("6f1e-uid"), dockerID: ok("d")}, "k8s-6f1e-uid", true},
		// On a host the agent represents the daemon, even with a kubeconfig.
		{"host prefers daemon", identitySources{k8sID: ok("u"), dockerID: ok("ABCD:EF01")}, "docker-abcdef01", true},
		{"host without daemon uses kubeconfig cluster", identitySources{k8sID: ok("u"), dockerID: down}, "k8s-u", true},
		{"in-cluster falls back to daemon", identitySources{inCluster: true, k8sID: down, dockerID: ok("d")}, "docker-d", true},
		{"nothing reachable", identitySources{k8sID: down, dockerID: down}, "", false},
		{"blank id is not an identity", identitySources{k8sID: ok(" "), dockerID: down}, "", false},
	}
	for _, c := range cases {
		got, stable := resolveIdentity(context.Background(), c.src)
		if got != c.want || stable != c.stable {
			t.Errorf("%s: got (%q, %t), want (%q, %t)", c.name, got, stable, c.want, c.stable)
		}
	}
}

// Once resolved, the identity is cached for the life of the process: later
// calls never re-probe, so a runtime flapping can't change it mid-session.
func TestInstanceIDIsCached(t *testing.T) {
	st := NewState(Config{InstanceID: "pinned"})
	if a, b := st.InstanceID(context.Background()), st.InstanceID(context.Background()); a != "pinned" || b != a {
		t.Errorf("explicit identity = %q then %q, want pinned twice", a, b)
	}

	st = NewState(Config{})
	st.id.stable = "k8s-abc"
	if got := st.InstanceID(context.Background()); got != "k8s-abc" {
		t.Errorf("cached identity = %q, want k8s-abc", got)
	}
}
