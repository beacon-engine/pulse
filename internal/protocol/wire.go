// Ports protocol/src/wire.rs — the gateway <-> agent protocol.
//
// Rust serde serializes enums *externally tagged*: a unit variant is a bare JSON
// string ("Ping"), and a data-carrying variant is a single-key object keyed by
// the variant name ({"ListContainers":{"docker_host":"…"}}). Go has no tagged
// enums, so Request/Response carry a Kind plus payload and implement custom
// JSON (un)marshaling that reproduces that exact shape for wire compatibility
// with the existing Rust gateway.
package protocol

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// ComposeStack is everything the agent needs to run `docker compose` for a
// stack without a database of its own.
type ComposeStack struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Content string `json:"content"`
	// EnvVars is a JSON-encoded []{key,value}, written to a .env file by the agent.
	EnvVars *string `json:"env_vars"`
}

// EventFrame is a single Docker lifecycle event streamed for WatchEvents.
type EventFrame struct {
	ContainerID   string `json:"container_id"`
	ContainerName string `json:"container_name"`
	// Action is the raw Docker action: die, oom, start, stop, restart, …
	Action   string  `json:"action"`
	ExitCode *string `json:"exit_code"`
}

// EventStreamID is the reserved correlation id for unsolicited event pushes.
// Real request ids start at 1, so events never collide with a reply.
const EventStreamID uint64 = 0

// ── Request ──────────────────────────────────────────────────────────────────

// Request is a gateway->agent request. Kind names the variant; only the fields
// listed for that variant in reqFields are meaningful. Dispatch on Kind.
type Request struct {
	Kind string

	DockerHost     string
	ID             string
	Ports          []PortMap
	Pull           bool
	Env            []string
	Cmd            []string
	Tail           uint32
	NetworkID      string
	ContainerID    string
	Stack          *ComposeStack
	ServiceID      string
	TargetReplicas uint64
	Namespace      string
	Deployment     string
	Name           string
	// Kubernetes: the container within a pod (logs) and the workload kind
	// (Deployment | StatefulSet | DaemonSet) for scale/restart.
	Container    string
	WorkloadKind string
}

// reqPayload is the union of every request field, with wire tags. It is
// marshaled once and filtered per-variant, and unmarshaled directly (unknown
// keys are ignored, so one decode covers every variant).
type reqPayload struct {
	DockerHost     string        `json:"docker_host"`
	ID             string        `json:"id"`
	Ports          []PortMap     `json:"ports"`
	Pull           bool          `json:"pull"`
	Env            []string      `json:"env"`
	Cmd            []string      `json:"cmd"`
	Tail           uint32        `json:"tail"`
	NetworkID      string        `json:"network_id"`
	ContainerID    string        `json:"container_id"`
	Stack          *ComposeStack `json:"stack"`
	ServiceID      string        `json:"service_id"`
	TargetReplicas uint64        `json:"target_replicas"`
	Namespace      string        `json:"namespace"`
	Deployment     string        `json:"deployment"`
	Name           string        `json:"name"`
	Container      string        `json:"container"`
	WorkloadKind   string        `json:"workload_kind"`
}

// Request variant names.
const (
	ReqPing                = "Ping"
	ReqCapabilities        = "Capabilities"
	ReqListContainers      = "ListContainers"
	ReqInspectContainer    = "InspectContainer"
	ReqInspectRawContainer = "InspectRawContainer"
	ReqRecreateContainer   = "RecreateContainer"
	ReqRecreateLatest      = "RecreateLatest"
	ReqUpdateContainerEnv  = "UpdateContainerEnv"
	ReqExecContainer       = "ExecContainer"
	ReqStartContainer      = "StartContainer"
	ReqStopContainer       = "StopContainer"
	ReqRestartContainer    = "RestartContainer"
	ReqContainerLogs       = "ContainerLogs"
	ReqContainerNetworks   = "ContainerNetworks"
	ReqSystemInfo          = "SystemInfo"
	ReqHardwareMetrics     = "HardwareMetrics"
	ReqListImages          = "ListImages"
	ReqDeleteImage         = "DeleteImage"
	ReqPruneImages         = "PruneImages"
	ReqListVolumes         = "ListVolumes"
	ReqDeleteVolume        = "DeleteVolume"
	ReqPruneVolumes        = "PruneVolumes"
	ReqListHostNetworks    = "ListHostNetworks"
	ReqNetworkConnect      = "NetworkConnect"
	ReqNetworkDisconnect   = "NetworkDisconnect"
	ReqListDetectedStacks  = "ListDetectedStacks"
	ReqComposeUp           = "ComposeUp"
	ReqComposeDown         = "ComposeDown"
	ReqComposePull         = "ComposePull"
	ReqListSwarmServices   = "ListSwarmServices"
	ReqListSwarmNodes      = "ListSwarmNodes"
	ReqListSwarmTasks      = "ListSwarmTasks"
	ReqUpdateSwarmService  = "UpdateSwarmService"
	ReqScaleSwarmService   = "ScaleSwarmService"
	ReqK8sPods             = "K8sPods"
	ReqK8sNamespaces       = "K8sNamespaces"
	ReqK8sRollout          = "K8sRollout"
	ReqK8sDeletePod        = "K8sDeletePod"
	ReqK8sWorkloads        = "K8sWorkloads"
	ReqK8sPodLogs          = "K8sPodLogs"
	ReqK8sScale            = "K8sScale"
	ReqK8sRestart          = "K8sRestart"
	ReqWatchEvents         = "WatchEvents"
)

