package rclone

import (
	"cmp"
	"context"
	"slices"
)

// Provider is a kind of remote, as config/providers lists them.
type Provider struct {
	Name        string   `json:"Name"`
	Description string   `json:"Description"`
	Prefix      string   `json:"Prefix"`
	Options     []Option `json:"Options"`
}

// Option is a setting of a provider, or a question of its configuration.
type Option struct {
	Name       string    `json:"Name"`
	Help       string    `json:"Help"`
	Provider   string    `json:"Provider"`
	DefaultStr string    `json:"DefaultStr"`
	Type       string    `json:"Type"`
	Required   bool      `json:"Required"`
	IsPassword bool      `json:"IsPassword"`
	Advanced   bool      `json:"Advanced"`
	Sensitive  bool      `json:"Sensitive"`
	Exclusive  bool      `json:"Exclusive"`
	Hide       int       `json:"Hide"`
	Examples   []Example `json:"Examples"`
}

// Example is a suggested value of an option.
type Example struct {
	Value    string `json:"Value"`
	Help     string `json:"Help"`
	Provider string `json:"Provider"`
}

// Providers lists the kinds of remotes the rcd knows.
func (c *Client) Providers(ctx context.Context) ([]Provider, error) {
	var out struct {
		Providers []Provider `json:"providers"`
	}
	if err := c.Call(ctx, "config/providers", map[string]any{}, &out); err != nil {
		return nil, err
	}
	return out.Providers, nil
}

// Remote is a configured remote.
type Remote struct {
	Name string
	Type string
	// Parameters are the remote's settings, passwords obscured.
	Parameters map[string]string
}

// Remotes lists the configured remotes, by name.
func (c *Client) Remotes(ctx context.Context) ([]Remote, error) {
	var dump map[string]map[string]string
	if err := c.Call(ctx, "config/dump", map[string]any{}, &dump); err != nil {
		return nil, err
	}
	out := make([]Remote, 0, len(dump))
	for name, params := range dump {
		r := Remote{Name: name, Type: params["type"], Parameters: map[string]string{}}
		for k, v := range params {
			if k != "type" {
				r.Parameters[k] = v
			}
		}
		out = append(out, r)
	}
	slices.SortFunc(out, func(a, b Remote) int { return cmp.Compare(a.Name, b.Name) })
	return out, nil
}

// ConfigStep is where a remote's configuration stands: done when State is
// empty, else a question to answer with Continue.
type ConfigStep struct {
	State  string  `json:"State"`
	Option *Option `json:"Option"`
	Error  string  `json:"Error"`
}

// Done reports whether the configuration needs no more answers.
func (s ConfigStep) Done() bool { return s.State == "" }

// CreateRemote configures a new remote of a provider. Passwords in
// parameters are given in clear and obscured by the rcd.
func (c *Client) CreateRemote(ctx context.Context, name, provider string, parameters map[string]string) (ConfigStep, error) {
	var step ConfigStep
	err := c.Call(ctx, "config/create", map[string]any{
		"name":       name,
		"type":       provider,
		"parameters": parameters,
		"opt":        map[string]any{"nonInteractive": true, "obscure": true},
	}, &step)
	return step, err
}

// UpdateRemote changes settings of a remote.
func (c *Client) UpdateRemote(ctx context.Context, name string, parameters map[string]string) (ConfigStep, error) {
	var step ConfigStep
	err := c.Call(ctx, "config/update", map[string]any{
		"name":       name,
		"parameters": parameters,
		"opt":        map[string]any{"nonInteractive": true, "obscure": true},
	}, &step)
	return step, err
}

// Continue answers the question of a configuration step, such as the token
// of an OAuth authorization made with "rclone authorize".
func (c *Client) Continue(ctx context.Context, name, state, answer string) (ConfigStep, error) {
	var step ConfigStep
	err := c.Call(ctx, "config/update", map[string]any{
		"name":       name,
		"parameters": map[string]string{},
		"opt":        map[string]any{"nonInteractive": true, "obscure": true, "continue": true, "state": state, "result": answer},
	}, &step)
	return step, err
}

// DeleteRemote removes a remote's configuration.
func (c *Client) DeleteRemote(ctx context.Context, name string) error {
	return c.Call(ctx, "config/delete", map[string]any{"name": name}, nil)
}
