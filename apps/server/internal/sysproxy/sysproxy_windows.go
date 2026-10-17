package sysproxy

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/sys/windows/registry"
)

func fromOS(context.Context) (Setting, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return Setting{}, nil
	}
	if err != nil {
		return Setting{}, fmt.Errorf("internet settings: %w", err)
	}
	defer k.Close()
	if enabled, _, err := k.GetIntegerValue("ProxyEnable"); err != nil || enabled == 0 {
		return Setting{}, nil
	}
	server, _, err := k.GetStringValue("ProxyServer")
	if err != nil {
		return Setting{}, nil
	}
	override, _, _ := k.GetStringValue("ProxyOverride")
	return ParseWindows(server, override)
}
