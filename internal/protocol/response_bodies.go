// Ports protocol/src/protocol/response.rs — response envelopes for HTTP-API use.
package protocol

type StackOpResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Output string `json:"output"`
}

type TokenResponse struct {
	Token string `json:"token"`
}
