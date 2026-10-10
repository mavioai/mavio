package plugins

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/plugin/manifest"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

const (
	// commandTimeout bounds a device controller's SendCommand call.
	commandTimeout = 30 * time.Second
	// maxDevices bounds the devices of a plugin, and maxDeviceField the
	// length of their IDs, names and products.
	maxDevices     = 1000
	maxDeviceField = 256
)

// Device is a remote device a plugin controls, listed as a session.
type Device struct {
	// Session is the device's session, kept while the plugin lists it.
	Session core.ID
	// Plugin is the ID of the plugin controlling it; ID is the device's ID
	// within the plugin.
	Plugin, ID    string
	Name, Product string
	Commands      []pluginv1.CommandKind
	// Listed is when the plugin last listed the device.
	Listed time.Time
}

// SetDevices replaces the devices of a device controller. Devices it
// lists again keep their sessions.
func (m *Manager) SetDevices(ctx context.Context, plugin string, list []*pluginv1.Device) error {
	m.mu.Lock()
	e, err := m.find(plugin)
	controller := err == nil && e.plugin != nil && manifest.HasCapability(e.manifest, pluginv1.Capability_CAPABILITY_DEVICE_CONTROLLER)
	m.mu.Unlock()
	if !controller {
		return connect.NewError(connect.CodePermissionDenied,
			fmt.Errorf("plugin %s does not declare CAPABILITY_DEVICE_CONTROLLER", plugin))
	}
	if err := validateDevices(list); err != nil {
		return connect.NewError(connect.CodeInvalidArgument, err)
	}
	now := m.now()
	m.devMu.Lock()
	old := m.devices[plugin]
	devices := make([]Device, 0, len(list))
	for _, d := range list {
		dev := Device{
			Session: core.NewID(), Plugin: plugin, ID: d.GetId(), Name: d.GetName(), Product: d.GetProduct(),
			Commands: d.GetCommands(), Listed: now,
		}
		if i := slices.IndexFunc(old, func(o Device) bool { return o.ID == d.GetId() }); i >= 0 {
			dev.Session = old[i].Session
		}
		devices = append(devices, dev)
	}
	if len(devices) == 0 {
		delete(m.devices, plugin)
	} else {
		m.devices[plugin] = devices
	}
	m.devMu.Unlock()
	m.devicesChanged(ctx, sessionsOf(old, devices))
	return nil
}

// validateDevices checks a plugin's list of devices.
func validateDevices(list []*pluginv1.Device) error {
	if len(list) > maxDevices {
		return fmt.Errorf("%d devices; at most %d", len(list), maxDevices)
	}
	seen := map[string]bool{}
	for _, d := range list {
		switch {
		case d.GetId() == "" || d.GetName() == "":
			return fmt.Errorf("device %q needs an ID and a name", d.GetId())
		case len(d.GetId()) > maxDeviceField || len(d.GetName()) > maxDeviceField || len(d.GetProduct()) > maxDeviceField:
			return fmt.Errorf("device %.20q: IDs, names and products have at most %d bytes", d.GetId(), maxDeviceField)
		case seen[d.GetId()]:
			return fmt.Errorf("device %q is listed twice", d.GetId())
		}
		seen[d.GetId()] = true
		for _, c := range d.GetCommands() {
			if c == pluginv1.CommandKind_COMMAND_KIND_UNSPECIFIED || pluginv1.CommandKind_name[int32(c)] == "" {
				return fmt.Errorf("device %q: unknown command kind %v", d.GetId(), c)
			}
		}
	}
	return nil
}

// dropDevices forgets the devices of a plugin that stops.
func (m *Manager) dropDevices(ctx context.Context, plugin string) {
	m.devMu.Lock()
	old := m.devices[plugin]
	delete(m.devices, plugin)
	m.devMu.Unlock()
	m.devicesChanged(ctx, sessionsOf(old))
}

