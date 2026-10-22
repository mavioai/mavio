package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/libs/plugin/guest"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

// containerClientConfig is a client the server runs in a container
// dependency of its name, which the manifest sets up for the plugin.
type containerClientConfig struct {
	Name string `json:"name"`
	// Enabled, RemoveCompletedDownloads and RemoveFailedDownloads are
	// true when unset.
	Enabled                  *bool    `json:"enabled"`
	Priority                 int      `json:"priority"`
	Tags                     []string `json:"tags"`
	RemoveCompletedDownloads *bool    `json:"remove_completed_downloads"`
	RemoveFailedDownloads    *bool    `json:"remove_failed_downloads"`
	// Settings are the settings of the client's definition the container
	// leaves to the administrator, over the definition's defaults.
	Settings json.RawMessage `json:"settings"`
}

// containerClient is how the plugin reaches the client of a container
// dependency: its definition, and its settings from the server's secrets.
type containerClient struct {
	definition string
	// credentials returns the settings signing in with the dependency's
	// secrets.
	credentials func(secrets map[string]string) map[string]any
}

// containerUser is the user the containers' Web UIs and RPC take.
const containerUser = "admin"

// containerClients are the clients of the manifest's container
// dependencies, by name.
var containerClients = map[string]containerClient{
	"qbittorrent": {definition: "qbittorrent", credentials: func(s map[string]string) map[string]any {
		return map[string]any{"username": containerUser, "password": s["password"]}
	}},
	"transmission": {definition: "transmission", credentials: func(s map[string]string) map[string]any {
		return map[string]any{"username": containerUser, "password": s["password"]}
	}},
	"sabnzbd": {definition: "sabnzbd", credentials: func(s map[string]string) map[string]any {
		return map[string]any{"api_key": s["api-key"]}
	}},
}

// containerFilled are the settings the container fills in: where the
// client is and how to sign in to it.
var containerFilled = map[string]bool{
	"host": true, "port": true, "use_ssl": true, "url_base": true,
	"username": true, "password": true, "api_key": true,
}

// containerFolder is where the containers put downloads, the folder the
// manifest mounts; the folders of the containers' directory settings are
// in it.
const containerFolder = "/downloads/"

// containerDefinition returns a definition as its containers' clients
// take it: without the settings the containers fill in, and with folders
// in the containers' folder.
func containerDefinition(d client.Definition) client.Definition {
	var fields []client.Field
	for _, f := range d.Fields {
		if containerFilled[f.Name] {
			continue
		}
		if directorySetting(f.Name) {
			f.Prefix = containerFolder
		}
		fields = append(fields, f)
	}
	d.Fields = fields
	return d
}

// directorySetting reports whether a setting is a folder downloads go in.
func directorySetting(name string) bool { return strings.HasSuffix(name, "_directory") }

// containerClientID is the ID of the client of a container dependency.
func containerClientID(name string) string { return "container-" + name }

// containerClientName is the name of the client of a container dependency
// of a definition.
func containerClientName(d client.Definition) string { return d.Name + " (container)" }

// containers returns the plugin's container dependencies by name; nil
// when the host cannot tell. The server configures the plugin again when
// their addresses change.
func (p *plugin) containers(ctx context.Context) map[string]*pluginv1.ContainerStatus {
	if p.host == nil || p.host() == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := pluginv1connect.NewHostServiceClient(p.host(), guest.HostURL).ListContainers(ctx, &pluginv1.ListContainersRequest{})
	if err != nil {
		p.log.Warn("list container dependencies", "err", err)
		return nil
	}
	out := map[string]*pluginv1.ContainerStatus{}
	for _, c := range resp.GetContainers() {
		out[c.GetName()] = c
	}
	return out
}

// buildContainers makes the clients of the enabled container clients
// whose containers have an address. Those that cannot be made are logged
// and left out.
func (p *plugin) buildContainers(ctx context.Context, c config) []*configured {
	var deps map[string]*pluginv1.ContainerStatus
	var out []*configured
	for _, cc := range c.ContainerClients {
		spec, ok := containerClients[cc.Name]
		if !ok || !orTrue(cc.Enabled) {
			continue
		}
		if deps == nil {
			if deps = p.containers(ctx); deps == nil {
				return nil
			}
		}
		dep := deps[cc.Name]
		if dep.GetAddress() == "" {
			continue
		}
		cfg, _, err := p.containerConfig(cc, spec, dep)
		if err == nil {
			var cl client.Client
			var def client.Definition
			if cl, def, err = p.newClient(cfg.ID, cfg.Definition, cfg.Settings); err == nil {
				out = append(out, &configured{cfg: cfg, def: def, client: cl, historyLimit: c.HistoryLimit, container: cc.Name})
				continue
			}
		}
		p.log.Error("configure the client of a container", "container", cc.Name, "err", err)
	}
	return out
}

// containerConfig returns the configuration of a container's client: its
// definition's defaults, the settings the administrator gave, and the
// container's address and credentials.
func (p *plugin) containerConfig(cc containerClientConfig, spec containerClient, dep *pluginv1.ContainerStatus) (clientConfig, client.Definition, error) {
	def, ok := p.byID[spec.definition]
	if !ok {
		return clientConfig{}, def, fmt.Errorf("unknown definition %q", spec.definition)
	}
	u, err := url.Parse(dep.GetAddress())
	if err != nil {
		return clientConfig{}, def, fmt.Errorf("address %q: %w", dep.GetAddress(), err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return clientConfig{}, def, fmt.Errorf("address %q has no port", dep.GetAddress())
	}
	settings := map[string]any{}
	for _, f := range def.Fields {
		if f.Default != nil && f.Type != client.FieldInfo {
			settings[f.Name] = f.Default
		}
	}
	if len(cc.Settings) > 0 {
		var given map[string]any
		if err := json.Unmarshal(cc.Settings, &given); err != nil {
			return clientConfig{}, def, fmt.Errorf("settings: %w", err)
		}
		for k, v := range given {
			if containerFilled[k] {
				continue
			}
			if dir, ok := v.(string); ok && directorySetting(k) && dir != "" && !strings.HasPrefix(dir, containerFolder) {
				return clientConfig{}, def, fmt.Errorf("setting %s: %q is not in %s", k, dir, containerFolder)
			}
			settings[k] = v
		}
	}
	settings["host"], settings["port"], settings["use_ssl"] = u.Hostname(), port, u.Scheme == "https"
	for k, v := range spec.credentials(dep.GetSecrets()) {
		settings[k] = v
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return clientConfig{}, def, err
	}
	return clientConfig{
		ID: containerClientID(cc.Name), Name: containerClientName(def), Definition: def.ID,
		Enabled: new(true), Priority: cc.Priority, Tags: cc.Tags,
		RemoveCompletedDownloads: cc.RemoveCompletedDownloads, RemoveFailedDownloads: cc.RemoveFailedDownloads,
		Settings: data,
	}, def, nil
}
