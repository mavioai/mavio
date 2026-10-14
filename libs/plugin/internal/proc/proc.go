// Package proc defines the protocol between the host and child-process
// plugins: how the host passes the socket path and token, and the handshake
// line the plugin prints once it is listening.
package proc

// Environment variables set by the host for the child process.
const (
	EnvSocket = "MAVIO_PLUGIN_SOCKET"
	EnvToken  = "MAVIO_PLUGIN_TOKEN"
	// EnvHostAddress names a loopback TCP address also serving the host
	// API on Windows, for plugins whose runtime has no Unix sockets there.
	// Requests on it, as on EnvHostSocket, carry the token.
	EnvHostAddress = "MAVIO_HOST_ADDRESS"
	// EnvHostSocket names the socket serving the host API. Requests on it
	// present the plugin's token, as the host's requests do.
	EnvHostSocket = "MAVIO_HOST_SOCKET"
	// EnvData names the plugin's writable data folder, in both runtimes.
	EnvData = "MAVIO_PLUGIN_DATA"
	// EnvHostOS names the host's operating system and its version, in
	// both runtimes, as Servarr applications name them in their
	// User-Agent, such as "ubuntu 22.04".
	EnvHostOS = "MAVIO_HOST_OS"
)

// ProtocolVersion is the handshake protocol version.
const ProtocolVersion = 1

// Handshake is printed by the plugin as one JSON line on stdout when it is
// ready to serve requests.
type Handshake struct {
	Protocol int    `json:"mavio_plugin"`
	PluginID string `json:"plugin_id"`
	// Address is a loopback TCP address, host:port, the plugin serves on
	// instead of the socket: runtimes such as Node's and Bun's have no
	// Unix sockets on Windows.
	Address string `json:"address,omitempty"`
}

// AuthScheme prefixes the token in the Authorization header.
const AuthScheme = "Bearer "

// TokenHeader carries the token of requests to HTTP routes instead, which
// keep their own Authorization header.
const TokenHeader = "Mavio-Plugin-Token"

// ShutdownPath is the RPC after which a process plugin exits.
const ShutdownPath = "/mavio.plugin.v1.PluginService/Shutdown"
