package plugins

import (
	"strings"
	"testing"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

func TestValidateDevices(t *testing.T) {
	device := func(id, name string, commands ...pluginv1.CommandKind) *pluginv1.Device {
		return pluginv1.Device_builder{Id: &id, Name: &name, Commands: commands}.Build()
	}
	tv := device("tv", "TV", pluginv1.CommandKind_COMMAND_KIND_PLAY)
	tests := []struct {
		name string
		list []*pluginv1.Device
		want string // part of the error; empty for none
	}{
		{"valid", []*pluginv1.Device{tv, device("radio", "Radio")}, ""},
		{"none", nil, ""},
		{"no ID", []*pluginv1.Device{device("", "TV")}, "needs an ID"},
		{"no name", []*pluginv1.Device{device("tv", "")}, "needs an ID and a name"},
		{"long name", []*pluginv1.Device{device("tv", strings.Repeat("x", 257))}, "at most 256"},
		{"twice", []*pluginv1.Device{tv, tv}, "listed twice"},
		{"unknown command", []*pluginv1.Device{device("tv", "TV", 99)}, "unknown command"},
		{"unspecified command", []*pluginv1.Device{device("tv", "TV", 0)}, "unknown command"},
	}
	for _, tt := range tests {
		err := validateDevices(tt.list)
		if tt.want == "" && err != nil || tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)) {
			t.Errorf("%s: validateDevices() = %v, want %q", tt.name, err, tt.want)
		}
	}
	if err := validateDevices(make([]*pluginv1.Device, maxDevices+1)); err == nil {
		t.Error("validateDevices() accepted too many devices")
	}
}