// reqUnit are the variants with no payload (serialized as a bare string).
var reqUnit = map[string]bool{
	ReqPing:            true,
	ReqHardwareMetrics: true,
	ReqK8sPods:         true,
	ReqK8sNamespaces:   true,
	ReqK8sWorkloads:    true,
}

// reqFields maps each data-carrying variant to the wire field names it emits.
var reqFields = map[string][]string{
	ReqCapabilities:        {"docker_host"},
	ReqListContainers:      {"docker_host"},
	ReqInspectContainer:    {"docker_host", "id"},
	ReqInspectRawContainer: {"docker_host", "id"},
	ReqRecreateContainer:   {"docker_host", "id", "ports"},
	ReqRecreateLatest:      {"docker_host", "id", "pull"},
	ReqUpdateContainerEnv:  {"docker_host", "id", "env"},
	ReqExecContainer:       {"docker_host", "id", "cmd"},
	ReqStartContainer:      {"docker_host", "id"},
	ReqStopContainer:       {"docker_host", "id"},
	ReqRestartContainer:    {"docker_host", "id"},
	ReqContainerLogs:       {"docker_host", "id", "tail"},
	ReqContainerNetworks:   {"docker_host", "id"},
	ReqSystemInfo:          {"docker_host"},
	ReqListImages:          {"docker_host"},
	ReqDeleteImage:         {"docker_host", "id"},
	ReqPruneImages:         {"docker_host"},
	ReqListVolumes:         {"docker_host"},
	ReqDeleteVolume:        {"docker_host", "name"},
	ReqPruneVolumes:        {"docker_host"},
	ReqListHostNetworks:    {"docker_host"},
	ReqNetworkConnect:      {"docker_host", "network_id", "container_id"},
	ReqNetworkDisconnect:   {"docker_host", "network_id", "container_id"},
	ReqListDetectedStacks:  {"docker_host"},
	ReqComposeUp:           {"docker_host", "stack"},
	ReqComposeDown:         {"docker_host", "stack"},
	ReqComposePull:         {"docker_host", "stack"},
	ReqListSwarmServices:   {"docker_host"},
	ReqListSwarmNodes:      {"docker_host"},
	ReqListSwarmTasks:      {"docker_host", "service_id"},
	ReqUpdateSwarmService:  {"docker_host", "service_id"},
	ReqScaleSwarmService:   {"docker_host", "service_id", "target_replicas"},
	ReqK8sRollout:          {"namespace", "deployment"},
	ReqK8sDeletePod:        {"namespace", "name"},
	ReqK8sPodLogs:          {"namespace", "name", "container", "tail"},
	ReqK8sScale:            {"namespace", "workload_kind", "name", "target_replicas"},
	ReqK8sRestart:          {"namespace", "workload_kind", "name"},
	ReqWatchEvents:         {"docker_host"},
}

func (r Request) MarshalJSON() ([]byte, error) {
	if reqUnit[r.Kind] {
		return json.Marshal(r.Kind)
	}
	fields, ok := reqFields[r.Kind]
	if !ok {
		return nil, fmt.Errorf("protocol: unknown Request kind %q", r.Kind)
	}
	full, err := json.Marshal(reqPayload{
		DockerHost: r.DockerHost, ID: r.ID, Ports: r.Ports, Pull: r.Pull,
		Env: r.Env, Cmd: r.Cmd, Tail: r.Tail, NetworkID: r.NetworkID,
		ContainerID: r.ContainerID, Stack: r.Stack, ServiceID: r.ServiceID,
		TargetReplicas: r.TargetReplicas, Namespace: r.Namespace,
		Deployment: r.Deployment, Name: r.Name,
		Container: r.Container, WorkloadKind: r.WorkloadKind,
	})
	if err != nil {
		return nil, err
	}
	var all map[string]json.RawMessage
	if err := json.Unmarshal(full, &all); err != nil {
		return nil, err
	}
	inner := make(map[string]json.RawMessage, len(fields))
	for _, k := range fields {
		if v, ok := all[k]; ok && string(v) != "null" {
			inner[k] = v
		}
	}
	return json.Marshal(map[string]any{r.Kind: inner})
}

