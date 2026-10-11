package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

var (
	fieldTypes = map[client.FieldType]pluginv1.SettingFieldType{
		client.FieldText:        pluginv1.SettingFieldType_SETTING_FIELD_TYPE_TEXT,
		client.FieldNumber:      pluginv1.SettingFieldType_SETTING_FIELD_TYPE_NUMBER,
		client.FieldCheckbox:    pluginv1.SettingFieldType_SETTING_FIELD_TYPE_CHECKBOX,
		client.FieldSelect:      pluginv1.SettingFieldType_SETTING_FIELD_TYPE_SELECT,
		client.FieldMultiSelect: pluginv1.SettingFieldType_SETTING_FIELD_TYPE_MULTI_SELECT,
		client.FieldInfo:        pluginv1.SettingFieldType_SETTING_FIELD_TYPE_INFO,
	}
	protocols = map[client.Protocol]pluginv1.DownloadProtocol{
		client.Torrent: pluginv1.DownloadProtocol_DOWNLOAD_PROTOCOL_TORRENT,
		client.Usenet:  pluginv1.DownloadProtocol_DOWNLOAD_PROTOCOL_USENET,
	}
	kinds = map[client.Kind]pluginv1.DownloadKind{
		client.Movie:  pluginv1.DownloadKind_DOWNLOAD_KIND_MOVIE,
		client.Series: pluginv1.DownloadKind_DOWNLOAD_KIND_SERIES,
	}
	statusNames = map[client.Status]pluginv1.DownloadStatus{
		client.Queued:      pluginv1.DownloadStatus_DOWNLOAD_STATUS_QUEUED,
		client.Paused:      pluginv1.DownloadStatus_DOWNLOAD_STATUS_PAUSED,
		client.Downloading: pluginv1.DownloadStatus_DOWNLOAD_STATUS_DOWNLOADING,
		client.Completed:   pluginv1.DownloadStatus_DOWNLOAD_STATUS_COMPLETED,
		client.Failed:      pluginv1.DownloadStatus_DOWNLOAD_STATUS_FAILED,
		client.Warning:     pluginv1.DownloadStatus_DOWNLOAD_STATUS_WARNING,
	}
)

// ListDownloadClientDefinitions implements DownloadClientProviderService.
func (p *plugin) ListDownloadClientDefinitions(context.Context, *pluginv1.ListDownloadClientDefinitionsRequest) (*pluginv1.ListDownloadClientDefinitionsResponse, error) {
	out := make([]*pluginv1.DownloadClientDefinition, 0, len(p.defs))
	for _, d := range p.defs {
		fields := make([]*pluginv1.SettingField, 0, len(d.Fields))
		for _, f := range d.Fields {
			var opts []*pluginv1.SettingFieldOption
			for _, o := range f.Options {
				opts = append(opts, pluginv1.SettingFieldOption_builder{ValueJson: proto.String(jsonValue(o.Value)), Label: proto.String(o.Label)}.Build())
			}
			fields = append(fields, pluginv1.SettingField_builder{
				Name: proto.String(f.Name), Label: proto.String(f.Label), HelpText: proto.String(f.HelpText),
				Type: fieldTypes[f.Type].Enum(), DefaultJson: proto.String(jsonValue(f.Default)),
				Options: opts, Advanced: proto.Bool(f.Advanced), Secret: proto.Bool(f.Secret),
			}.Build())
		}
		out = append(out, pluginv1.DownloadClientDefinition_builder{
			Id: proto.String(d.ID), Name: proto.String(d.Name), Description: proto.String(d.Description),
			Protocol: protocols[d.Protocol].Enum(), Fields: fields,
		}.Build())
	}
	return pluginv1.ListDownloadClientDefinitionsResponse_builder{Definitions: out}.Build(), nil
}

// jsonValue returns v as JSON; empty for nil.
func jsonValue(v any) string {
	if v == nil {
		return ""
	}
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(data)
}

// ListDownloadClients implements DownloadClientProviderService.
func (p *plugin) ListDownloadClients(context.Context, *pluginv1.ListDownloadClientsRequest) (*pluginv1.ListDownloadClientsResponse, error) {
	p.mu.RLock()
	list := slices.Clone(p.clients)
	p.mu.RUnlock()
	out := make([]*pluginv1.DownloadClient, len(list))
	for i, c := range list {
		opts := c.cfg.options()
		b := pluginv1.DownloadClient_builder{
			Id: proto.String(c.cfg.ID), Name: proto.String(c.name()), DefinitionId: proto.String(c.def.ID),
			Protocol: protocols[c.def.Protocol].Enum(), Enabled: proto.Bool(orTrue(c.cfg.Enabled)),
			Priority:                 proto.Int32(int32(c.cfg.priority())),
			RemoveCompletedDownloads: proto.Bool(opts.RemoveCompletedDownloads), RemoveFailedDownloads: proto.Bool(opts.RemoveFailedDownloads),
			PrefersTorrentFile: proto.Bool(c.def.PrefersTorrentFile), Host: proto.String(c.client.Host()),
		}
		if until, failure, ok := p.statuses.Disabled(c.cfg.ID); ok {
			b.DisabledUntil, b.Failure = timestamppb.New(until), proto.String(failure)
		}
		out[i] = b.Build()
	}
	return pluginv1.ListDownloadClientsResponse_builder{Clients: out}.Build(), nil
}

