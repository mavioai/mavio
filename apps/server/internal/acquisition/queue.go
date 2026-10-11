package acquisition

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// ClientStatus is what a download client last told of a tracked
// download.
type ClientStatus struct {
	Status        pluginv1.DownloadStatus
	RemainingSize int64
	// RemainingTime is unknown when HasRemainingTime is false.
	RemainingTime    time.Duration
	HasRemainingTime bool
	Message          string
	At               time.Time
}

// msgNotInClient is the warning of downloads that left their client.
const msgNotInClient = "Download is no longer in the download client"

// Status returns what a download's client last told of it, if it was
// polled since the server started.
func (r *Runtime) Status(id core.ID) (ClientStatus, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.statuses[id]
	return s, ok
}

func (r *Runtime) setStatus(id core.ID, item *pluginv1.Download) {
	s := ClientStatus{
		Status: item.GetStatus(), RemainingSize: item.GetRemainingSize(), Message: item.GetMessage(), At: r.now(),
		HasRemainingTime: item.HasRemainingTime(),
	}
	if s.HasRemainingTime {
		s.RemainingTime = item.GetRemainingTime().AsDuration()
	}
	r.mu.Lock()
	if r.statuses == nil {
		r.statuses = map[core.ID]ClientStatus{}
	}
	r.statuses[id] = s
	r.mu.Unlock()
}

// pruneStatuses forgets the statuses of downloads no longer tracked.
func (r *Runtime) pruneStatuses(keep map[core.ID]bool) {
	r.mu.Lock()
	for id := range r.statuses {
		if !keep[id] {
			delete(r.statuses, id)
		}
	}
	r.mu.Unlock()
}

// polled is a tracked download as its client lists it.
type polled struct {
	client Client
	item   *pluginv1.Download
}

// CheckQueue polls the download clients and moves the tracked downloads
// on, as Radarr's DownloadMonitoringService and DownloadProcessingService:
// completed downloads are imported when CompletedDownloadHandling is set,
// failed and encrypted ones are handled as failed, and imported ones are
// removed from their clients when the clients say so. Downloads a client
// lists that are not tracked are left alone; tracked downloads gone from
// a client that answered are warned of.
func (r *Runtime) CheckQueue(ctx context.Context) error {
	r.importing.Lock()
	defer r.importing.Unlock()
	tracked, err := r.unfinished(ctx, core.NilID)
	if err != nil {
		return err
	}
	providers := r.clients()
	if len(providers) == 0 {
		r.pruneStatuses(nil)
		return nil
	}
	clients, failures := r.ListClients(ctx)
	byRef := map[string]Client{}
	for _, c := range clients {
		byRef[c.Ref()] = c
	}
	mappings, err := r.remotePathMappings(ctx)
	if err != nil {
		return err
	}
	answered := map[string]bool{}
	var found []polled
	for _, p := range providers {
		if _, failed := failures[p.ID]; failed {
			continue
		}
		resp, err := p.Client.ListDownloads(ctx, pluginv1.ListDownloadsRequest_builder{IncludeStatuses: new(true)}.Build())
		if err != nil {
			r.logger().WarnContext(ctx, "couldn't list downloads", "plugin", p.ID, "err", err)
			continue
		}
		failedClients := map[string]bool{}
		for _, f := range resp.GetFailures() {
			failedClients[f.GetClientId()] = true
			r.logger().WarnContext(ctx, "download client failed", "plugin", p.ID, "client", f.GetClientId(), "err", f.GetMessage())
		}
		for _, c := range clients {
			// A client left out after failing is not asked, as Radarr's
			// DownloadClientProvider.GetDownloadClients filters blocked
			// clients: its downloads keep their last state.
			backingOff := c.HasDisabledUntil() && c.GetDisabledUntil().AsTime().After(r.now())
			if c.PluginID == p.ID && c.GetEnabled() && !backingOff && !failedClients[c.GetId()] {
				answered[c.Ref()] = true
			}
		}
		for _, item := range resp.GetDownloads() {
			c, ok := byRef[p.ID+"/"+item.GetClientId()]
			if !ok {
				c = Client{PluginID: p.ID, DownloadClient: pluginv1.DownloadClient_builder{Id: new(item.GetClientId())}.Build()}
			}
			found = append(found, polled{client: c, item: item})
		}
	}

	rn, err := r.newRun(ctx)
	if err != nil {
		return err
	}
	keep := map[core.ID]bool{}
	for _, f := range found {
		d, err := r.Store.Downloads().GetByDownloadID(ctx, f.client.PluginID, f.item.GetClientId(), f.item.GetId())
		if errors.Is(err, core.ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		keep[d.ID] = true
		r.setStatus(d.ID, f.item)
		if err := r.track(ctx, rn, &d, f, mappings); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			r.logger().WarnContext(ctx, "couldn't process download", "download", d.Title, "err", err)
		}
	}
	for _, d := range tracked {
		if keep[d.ID] {
			continue
		}
		if !answered[d.PluginID+"/"+d.ClientID] {
			// The client did not answer: what it last told stands.
			keep[d.ID] = true
			continue
		}
		if slices.Contains(d.Messages, msgNotInClient) {
			continue
		}
		d.Messages = append(d.Messages, msgNotInClient)
		d.UpdatedAt = r.now()
		if err := r.Store.Downloads().Put(ctx, &d); err != nil {
			return err
		}
	}
	r.pruneStatuses(keep)
	return nil
}

