package protocol

import (
	"encoding/json"
	"testing"
)

// These lock the JSON shapes to what the Rust `serde` externally-tagged encoding
// produces, so a Go agent stays wire-compatible with the existing Rust gateway.

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func TestRequestMarshalShapes(t *testing.T) {
	cases := []struct {
		req  Request
		want string
	}{
		{Request{Kind: ReqPing}, `"Ping"`},
		{Request{Kind: ReqHardwareMetrics}, `"HardwareMetrics"`},
		{Request{Kind: ReqK8sPods}, `"K8sPods"`},
		{Request{Kind: ReqCapabilities, DockerHost: "h"}, `{"Capabilities":{"docker_host":"h"}}`},
		{Request{Kind: ReqListContainers, DockerHost: "h"}, `{"ListContainers":{"docker_host":"h"}}`},
		{Request{Kind: ReqListContainers}, `{"ListContainers":{"docker_host":""}}`}, // empty host = local, must emit
		{Request{Kind: ReqInspectContainer, DockerHost: "h", ID: "c1"}, `{"InspectContainer":{"docker_host":"h","id":"c1"}}`},
		{Request{Kind: ReqContainerLogs, ID: "c1", Tail: 100}, `{"ContainerLogs":{"docker_host":"","id":"c1","tail":100}}`},
		{Request{Kind: ReqK8sRollout, Namespace: "ns", Deployment: "api"}, `{"K8sRollout":{"deployment":"api","namespace":"ns"}}`},
		{Request{Kind: ReqK8sWorkloads}, `"K8sWorkloads"`},
		{Request{Kind: ReqK8sNodes}, `"K8sNodes"`},
		{Request{Kind: ReqK8sServices}, `"K8sServices"`},
		{Request{Kind: ReqK8sEvents}, `"K8sEvents"`},
		{Request{Kind: ReqK8sPodLogs, Namespace: "ns", Name: "web-1", Container: "app", Tail: 50}, `{"K8sPodLogs":{"container":"app","name":"web-1","namespace":"ns","tail":50}}`},
		{Request{Kind: ReqK8sScale, Namespace: "ns", WorkloadKind: "Deployment", Name: "api", TargetReplicas: 3}, `{"K8sScale":{"name":"api","namespace":"ns","target_replicas":3,"workload_kind":"Deployment"}}`},
		{Request{Kind: ReqK8sRestart, Namespace: "ns", WorkloadKind: "StatefulSet", Name: "db"}, `{"K8sRestart":{"name":"db","namespace":"ns","workload_kind":"StatefulSet"}}`},
		{Request{Kind: ReqScaleSwarmService, ServiceID: "s", TargetReplicas: 3}, `{"ScaleSwarmService":{"docker_host":"","service_id":"s","target_replicas":3}}`},
	}
	for _, c := range cases {
		got := mustJSON(t, c.req)
		// Field order within the inner object is not guaranteed by encoding/json
		// map marshaling to match byte-for-byte, so compare by re-decoding.
		if !jsonEqual(t, got, c.want) {
			t.Errorf("Request %s => %s, want %s", c.req.Kind, got, c.want)
		}
	}
}

func TestRequestRoundTrip(t *testing.T) {
	// Simulate a request as the Rust gateway would send it, decode, re-encode.
	in := `{"ScaleSwarmService":{"docker_host":"","service_id":"svc","target_replicas":5}}`
	var r Request
	if err := json.Unmarshal([]byte(in), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if r.Kind != ReqScaleSwarmService || r.ServiceID != "svc" || r.TargetReplicas != 5 {
		t.Fatalf("decoded wrong: %+v", r)
	}
	if !jsonEqual(t, mustJSON(t, r), in) {
		t.Errorf("round-trip mismatch: %s vs %s", mustJSON(t, r), in)
	}

	var ping Request
	if err := json.Unmarshal([]byte(`"Ping"`), &ping); err != nil || ping.Kind != ReqPing {
		t.Fatalf("unit decode failed: %v %+v", err, ping)
	}
}

func TestResponseMarshalShapes(t *testing.T) {
	if got := mustJSON(t, RespOk()); got != `"Ok"` {
		t.Errorf("RespOk => %s", got)
	}
	if got := mustJSON(t, RespPong()); got != `"Pong"` {
		t.Errorf("RespPong => %s", got)
	}
	if got := mustJSON(t, RespID("abc")); got != `{"Id":"abc"}` {
		t.Errorf("RespID => %s", got)
	}
	if got := mustJSON(t, RespText("hello")); got != `{"Text":"hello"}` {
		t.Errorf("RespText => %s", got)
	}
	if got := mustJSON(t, RespCount(42)); got != `{"Count":42}` {
		t.Errorf("RespCount => %s", got)
	}
	if got := mustJSON(t, RespError(502, "boom")); !jsonEqual(t, got, `{"Error":{"status":502,"message":"boom"}}`) {
		t.Errorf("RespError => %s", got)
	}
	stack := "prod"
	if got := mustJSON(t, RespContainers([]ContainerInfo{{ID: "c1", Name: "web", Stack: &stack}})); !jsonEqual(
		t, got, `{"Containers":[{"id":"c1","name":"web","image":"","status":"","state":"","stack":"prod"}]}`) {
		t.Errorf("RespContainers => %s", got)
	}
}

func TestFrameRoundTrip(t *testing.T) {
	rf := RequestFrame{ID: 7, Req: Request{Kind: ReqListContainers, DockerHost: "h"}}
	if !jsonEqual(t, mustJSON(t, rf), `{"id":7,"req":{"ListContainers":{"docker_host":"h"}}}`) {
		t.Errorf("RequestFrame => %s", mustJSON(t, rf))
	}
	respf := ResponseFrame{ID: EventStreamID, Resp: RespEvent(EventFrame{ContainerName: "web", Action: "die"})}
	if !jsonEqual(t, mustJSON(t, respf),
		`{"id":0,"resp":{"Event":{"container_id":"","container_name":"web","action":"die","exit_code":null}}}`) {
		t.Errorf("ResponseFrame => %s", mustJSON(t, respf))
	}
}

// jsonEqual compares two JSON documents structurally (order-independent).
func jsonEqual(t *testing.T, a, b string) bool {
	t.Helper()
	var av, bv any
	if err := json.Unmarshal([]byte(a), &av); err != nil {
		t.Fatalf("bad json a: %v (%s)", err, a)
	}
	if err := json.Unmarshal([]byte(b), &bv); err != nil {
		t.Fatalf("bad json b: %v (%s)", err, b)
	}
	return jsonDeepEqual(av, bv)
}

func jsonDeepEqual(a, b any) bool {
	ab, _ := json.Marshal(canon(a))
	bb, _ := json.Marshal(canon(b))
	return string(ab) == string(bb)
}

// canon recursively sorts map keys by re-marshaling (encoding/json already sorts
// map keys), so structural comparison ignores object key order.
func canon(v any) any { return v }