// TestDownloadClient implements DownloadClientProviderService.
func (p *plugin) TestDownloadClient(ctx context.Context, req *pluginv1.TestDownloadClientRequest) (*pluginv1.TestDownloadClientResponse, error) {
	var cl client.Client
	if req.GetDefinitionId() != "" {
		c, _, err := p.newClient(req.GetDefinitionId(), []byte(req.GetSettingsJson()))
		if err != nil {
			return pluginv1.TestDownloadClientResponse_builder{Failure: proto.String(err.Error())}.Build(), nil
		}
		cl = c
	} else {
		c, err := p.client(req.GetClientId())
		if err != nil {
			return nil, err
		}
		cl = c.client
	}
	if err := cl.Test(ctx); err != nil {
		return pluginv1.TestDownloadClientResponse_builder{Failure: proto.String(err.Error())}.Build(), nil
	}
	return &pluginv1.TestDownloadClientResponse{}, nil
}

// AddDownload implements DownloadClientProviderService.
func (p *plugin) AddDownload(ctx context.Context, req *pluginv1.AddDownloadRequest) (*pluginv1.AddDownloadResponse, error) {
	c, err := p.client(req.GetClientId())
	if err != nil {
		return nil, err
	}
	r := &client.Release{
		Title: req.GetTitle(), Recent: req.GetRecent(), Data: req.GetData(), FileName: req.GetFileName(),
		MagnetURL: req.GetMagnetUrl(), InfoHash: req.GetInfoHash(),
	}
	switch req.GetKind() {
	case pluginv1.DownloadKind_DOWNLOAD_KIND_SERIES:
		r.Kind = client.Series
	default:
		r.Kind = client.Movie
	}
	if req.HasSeedRatio() {
		r.SeedRatio = proto.Float64(req.GetSeedRatio())
	}
	if req.HasSeedTime() {
		d := req.GetSeedTime().AsDuration()
		r.SeedTime = &d
	}
	id, err := c.client.Add(ctx, r)
	if err != nil {
		return nil, p.failure(c, err)
	}
	p.statuses.RecordSuccess(c.cfg.ID)
	return pluginv1.AddDownloadResponse_builder{DownloadId: proto.String(id)}.Build(), nil
}

// failure records a client's failure to reach or sign in to it, and
// returns the error with its code.
func (p *plugin) failure(c *configured, err error) error {
	var auth *client.AuthError
	var unavailable *client.UnavailableError
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return connect.NewError(connect.CodeCanceled, err)
	case errors.Is(err, client.ErrMagnetNotSupported), errors.Is(err, client.ErrNotSupported):
		return connect.NewError(connect.CodeUnimplemented, err)
	case errors.As(err, &auth):
		p.statuses.RecordFailure(c.cfg.ID, err.Error())
		return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("%s: %w", c.name(), err))
	case errors.As(err, &unavailable):
		p.statuses.RecordFailure(c.cfg.ID, err.Error())
		return connect.NewError(connect.CodeUnavailable, fmt.Errorf("%s: %w", c.name(), err))
	}
	return connect.NewError(connect.CodeInternal, fmt.Errorf("%s: %w", c.name(), err))
}

// ListDownloads implements DownloadClientProviderService. Each client
// failing to list is recorded, as Radarr's download monitoring does.
func (p *plugin) ListDownloads(ctx context.Context, req *pluginv1.ListDownloadsRequest) (*pluginv1.ListDownloadsResponse, error) {
	var targets []*configured
	if ids := req.GetClientIds(); len(ids) > 0 {
		for _, id := range ids {
			c, err := p.client(id)
			if err != nil {
				return nil, err
			}
			targets = append(targets, c)
		}
	} else {
		p.mu.RLock()
		for _, c := range p.clients {
			if _, _, disabled := p.statuses.Disabled(c.cfg.ID); orTrue(c.cfg.Enabled) && !disabled {
				targets = append(targets, c)
			}
		}
		p.mu.RUnlock()
	}
	type result struct {
		items   []*pluginv1.Download
		status  *pluginv1.DownloadClientStatus
		failure *pluginv1.DownloadClientFailure
	}
	results := make([]result, len(targets))
	var wg sync.WaitGroup
	for i, c := range targets {
		wg.Go(func() {
			fail := func(err error) {
				p.statuses.RecordFailure(c.cfg.ID, err.Error())
				f := pluginv1.DownloadClientFailure_builder{ClientId: proto.String(c.cfg.ID), Message: proto.String(err.Error())}
				if until, _, ok := p.statuses.Disabled(c.cfg.ID); ok {
					f.DisabledUntil = timestamppb.New(until)
				}
				results[i].failure = f.Build()
			}
			items, err := c.client.Items(ctx, c.cfg.options())
			if err != nil {
				fail(err)
				return
			}
			p.statuses.RecordSuccess(c.cfg.ID)
			for _, it := range items {
				results[i].items = append(results[i].items, itemProto(c.cfg.ID, it))
			}
			if !req.GetIncludeStatuses() {
				return
			}
			info, err := c.client.Info(ctx)
			if err != nil {
				fail(err)
				return
			}
			results[i].status = pluginv1.DownloadClientStatus_builder{
				ClientId: proto.String(c.cfg.ID), IsLocalhost: proto.Bool(info.IsLocalhost), SortingMode: proto.String(info.SortingMode),
				RemovesCompletedDownloads: proto.Bool(info.RemovesCompletedDownloads), OutputRootFolders: info.OutputRootFolders,
			}.Build()
		})
	}
	wg.Wait()
	resp := &pluginv1.ListDownloadsResponse{}
	var downloads []*pluginv1.Download
	var statuses []*pluginv1.DownloadClientStatus
	var failures []*pluginv1.DownloadClientFailure
	for _, r := range results {
		downloads = append(downloads, r.items...)
		if r.status != nil {
			statuses = append(statuses, r.status)
		}
		if r.failure != nil {
			failures = append(failures, r.failure)
		}
	}
	resp.SetDownloads(downloads)
	resp.SetStatuses(statuses)
	resp.SetFailures(failures)
	return resp, nil
}

