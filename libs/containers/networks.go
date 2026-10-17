package containers

import (
	"context"
	"net/http"
)

// Network is what the engine tells of a network.
type Network struct {
	ID         string `json:"Id"`
	Name       string
	Driver     string
	Labels     map[string]string
	Containers map[string]struct{ Name string }
}

// CreateNetwork creates a bridge network and returns its ID;
// core.ErrConflict when one of the name exists. Engines before API 1.44
// allow networks of the same name unless asked to check, which later
// ones always do.
func (c *Client) CreateNetwork(ctx context.Context, name string, labels map[string]string) (string, error) {
	body := struct {
		Name           string
		Driver         string
		CheckDuplicate bool
		Labels         map[string]string `json:",omitempty"`
	}{name, "bridge", true, labels}
	var out struct {
		ID string `json:"Id"`
	}
	err := c.call(ctx, http.MethodPost, "/networks/create", nil, body, &out)
	return out.ID, err
}

// InspectNetwork returns a network by name or ID; core.ErrNotFound when
// there is none.
func (c *Client) InspectNetwork(ctx context.Context, id string) (Network, error) {
	var n Network
	err := c.call(ctx, http.MethodGet, "/networks/"+id, nil, nil, &n)
	return n, err
}

// ConnectNetwork connects a container to a network, under aliases there.
func (c *Client) ConnectNetwork(ctx context.Context, network, container string, aliases []string) error {
	body := struct {
		Container      string
		EndpointConfig endpointConfig
	}{container, endpointConfig{Aliases: aliases}}
	return c.call(ctx, http.MethodPost, "/networks/"+network+"/connect", nil, body, nil)
}

// RemoveNetwork removes a network; core.ErrNotFound when there is none.
func (c *Client) RemoveNetwork(ctx context.Context, id string) error {
	return c.call(ctx, http.MethodDelete, "/networks/"+id, nil, nil, nil)
}
