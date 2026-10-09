package core

import (
	"encoding/json"
	"fmt"
	"time"
)

// PluginConfig is the configuration an administrator gave a plugin: a JSON
// document matching the configuration schema of the plugin's manifest. It
// is kept by plugin ID, so it survives upgrades and reinstalls.
type PluginConfig struct {
	PluginID  string
	JSON      string
	UpdatedAt time.Time
}

// Validate checks the configuration's invariants. Conformance to the
// plugin's schema is checked by whoever knows the manifest.
func (c *PluginConfig) Validate() error {
	switch {
	case c.PluginID == "":
		return fmt.Errorf("%w: plugin configuration requires a plugin ID", ErrInvalid)
	case !json.Valid([]byte(c.JSON)):
		return fmt.Errorf("%w: configuration of plugin %s is not JSON", ErrInvalid, c.PluginID)
	}
	return nil
}
