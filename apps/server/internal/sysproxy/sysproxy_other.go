//go:build !darwin && !windows

package sysproxy

import "context"

// fromOS finds none: elsewhere the environment sets the system proxy.
func fromOS(context.Context) (Setting, error) { return Setting{}, nil }