func (m *Manager) devicesChanged(ctx context.Context, sessions []core.ID) {
	if len(sessions) > 0 && m.cfg.DevicesChanged != nil {
		m.log.DebugContext(ctx, "devices changed", "sessions", len(sessions))
		m.cfg.DevicesChanged(sessions)
	}
}

// sessionsOf returns the sessions of the devices of lists, once each.
func sessionsOf(lists ...[]Device) []core.ID {
	var out []core.ID
	for _, list := range lists {
		for _, d := range list {
			if !slices.Contains(out, d.Session) {
				out = append(out, d.Session)
			}
		}
	}
	return out
}

// Devices lists the devices of every plugin, by plugin and as listed.
func (m *Manager) Devices() []Device {
	m.devMu.Lock()
	defer m.devMu.Unlock()
	var out []Device
	for _, list := range m.devices {
		out = append(out, list...)
	}
	slices.SortStableFunc(out, func(a, b Device) int { return cmp.Compare(a.Plugin, b.Plugin) })
	return out
}

// Device returns the device of a session.
func (m *Manager) Device(session core.ID) (Device, bool) {
	m.devMu.Lock()
	defer m.devMu.Unlock()
	for _, list := range m.devices {
		if i := slices.IndexFunc(list, func(d Device) bool { return d.Session == session }); i >= 0 {
			return list[i], true
		}
	}
	return Device{}, false
}

// IsDevice reports whether a session is a plugin's device.
func (m *Manager) IsDevice(session core.ID) bool {
	_, ok := m.Device(session)
	return ok
}

// pluginDevice returns a plugin's device by its ID within the plugin.
func (m *Manager) pluginDevice(plugin, id string) (Device, bool) {
	m.devMu.Lock()
	defer m.devMu.Unlock()
	list := m.devices[plugin]
	if i := slices.IndexFunc(list, func(d Device) bool { return d.ID == id }); i >= 0 {
		return list[i], true
	}
	return Device{}, false
}

// deviceSession returns the session a plugin acts in when it names one of
// its devices.
func (m *Manager) deviceSession(plugin, id string) (core.AuthSession, bool) {
	d, ok := m.pluginDevice(plugin, id)
	if !ok {
		return core.AuthSession{}, false
	}
	return core.AuthSession{ID: d.Session, DeviceID: d.ID, DeviceName: d.Name, Client: d.Product, LastSeenAt: d.Listed}, true
}

// commandKind returns the kind of a command.
func commandKind(c *pluginv1.Command) pluginv1.CommandKind {
	switch {
	case c.HasPlay():
		return pluginv1.CommandKind_COMMAND_KIND_PLAY
	case c.HasPlayState():
		return pluginv1.CommandKind_COMMAND_KIND_PLAY_STATE
	case c.HasSeek():
		return pluginv1.CommandKind_COMMAND_KIND_SEEK
	case c.HasMessage():
		return pluginv1.CommandKind_COMMAND_KIND_MESSAGE
	}
	return pluginv1.CommandKind_COMMAND_KIND_UNSPECIFIED
}

// SendCommand has the plugin controlling a device carry out a command a
// user sent. The plugin's errors are the caller's.
func (m *Manager) SendCommand(ctx context.Context, session core.ID, from core.User, c *pluginv1.Command) error {
	d, ok := m.Device(session)
	if !ok {
		return connect.NewError(connect.CodeNotFound, fmt.Errorf("no device %s", session))
	}
	if !slices.Contains(d.Commands, commandKind(c)) {
		return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("device %s does not take that command", d.Name))
	}
	pl, ok := m.running(d.Plugin)
	if !ok || pl.Devices() == nil {
		return connect.NewError(connect.CodeUnavailable, fmt.Errorf("plugin %s is not ready", d.Plugin))
	}
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	_, err := pl.Devices().SendCommand(ctx, pluginv1.SendCommandRequest_builder{
		DeviceId: &d.ID, SessionId: new(d.Session.String()), UserId: new(from.ID.String()), UserName: &from.Name, Command: c,
	}.Build())
	if err != nil {
		return fmt.Errorf("device %s: %w", d.Name, err)
	}
	return nil
}
