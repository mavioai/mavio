//go:build !darwin && !linux && !windows

package storage

import "context"

// volumes lists nothing where volumes are not read.
func volumes(context.Context) ([]Volume, error) { return nil, nil }