func itemProto(clientID string, it client.Item) *pluginv1.Download {
	b := pluginv1.Download_builder{
		ClientId: proto.String(clientID), Id: proto.String(it.ID), Title: proto.String(it.Title), Category: proto.String(it.Category),
		Kind: kinds[it.Kind].Enum(), TotalSize: proto.Int64(it.TotalSize), RemainingSize: proto.Int64(it.RemainingSize),
		OutputPath: proto.String(it.OutputPath), Message: proto.String(it.Message), Status: statusNames[it.Status].Enum(),
		Encrypted: proto.Bool(it.Encrypted), CanMoveFiles: proto.Bool(it.CanMoveFiles), CanBeRemoved: proto.Bool(it.CanBeRemoved),
		HasPostImportCategory: proto.Bool(it.HasPostImportCategory),
	}
	if it.RemainingTime != nil {
		b.RemainingTime = durationpb.New(*it.RemainingTime)
	}
	if it.SeedRatio != nil {
		b.SeedRatio = proto.Float64(*it.SeedRatio)
	}
	return b.Build()
}

// item finds a client's download by ID; a download not listed is one of
// that ID alone.
func (p *plugin) item(ctx context.Context, c *configured, id string) (client.Item, bool, error) {
	items, err := c.client.Items(ctx, c.cfg.options())
	if err != nil {
		return client.Item{}, false, p.failure(c, err)
	}
	for _, it := range items {
		if it.ID == id {
			return it, true, nil
		}
	}
	return client.Item{ID: id}, false, nil
}

// GetDownload implements DownloadClientProviderService.
func (p *plugin) GetDownload(ctx context.Context, req *pluginv1.GetDownloadRequest) (*pluginv1.GetDownloadResponse, error) {
	c, err := p.client(req.GetClientId())
	if err != nil {
		return nil, err
	}
	it, ok, err := p.item(ctx, c, req.GetDownloadId())
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("download %s not found in %s", req.GetDownloadId(), c.name()))
	}
	if imp, ok := c.client.(client.Importer); ok {
		if it, err = imp.ImportItem(ctx, it, c.cfg.options()); err != nil {
			return nil, p.failure(c, err)
		}
	}
	return pluginv1.GetDownloadResponse_builder{Download: itemProto(c.cfg.ID, it)}.Build(), nil
}

// RemoveDownload implements DownloadClientProviderService.
func (p *plugin) RemoveDownload(ctx context.Context, req *pluginv1.RemoveDownloadRequest) (*pluginv1.RemoveDownloadResponse, error) {
	c, err := p.client(req.GetClientId())
	if err != nil {
		return nil, err
	}
	it, _, err := p.item(ctx, c, req.GetDownloadId())
	if err != nil {
		return nil, err
	}
	if err := c.client.Remove(ctx, it, req.GetDeleteData()); err != nil {
		return nil, p.failure(c, err)
	}
	return &pluginv1.RemoveDownloadResponse{}, nil
}

// MarkDownloadImported implements DownloadClientProviderService.
func (p *plugin) MarkDownloadImported(ctx context.Context, req *pluginv1.MarkDownloadImportedRequest) (*pluginv1.MarkDownloadImportedResponse, error) {
	c, err := p.client(req.GetClientId())
	if err != nil {
		return nil, err
	}
	m, ok := c.client.(client.Marker)
	if !ok {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("%s does not support marking items as imported", c.def.Name))
	}
	it, _, err := p.item(ctx, c, req.GetDownloadId())
	if err != nil {
		return nil, err
	}
	if err := m.MarkImported(ctx, it); err != nil {
		return nil, p.failure(c, err)
	}
	return &pluginv1.MarkDownloadImportedResponse{}, nil
}
