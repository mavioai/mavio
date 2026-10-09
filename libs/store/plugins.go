package store

import (
	"context"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store/internal/ent/pluginconfig"
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
