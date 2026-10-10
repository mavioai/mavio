// Package proc defines the protocol between the host and child-process
// plugins: how the host passes the socket path and token, and the handshake
// line the plugin prints once it is listening.
package proc

// Environment variables set by the host for the child process.
const (
	EnvSocket = "MAVIO_PLUGIN_SOCKET"
	EnvToken  = "MAVIO_PLUGIN_TOKEN"
	// EnvHostSocket names the socket serving the host API. Requests on it
	// present the plugin's token, as the host's requests do.
	EnvHostSocket = "MAVIO_HOST_SOCKET"
)

// ProtocolVersion is the handshake protocol version.
const ProtocolVersion = 1

// Handshake is printed by the plugin as one JSON line on stdout when it is
// ready to serve requests.
type Handshake struct {
	Protocol int    `json:"mavio_plugin"`
	PluginID string `json:"plugin_id"`
}

// AuthScheme prefixes the token in the Authorization header.
const AuthScheme = "Bearer "

// ShutdownPath is the RPC after which a process plugin exits.
const ShutdownPath = "/mavio.plugin.v1.PluginService/Shutdown"
