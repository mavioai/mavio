// Package guest is the plugin-side SDK. A plugin registers the Connect
// handlers of the services it implements with Handle, and the runtime entry
// point — guest/wasm or guest/process — serves them:
//
//	func init() {
//		guest.Handle(pluginv1connect.NewPluginServiceHandler(&lifecycle{}))
//		guest.Handle(pluginv1connect.NewMetadataProviderServiceHandler(&provider{}))
//	}
//
// The same handler code builds for both runtimes.
package guest

import (
	"net/http"
	"sync"
)

var (
	mu         sync.RWMutex
	mux        = http.NewServeMux()
	httpClient = http.DefaultClient
)

// Handle registers the handler for a path, in the form returned by the
// generated New…ServiceHandler constructors.
func Handle(path string, h http.Handler) {
	mu.Lock()
	defer mu.Unlock()
	mux.Handle(path, h)
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

// SetHTTPClient replaces the client returned by HTTPClient. Runtime entry
// points call it; plugins normally do not.
func SetHTTPClient(c *http.Client) {
	mu.Lock()
	defer mu.Unlock()
	httpClient = c
}
