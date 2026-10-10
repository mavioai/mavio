// Package guest is the plugin-side SDK. A plugin registers the Connect
// handlers of the services it implements with Handle, and the runtime entry
// point — guest/wasm or guest/process — serves them:
//
//	func init() {
//		guest.Handle(pluginv1connect.NewPluginServiceHandler(&lifecycle{}))
//		guest.Handle(pluginv1connect.NewMetadataProviderServiceHandler(&provider{}))
//	}
//
// The same handler code builds for both runtimes. A plugin declaring
// CAPABILITY_HTTP_HANDLER also registers its HTTP routes with HandleHTTP.
package guest

import (
	"errors"
	"net/http"
	"os"
	"sync"

	"github.com/mavioai/mavio/libs/plugin/internal/abi"
	"github.com/mavioai/mavio/libs/plugin/internal/proc"
)

// HostURL is the base URL of the host API: the server's Connect services and
// its plain HTTP routes, reached with the client HostClient returns.
const HostURL = "http://" + abi.HostName

// Headers of host API requests; see HostClient.
const (
	// UserHeader names the user a request acts as.
	UserHeader = "Mavio-User"
	// DeviceHeader names the device, among a device controller's, that a
	// request acting as a user acts as.
	DeviceHeader = "Mavio-Device"
)

var (
	mu         sync.RWMutex
	mux        = http.NewServeMux()
	httpClient = http.DefaultClient
	hostClient = &http.Client{Transport: noHost{}}
)

type noHost struct{}

func (noHost) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("guest: the host API is not available outside a Mavio host")
}

// Handle registers the handler for a path, in the form returned by the
// generated New…ServiceHandler constructors.
func Handle(path string, h http.Handler) {
	mu.Lock()
	defer mu.Unlock()
	mux.Handle(path, h)
}

// HandleHTTP registers the handler of the plugin's HTTP routes, which the
// server serves under /plugins/{id}/. It sees request paths without that
// prefix, starting with "/". Requests made with a valid access token
// carry the user in the Mavio-User-Id, Mavio-User-Name and
// Mavio-User-Admin headers instead of the token.
func HandleHTTP(h http.Handler) {
	Handle(abi.HTTPPrefix+"/", http.StripPrefix(abi.HTTPPrefix, h))
}

// Handler returns the handler serving all registered services.
func Handler() http.Handler {
	mu.RLock()
	defer mu.RUnlock()
	return mux
}

// HTTPClient returns the client plugins use for outbound HTTP. In the WASM
// runtime its requests are performed by the host and restricted to the hosts
// the manifest declares; in the process runtime it is http.DefaultClient.
func HTTPClient() *http.Client {
	mu.RLock()
	defer mu.RUnlock()
	return httpClient
}

// HostClient returns the client for the host API at HostURL. The host
// authorizes its requests by the scopes of the manifest's permissions.api;
// a request acts as the plugin itself unless it names a user in
// UserHeader and the manifest sets permissions.act_as_users. A device
// controller's request acting as a user may also name one of its devices
// in DeviceHeader, so that the playbacks it starts belong to the device.
func HostClient() *http.Client {
	mu.RLock()
	defer mu.RUnlock()
	return hostClient
}

// SetHostClient replaces the client returned by HostClient. Runtime entry
// points call it; plugins normally do not.
func SetHostClient(c *http.Client) {
	mu.Lock()
	defer mu.Unlock()
	hostClient = c
}

// DataDir returns the plugin's writable data folder, which the host keeps
// across restarts and upgrades, includes in backups and deletes when the
// plugin is uninstalled; it is empty when the host gives none.
func DataDir() string { return os.Getenv(proc.EnvData) }

// SetHTTPClient replaces the client returned by HTTPClient. Runtime entry
// points call it; plugins normally do not.
func SetHTTPClient(c *http.Client) {
	mu.Lock()
	defer mu.Unlock()
	httpClient = c
}
