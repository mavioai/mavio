package core

import (
	"errors"
	"testing"
)

func TestPluginContainerValidate(t *testing.T) {
	tests := []struct {
		name string
		c    PluginContainer
		ok   bool
	}{
		{"unset", PluginContainer{PluginID: "dev.mavio.voice", Name: "whisper", Mode: ContainerUnset}, true},
		{"managed", PluginContainer{PluginID: "dev.mavio.voice", Name: "whisper", Mode: ContainerManaged, UseGPU: true}, true},
		{"external", PluginContainer{PluginID: "dev.mavio.voice", Name: "whisper", Mode: ContainerExternal, Address: "http://10.0.0.2:9000"}, true},
		{"external with path", PluginContainer{PluginID: "dev.mavio.voice", Name: "whisper", Mode: ContainerExternal, Address: "https://asr.example.com/v1"}, true},
		{"managed through a proxy", PluginContainer{PluginID: "dev.mavio.voice", Name: "whisper", Mode: ContainerManaged, Proxy: "01a14422-45e1-730c-a64f-a7e1a6b5e6b7"}, true},
		{"managed through the system proxy", PluginContainer{PluginID: "dev.mavio.voice", Name: "whisper", Mode: ContainerManaged, Proxy: SystemProxyID}, true},
		{"unknown proxy", PluginContainer{PluginID: "dev.mavio.voice", Name: "whisper", Mode: ContainerManaged, Proxy: "office"}, false},
		{"no plugin", PluginContainer{Name: "whisper", Mode: ContainerUnset}, false},
		{"no name", PluginContainer{PluginID: "dev.mavio.voice", Mode: ContainerUnset}, false},
		{"no mode", PluginContainer{PluginID: "dev.mavio.voice", Name: "whisper"}, false},
		{"unknown mode", PluginContainer{PluginID: "dev.mavio.voice", Name: "whisper", Mode: "compose"}, false},
		{"managed with address", PluginContainer{PluginID: "dev.mavio.voice", Name: "whisper", Mode: ContainerManaged, Address: "http://a:1"}, false},
		{"external without address", PluginContainer{PluginID: "dev.mavio.voice", Name: "whisper", Mode: ContainerExternal}, false},
		{"external relative", PluginContainer{PluginID: "dev.mavio.voice", Name: "whisper", Mode: ContainerExternal, Address: "asr:9000"}, false},
		{"external other scheme", PluginContainer{PluginID: "dev.mavio.voice", Name: "whisper", Mode: ContainerExternal, Address: "tcp://asr:9000"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.c.Validate()
			if tt.ok && err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
			if !tt.ok && !errors.Is(err, ErrInvalid) {
				t.Errorf("Validate() = %v, want ErrInvalid", err)
			}
		})
	}
}
