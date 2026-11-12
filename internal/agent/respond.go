package agent

import (
	"github.com/moby/moby/client"

	"github.com/beacon-engine/pulse/internal/agent/docker"
	"github.com/beacon-engine/pulse/internal/protocol"
)

// Helpers that turn an operation's result into the protocol response Dispatch
// returns (status codes match the original Rust agent's conn_error / exec_error).

func connError(msg string) protocol.Response {
	return protocol.RespError(502, "cannot connect to Docker: "+msg)
}

// dockerOp resolves the Docker client for host (502 on failure) then runs fn.
func dockerOp(st *State, host string, fn func(*client.Client) protocol.Response) protocol.Response {
	cli, err := st.Docker.ClientFor(host)
	if err != nil {
		return connError(err.Error())
	}
	return fn(cli)
}

// Result → Response helpers matching the Rust dispatch's mapping.
func okOrErr(err error) protocol.Response {
	if err != nil {
		return docker.ExecError(err)
	}
	return protocol.RespOk()
}

func idOrErr(id string, err error) protocol.Response {
	if err != nil {
		return docker.ExecError(err)
	}
	return protocol.RespID(id)
}

func countOrErr(n int64, err error) protocol.Response {
	if err != nil {
		return docker.ExecError(err)
	}
	return protocol.RespCount(n)
}

func textOrErr(out string, err error) protocol.Response {
	if err != nil {
		return docker.ExecError(err)
	}
	return protocol.RespText(out)
}
