package containers

import (
	"context"
	"net/http"
	"net/url"
)

// Volume is what the engine tells of a volume.
type Volume struct {
	Name       string
	Labels     map[string]string
	Mountpoint string
}

// ListVolumes lists the volumes that carry all the labels.
func (c *Client) ListVolumes(ctx context.Context, labels map[string]string) ([]Volume, error) {
	var out struct{ Volumes []Volume }
	err := c.call(ctx, http.MethodGet, "/volumes", labelFilters(labels), nil, &out)
	return out.Volumes, err
}

// RemoveVolume removes a volume; core.ErrNotFound when there is none,
// core.ErrConflict while a container uses it.
func (c *Client) RemoveVolume(ctx context.Context, name string) error {
	return c.call(ctx, http.MethodDelete, "/volumes/"+url.PathEscape(name), nil, nil, nil)
}