func (r *Request) UnmarshalJSON(b []byte) error {
	// Unit variant: a bare JSON string.
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		r.Kind = s
		return nil
	}
	// Data variant: a single-key object {Kind: {fields…}}.
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(b, &obj); err != nil {
		return err
	}
	if len(obj) != 1 {
		return fmt.Errorf("protocol: expected single-key Request object, got %d keys", len(obj))
	}
	for kind, raw := range obj {
		var p reqPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		r.Kind = kind
		r.DockerHost = p.DockerHost
		r.ID = p.ID
		r.Ports = p.Ports
		r.Pull = p.Pull
		r.Env = p.Env
		r.Cmd = p.Cmd
		r.Tail = p.Tail
		r.NetworkID = p.NetworkID
		r.ContainerID = p.ContainerID
		r.Stack = p.Stack
		r.ServiceID = p.ServiceID
		r.TargetReplicas = p.TargetReplicas
		r.Namespace = p.Namespace
		r.Deployment = p.Deployment
		r.Name = p.Name
		r.Container = p.Container
		r.WorkloadKind = p.WorkloadKind
	}
	return nil
}

// ── Response ─────────────────────────────────────────────────────────────────

// Response is an agent->gateway reply. Kind names the variant; Payload holds the
// inner value (nil for the unit variants Ok/Pong). Build one with the Resp*
// constructors below.
type Response struct {
	Kind    string
	Payload any
}

type errorBody struct {
	Status  uint16 `json:"status"`
	Message string `json:"message"`
}

// Response variant names.
const (
	RespKindOk                = "Ok"
	RespKindPong              = "Pong"
	RespKindCapabilities      = "Capabilities"
	RespKindContainers        = "Containers"
	RespKindContainerDetail   = "ContainerDetail"
	RespKindRaw               = "Raw"
	RespKindID                = "Id"
	RespKindText              = "Text"
	RespKindCount             = "Count"
	RespKindSystemInfo        = "SystemInfo"
	RespKindHardwareMetrics   = "HardwareMetrics"
	RespKindImages            = "Images"
	RespKindVolumes           = "Volumes"
	RespKindHostNetworks      = "HostNetworks"
	RespKindContainerNetworks = "ContainerNetworks"
	RespKindDetectedStacks    = "DetectedStacks"
	RespKindSwarmServices     = "SwarmServices"
	RespKindSwarmNodes        = "SwarmNodes"
	RespKindSwarmTasks        = "SwarmTasks"
	RespKindPods              = "Pods"
	RespKindNamespaces        = "Namespaces"
	RespKindWorkloads         = "Workloads"
	RespKindEvent             = "Event"
	RespKindError             = "Error"
	// RespKindHeartbeat is an agent->gateway liveness ping pushed on
	// EventStreamID (push sessions only). The gateway bumps the environment's
	// last_seen; it is never a reply to a request.
	RespKindHeartbeat = "Heartbeat"
	// RespKindDeregister is an agent->gateway notice, pushed on EventStreamID
	// just before a clean shutdown (SIGTERM / scale-down), that the agent is
	// going away on purpose. The gateway marks the environment offline
	// *gracefully* so no "agent down" alert fires.
	RespKindDeregister = "Deregister"
)

// Response constructors (Resp* to avoid clashing with the data type names).
func RespOk() Response                               { return Response{Kind: RespKindOk} }
func RespPong() Response                             { return Response{Kind: RespKindPong} }
func RespHeartbeat() Response                        { return Response{Kind: RespKindHeartbeat} }
func RespDeregister() Response                       { return Response{Kind: RespKindDeregister} }
func RespCapabilities(v Capabilities) Response       { return Response{RespKindCapabilities, v} }
func RespContainers(v []ContainerInfo) Response      { return Response{RespKindContainers, v} }
func RespContainerDetail(v ContainerDetail) Response { return Response{RespKindContainerDetail, v} }
func RespRaw(v json.RawMessage) Response             { return Response{RespKindRaw, v} }
func RespID(v string) Response                       { return Response{RespKindID, v} }
func RespText(v string) Response                     { return Response{RespKindText, v} }
func RespCount(v int64) Response                     { return Response{RespKindCount, v} }
func RespSystemInfo(v SystemInfo) Response           { return Response{RespKindSystemInfo, v} }
func RespHardwareMetrics(v HardwareMetrics) Response { return Response{RespKindHardwareMetrics, v} }
func RespImages(v []ImageInfo) Response              { return Response{RespKindImages, v} }
func RespVolumes(v []VolumeInfo) Response            { return Response{RespKindVolumes, v} }
func RespHostNetworks(v []HostNetwork) Response      { return Response{RespKindHostNetworks, v} }
func RespContainerNetworks(v []ContainerNetworkInfo) Response {
	return Response{RespKindContainerNetworks, v}
}
func RespDetectedStacks(v []Stack) Response       { return Response{RespKindDetectedStacks, v} }
func RespSwarmServices(v []SwarmService) Response { return Response{RespKindSwarmServices, v} }
func RespSwarmNodes(v []SwarmNode) Response       { return Response{RespKindSwarmNodes, v} }
func RespSwarmTasks(v []SwarmTask) Response       { return Response{RespKindSwarmTasks, v} }
func RespPods(v []PodInfo) Response               { return Response{RespKindPods, v} }
func RespNamespaces(v []NamespaceInfo) Response   { return Response{RespKindNamespaces, v} }
func RespWorkloads(v []WorkloadInfo) Response     { return Response{RespKindWorkloads, v} }
func RespEvent(v EventFrame) Response             { return Response{RespKindEvent, v} }

