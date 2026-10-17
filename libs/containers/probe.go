package containers

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
)

// Probe checks that the service at a base URL is ready: with a health
// path, GET of it answers 2xx; without one, the URL's port accepts
// connections. client nil uses http.DefaultClient; ctx bounds the check.
func Probe(ctx context.Context, client *http.Client, address, path string) error {
	u, err := url.Parse(address)
	if err != nil {
		return err
	}
	if path == "" {
		host := u.Host
		if u.Port() == "" {
			port := "80"
			if u.Scheme == "https" {
				port = "443"
			}
			host = net.JoinHostPort(u.Hostname(), port)
		}
		var d net.Dialer
		conn, err := d.DialContext(ctx, "tcp", host)
		if err != nil {
			return err
		}
		return conn.Close()
	}
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address+path, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("GET %s: %s", path, resp.Status)
	}
	return nil
}