// track moves a tracked download its client listed on, as Radarr's
// FailedDownloadService.Check, CompletedDownloadService.Check and the
// processing of their pending states.
func (r *Runtime) track(ctx context.Context, rn *run, d *core.Download, p polled, mappings []acquire.RemotePathMapping) error {
	item := p.item
	switch d.State {
	case core.DownloadImported:
		return r.removeImported(ctx, d, p)
	case core.DownloadFailed, core.DownloadIgnored:
		return nil
	case core.DownloadFailedPending:
		return r.failTracked(ctx, d, p, failureMessage(item))
	}
	if item.GetEncrypted() || item.GetStatus() == pluginv1.DownloadStatus_DOWNLOAD_STATUS_FAILED {
		d.State = core.DownloadFailedPending
		if err := r.putDownload(ctx, d); err != nil {
			return err
		}
		return r.failTracked(ctx, d, p, failureMessage(item))
	}
	if item.GetStatus() != pluginv1.DownloadStatus_DOWNLOAD_STATUS_COMPLETED {
		// Still downloading: the client's warning, if any, is the message.
		var msgs []string
		if item.GetStatus() == pluginv1.DownloadStatus_DOWNLOAD_STATUS_WARNING && item.GetMessage() != "" {
			msgs = []string{item.GetMessage()}
		}
		if d.State == core.DownloadDownloading && slices.Equal(msgs, d.Messages) {
			return nil
		}
		d.State, d.Messages = core.DownloadDownloading, msgs
		return r.putDownload(ctx, d)
	}
	if !r.settings().CompletedDownloadHandling {
		return nil
	}
	return r.importDownload(ctx, rn, d, p, mappings)
}

// failureMessage is why a download failed, as Radarr's ProcessFailed
// says.
func failureMessage(item *pluginv1.Download) string {
	switch {
	case item.GetEncrypted():
		return "Encrypted download detected"
	case item.GetStatus() == pluginv1.DownloadStatus_DOWNLOAD_STATUS_FAILED && strings.TrimSpace(item.GetMessage()) != "":
		return item.GetMessage()
	}
	return "Failed download detected"
}

func (r *Runtime) putDownload(ctx context.Context, d *core.Download) error {
	d.UpdatedAt = r.now()
	return r.Store.Downloads().Put(ctx, d)
}

// warn sets the messages of a download, keeping its state, as Radarr's
// TrackedDownload.Warn.
func (r *Runtime) warn(ctx context.Context, d *core.Download, msgs ...string) error {
	if slices.Equal(d.Messages, msgs) {
		return nil
	}
	d.Messages = msgs
	r.logger().DebugContext(ctx, "download warning", "download", d.Title, "messages", msgs)
	return r.putDownload(ctx, d)
}

// removeImported removes an imported download from its client once it
// can be, when the client is set to remove completed downloads, as
// Radarr's DownloadEventHub.
func (r *Runtime) removeImported(ctx context.Context, d *core.Download, p polled) error {
	if !p.client.GetRemoveCompletedDownloads() || !p.item.GetCanBeRemoved() ||
		p.item.GetStatus() == pluginv1.DownloadStatus_DOWNLOAD_STATUS_DOWNLOADING {
		return nil
	}
	r.removeFromClient(ctx, d, true)
	return nil
}

// removeFromClient removes a download from its client, logging what
// fails, as Radarr's DownloadEventHub.RemoveFromDownloadClient. It
// reports whether the download was removed.
func (r *Runtime) removeFromClient(ctx context.Context, d *core.Download, deleteData bool) bool {
	api := r.clientAPI(d.PluginID)
	if api == nil {
		r.logger().WarnContext(ctx, "couldn't remove download: its plugin is not ready", "download", d.Title, "plugin", d.PluginID)
		return false
	}
	_, err := api.RemoveDownload(ctx, pluginv1.RemoveDownloadRequest_builder{
		ClientId: new(d.ClientID), DownloadId: new(d.DownloadID), DeleteData: new(deleteData),
	}.Build())
	switch {
	case connect.CodeOf(err) == connect.CodeUnimplemented:
		r.logger().WarnContext(ctx, "removing downloads is not supported by the download client", "client", d.ClientID)
		return false
	case err != nil:
		r.logger().ErrorContext(ctx, "couldn't remove download from its client", "download", d.Title, "client", d.ClientID, "err", err)
		return false
	}
	r.logger().DebugContext(ctx, "removed download from its client", "download", d.Title, "client", d.ClientID)
	return true
}

// remotePathMappings returns the stored remote path mappings.
func (r *Runtime) remotePathMappings(ctx context.Context) ([]acquire.RemotePathMapping, error) {
	stored, err := r.Store.RemotePathMappings().List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]acquire.RemotePathMapping, len(stored))
	for i, m := range stored {
		out[i] = acquire.RemotePathMapping{Host: m.Host, RemotePath: m.RemotePath, LocalPath: m.LocalPath}
	}
	return out, nil
}
