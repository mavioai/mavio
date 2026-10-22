package acquisition

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// ClientPath is a folder a download client puts downloads in, as the
// client sees it, and where the server reads it.
type ClientPath struct {
	// RemotePath is the folder as the client reports it.
	RemotePath string
	// LocalPath is where the server reads it, by a download path mapping:
	// an absolute path or a remote's folder; empty when the server reads
	// RemotePath itself.
	LocalPath string
	// Container: the mapping is a folder of the client's managed
	// container, which the server mounted there, rather than stored.
	Container bool
	// RootFolder: the client reports the folder as one completed
	// downloads end up in, rather than only a mapping naming it.
	RootFolder bool
	// SuggestedPath is a folder found to hold RemotePath's files, for a
	// folder the server cannot read; empty for none.
	SuggestedPath string
	// Readable: the server reads the folder; Error says why not.
	Readable bool
	Error    string
	// LinkLibraryPaths are the library paths on the folder's storage,
	// which imports hard link files to.
	LinkLibraryPaths []string
}

// ErrClientNotFound is returned for a download client no ready provider
// has configured.
var ErrClientNotFound = errors.New("download client not found")

// client returns a client of a provider.
func (r *Runtime) client(ctx context.Context, pluginID, clientID string) (Client, error) {
	api := r.clientAPI(pluginID)
	if api == nil {
		return Client{}, fmt.Errorf("%w: %s/%s", ErrClientNotFound, pluginID, clientID)
	}
	resp, err := api.ListDownloadClients(ctx, &pluginv1.ListDownloadClientsRequest{})
	if err != nil {
		return Client{}, err
	}
	for _, c := range resp.GetClients() {
		if c.GetId() == clientID {
			return Client{PluginID: pluginID, DownloadClient: c}, nil
		}
	}
	return Client{}, fmt.Errorf("%w: %s/%s", ErrClientNotFound, pluginID, clientID)
}

// ClientPaths returns the folders a client puts completed downloads in,
// those its download path mappings name first, with where the server
// reads each; the failure is why the client did not tell its folders.
func (r *Runtime) ClientPaths(ctx context.Context, pluginID, clientID string) ([]ClientPath, string, error) {
	c, err := r.client(ctx, pluginID, clientID)
	if err != nil {
		return nil, "", err
	}
	stored, err := r.Store.DownloadPathMappings().List(ctx)
	if err != nil {
		return nil, "", err
	}
	var mappings []acquire.DownloadPathMapping
	var out []ClientPath
	for _, m := range stored {
		if m.PluginID == pluginID && m.ClientID == clientID {
			mappings = append(mappings, acquire.DownloadPathMapping{Client: c.Ref(), RemotePath: m.RemotePath, LocalPath: m.LocalPath})
			out = append(out, ClientPath{RemotePath: m.RemotePath, LocalPath: m.LocalPath})
		}
	}
	for _, m := range r.containerMappings(ctx, c) {
		mappings = append(mappings, m)
		out = append(out, ClientPath{RemotePath: m.RemotePath, LocalPath: m.LocalPath, Container: true})
	}
	roots, failure := r.rootFolders(ctx, c)
	for _, root := range roots {
		mapped := false
		for i, m := range mappings {
			if acquire.RemapDownloadPath(mappings[i:i+1], c.Ref(), root) != root || m.RemotePath == root {
				out[i].RootFolder, mapped = true, true
				break
			}
		}
		if !mapped {
			out = append(out, ClientPath{RemotePath: root, RootFolder: true})
		}
	}
	libs, err := r.libraryPaths(ctx)
	if err != nil {
		return nil, "", err
	}
	for i := range out {
		r.checkClientPath(ctx, c, &out[i], libs)
	}
	return out, failure, nil
}

// rootFolders returns the folders a client says completed downloads end
// up in, or why it said none.
func (r *Runtime) rootFolders(ctx context.Context, c Client) ([]string, string) {
	api := r.clientAPI(c.PluginID)
	if api == nil {
		return nil, ErrClientNotFound.Error()
	}
	resp, err := api.ListDownloads(ctx, pluginv1.ListDownloadsRequest_builder{ClientIds: []string{c.GetId()}, IncludeStatuses: new(true)}.Build())
	if err != nil {
		return nil, err.Error()
	}
	for _, f := range resp.GetFailures() {
		if f.GetClientId() == c.GetId() {
			return nil, f.GetMessage()
		}
	}
	var roots []string
	for _, s := range resp.GetStatuses() {
		if s.GetClientId() == c.GetId() {
			for _, f := range s.GetOutputRootFolders() {
				if f != "" && !slices.Contains(roots, f) {
					roots = append(roots, f)
				}
			}
		}
	}
	return roots, ""
}

// libraryPaths returns the paths of every library.
func (r *Runtime) libraryPaths(ctx context.Context) ([]string, error) {
	libs, err := r.Store.Libraries().List(ctx)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, l := range libs {
		out = append(out, l.Paths...)
	}
	slices.Sort(out)
	return slices.Compact(out), nil
}

