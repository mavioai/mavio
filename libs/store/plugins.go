package store

import (
	"context"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store/internal/ent/pluginconfig"
	"github.com/mavioai/mavio/libs/store/internal/ent/plugincontainer"
)

type pluginConfigs struct{ s *Store }

// PluginConfigs returns the configurations administrators gave plugins.
func (s *Store) PluginConfigs() core.PluginConfigRepository { return pluginConfigs{s} }

func (r pluginConfigs) Get(ctx context.Context, pluginID string) (core.PluginConfig, error) {
	c, err := r.s.read.PluginConfig.Query().Where(pluginconfig.PluginID(pluginID)).Only(ctx)
	if err != nil {
		return core.PluginConfig{}, mapErr(err, "get plugin configuration")
	}
	return core.PluginConfig{PluginID: c.PluginID, JSON: c.Config, UpdatedAt: c.UpdatedAt}, nil
}

func (r pluginConfigs) Put(ctx context.Context, c *core.PluginConfig) error {
	if err := c.Validate(); err != nil {
		return err
	}
	c.UpdatedAt = time.Now()
	err := r.s.write.PluginConfig.Create().
		SetPluginID(c.PluginID).
		SetConfig(c.JSON).
		SetUpdatedAt(c.UpdatedAt).
		OnConflictColumns(pluginconfig.FieldPluginID).
		UpdateNewValues().
		Exec(ctx)
	return mapErr(err, "put plugin configuration")
}

func (r pluginConfigs) Delete(ctx context.Context, pluginID string) error {
	_, err := r.s.write.PluginConfig.Delete().Where(pluginconfig.PluginID(pluginID)).Exec(ctx)
	return mapErr(err, "delete plugin configuration")
}

type pluginContainers struct{ s *Store }

// PluginContainers returns what the server keeps of plugins' container
// dependencies.
func (s *Store) PluginContainers() core.PluginContainerRepository { return pluginContainers{s} }

func (r pluginContainers) List(ctx context.Context, pluginID string) ([]core.PluginContainer, error) {
	rows, err := r.s.read.PluginContainer.Query().
		Where(plugincontainer.PluginID(pluginID)).
		Order(plugincontainer.ByName()).
		All(ctx)
	if err != nil {
		return nil, mapErr(err, "list plugin containers")
	}
	out := make([]core.PluginContainer, len(rows))
	for i, c := range rows {
		out[i] = core.PluginContainer{
			PluginID: c.PluginID, Name: c.Name, Mode: core.ContainerMode(c.Mode), Address: c.Address,
			Secrets: c.Secrets, UseGPU: c.UseGpu, Proxy: c.Proxy, Folders: c.Folders, PeerPort: c.PeerPort, UpdatedAt: c.UpdatedAt,
		}
	}
	return out, nil
}

func (r pluginContainers) Put(ctx context.Context, c *core.PluginContainer) error {
	if err := c.Validate(); err != nil {
		return err
	}
	c.UpdatedAt = time.Now()
	err := r.s.write.PluginContainer.Create().
		SetPluginID(c.PluginID).
		SetName(c.Name).
		SetMode(string(c.Mode)).
		SetAddress(c.Address).
		SetSecrets(c.Secrets).
		SetUseGpu(c.UseGPU).
		SetProxy(c.Proxy).
		SetFolders(c.Folders).
		SetPeerPort(c.PeerPort).
		SetUpdatedAt(c.UpdatedAt).
		OnConflictColumns(plugincontainer.FieldPluginID, plugincontainer.FieldName).
		UpdateNewValues().
		Exec(ctx)
	return mapErr(err, "put plugin container")
}

func (r pluginContainers) Delete(ctx context.Context, pluginID, name string) error {
	q := r.s.write.PluginContainer.Delete().Where(plugincontainer.PluginID(pluginID))
	if name != "" {
		q = q.Where(plugincontainer.Name(name))
	}
	_, err := q.Exec(ctx)
	return mapErr(err, "delete plugin container")
}