// RespError builds an error reply. status is an HTTP status code the gateway relays.
func RespError(status uint16, message string) Response {
	return Response{RespKindError, errorBody{Status: status, Message: message}}
}

func (r Response) MarshalJSON() ([]byte, error) {
	if r.Payload == nil {
		return json.Marshal(r.Kind) // unit variant: bare string
	}
	return json.Marshal(map[string]any{r.Kind: r.Payload})
}

func (r *Response) UnmarshalJSON(b []byte) error {
	// Unit variant: a bare JSON string.
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		r.Kind = s
		r.Payload = nil
		return nil
	}
	// Data variant: {Kind: payload}. The payload is kept raw for the caller to
	// decode into the concrete type implied by Kind.
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(b, &obj); err != nil {
		return err
	}
	if len(obj) != 1 {
		return fmt.Errorf("protocol: expected single-key Response object, got %d keys", len(obj))
	}
	for kind, raw := range obj {
		r.Kind = kind
		r.Payload = raw
	}
	return nil
}

// AsError reports the status and message of an Error response. ok is false for
// non-error responses. Used by the gateway to map an agent error onto HTTP.
func (r Response) AsError() (status uint16, message string, ok bool) {
	if r.Kind != RespKindError {
		return 0, "", false
	}
	switch p := r.Payload.(type) {
	case errorBody:
		return p.Status, p.Message, true
	case json.RawMessage:
		var e errorBody
		if json.Unmarshal(p, &e) == nil {
			return e.Status, e.Message, true
		}
	}
	return 500, "agent error", true
}

// Decode unmarshals a data response's payload into target. A unit response (nil
// payload) is a no-op. Handles both a decoded payload (json.RawMessage, as the
// gateway receives from an agent) and a constructed one (a typed value).
func (r Response) Decode(target any) error {
	switch p := r.Payload.(type) {
	case nil:
		return nil
	case json.RawMessage:
		return json.Unmarshal(p, target)
	default:
		b, err := json.Marshal(p)
		if err != nil {
			return err
		}
		return json.Unmarshal(b, target)
	}
}

// ── Correlation frames ───────────────────────────────────────────────────────

// RequestFrame is a gateway->agent request tagged with a correlation id for the
// multiplexed WebSocket session.
type RequestFrame struct {
	ID  uint64  `json:"id"`
	Req Request `json:"req"`
}

// ResponseFrame is an agent->gateway message. For a reply, ID echoes the
// originating RequestFrame.ID; for a pushed event it is EventStreamID.
type ResponseFrame struct {
	ID   uint64   `json:"id"`
	Resp Response `json:"resp"`
}

// ── Length-prefixed framing (the TCP pull path) ──────────────────────────────

// MaxFrame guards against a corrupt/hostile length prefix.
const MaxFrame = 64 * 1024 * 1024

// ErrFrameTooLarge is returned when a length prefix exceeds MaxFrame. In practice
// this usually means a non-protocol client (an HTTP request, a k8s httpGet probe,
// or a Docker client pointed at this port) spoke to the binary protocol port.
var ErrFrameTooLarge = errors.New("frame exceeds maximum size")

// WriteFrame writes a length-prefixed JSON frame: a big-endian uint32 length
// followed by that many bytes of JSON.
func WriteFrame(w io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if len(b) > MaxFrame {
		return ErrFrameTooLarge
	}
	var lb [4]byte
	binary.BigEndian.PutUint32(lb[:], uint32(len(b)))
	if _, err := w.Write(lb[:]); err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

// ReadFrame reads a single length-prefixed JSON frame into v. It returns io.EOF
// when the peer closes cleanly between frames.
func ReadFrame(r io.Reader, v any) error {
	var lb [4]byte
	if _, err := io.ReadFull(r, lb[:]); err != nil {
		return err
	}
	n := binary.BigEndian.Uint32(lb[:])
	if int64(n) > MaxFrame {
		return ErrFrameTooLarge
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return err
	}
	return json.Unmarshal(buf, v)
}