// checkClientPath tells whether the server reads a client's folder,
// suggests where it may for one it does not, and finds the library paths
// imports from it hard link to.
func (r *Runtime) checkClientPath(ctx context.Context, c Client, p *ClientPath, libs []string) {
	local := cmp.Or(p.LocalPath, p.RemotePath)
	if err := r.readable(ctx, local); err != nil {
		p.Error = err.Error()
		if p.LocalPath == "" {
			p.SuggestedPath = r.suggestPath(ctx, c, p.RemotePath)
		}
		return
	}
	p.Readable = true
	l, _ := downloadPath(local)
	for _, lib := range libs {
		if r.linkable(ctx, l, lib) {
			p.LinkLibraryPaths = append(p.LinkLibraryPaths, lib)
		}
	}
}

// readable checks that a download folder is a folder the server reads.
func (r *Runtime) readable(ctx context.Context, p string) error {
	l, ok := downloadPath(p)
	if !ok {
		if _, rest, isRemote := core.SplitRemotePath(p); !isRemote || strings.Trim(rest, "/") != "" {
			return fmt.Errorf("%q is not an absolute path of the server", p)
		}
	}
	if !core.IsRemotePath(l) {
		fi, err := os.Stat(l)
		if err != nil {
			return err
		}
		if !fi.IsDir() {
			return fmt.Errorf("%s is not a folder", l)
		}
		return nil
	}
	// A remote's folder, its root among them, is read by listing it.
	st, err := r.Storages.Open(ctx, l)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()
	_, err = fs.ReadDir(core.StorageFS(ctx, st), ".")
	return err
}

// suggestPath returns a folder of a remote on the client's host that
// holds a folder named as the client's, or one of its parents', is, or
// "" for none: an SFTP or SMB remote of a NAS that the client runs on
// sees its folders below the remote's root.
func (r *Runtime) suggestPath(ctx context.Context, c Client, remote string) string {
	host := hostName(c.GetHost())
	if host == "" || r.RemoteHosts == nil {
		return ""
	}
	hosts, err := r.RemoteHosts(ctx)
	if err != nil {
		r.logger().DebugContext(ctx, "list remotes for download path suggestions", "err", err)
		return ""
	}
	parts := strings.FieldsFunc(remote, isSep)
	if len(parts) > 0 && strings.HasSuffix(parts[0], ":") {
		// A Windows drive.
		parts = parts[1:]
	}
	for _, name := range slices.Sorted(maps.Keys(hosts)) {
		if !strings.EqualFold(hostName(hosts[name]), host) {
			continue
		}
		for i := range parts {
			p := core.RemoteScheme + name + ":" + strings.Join(parts[i:], "/")
			if r.readable(ctx, p) == nil {
				return p
			}
		}
	}
	return ""
}

func isSep(r rune) bool { return r == '/' || r == '\\' }

// hostName returns the host of a host name, address or URL, without a
// port.
func hostName(s string) string {
	s = strings.TrimSpace(s)
	if u, err := url.Parse(s); err == nil && u.Host != "" {
		s = u.Host
	}
	if h, _, err := net.SplitHostPort(s); err == nil {
		s = h
	}
	return strings.Trim(s, "[]")
}

// linkable reports whether files of a download folder hard link to a
// library path: both on one local volume, or both on one remote that
// links files.
func (r *Runtime) linkable(ctx context.Context, folder, lib string) bool {
	if core.IsRemotePath(folder) != core.IsRemotePath(lib) {
		return false
	}
	if !core.IsRemotePath(folder) {
		same, ok := sameDevice(filepath.FromSlash(folder), filepath.FromSlash(lib))
		return ok && same
	}
	fr, _, _ := core.SplitRemotePath(folder)
	lr, _, _ := core.SplitRemotePath(lib)
	if fr != lr {
		return false
	}
	st, err := r.Storages.Open(ctx, lib)
	if err != nil {
		return false
	}
	defer func() { _ = st.Close() }()
	return st.Capabilities().HardLinks
}

// SetClientPaths replaces the download path mappings of a client, but for
// its container's, and returns its folders as ClientPaths does.
func (r *Runtime) SetClientPaths(ctx context.Context, pluginID, clientID string, paths []ClientPath) ([]ClientPath, string, error) {
	if _, err := r.client(ctx, pluginID, clientID); err != nil {
		return nil, "", err
	}
	var ms []core.DownloadPathMapping
	for _, p := range paths {
		if p.LocalPath == "" || p.Container {
			continue
		}
		local := p.LocalPath
		if !core.IsRemotePath(local) {
			local = filepath.Clean(local)
		} else if remote, rest, ok := core.SplitRemotePath(local); ok {
			local = core.RemoteScheme + remote + ":" + strings.Trim(path.Clean("/"+rest), "/")
		}
		ms = append(ms, core.DownloadPathMapping{
			PluginID: pluginID, ClientID: clientID, RemotePath: p.RemotePath, LocalPath: local,
		})
	}
	if err := r.Store.DownloadPathMappings().ReplaceClient(ctx, pluginID, clientID, ms); err != nil {
		return nil, "", err
	}
	return r.ClientPaths(ctx, pluginID, clientID)
}
