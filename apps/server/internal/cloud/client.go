package cloud

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/crypto/acme"
	"golang.org/x/net/websocket"

	"github.com/mavioai/mavio/apps/server/internal/identity"
	cloudv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/cloud/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/cloud/v1/cloudv1connect"
)

// NoticesPath is the path of the notices' WebSocket under the API's base
// URL.
const NoticesPath = "/servers/notices"

// Client calls mavio.dev's API as the server, authenticated by its
// identity key.
type Client struct {
	base string
	key  *identity.Key
	api  cloudv1connect.ServerServiceClient
	now  func() time.Time
}

// NewClient returns a client of the API at base, such as
// https://mavio.dev/api, signing with key; a nil hc uses a client with a
// 30-second timeout.
func NewClient(base string, key *identity.Key, hc *http.Client) *Client {
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	c := &Client{base: strings.TrimSuffix(base, "/"), key: key, now: time.Now}
	sign := connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			req.Header().Set("Authorization", "Bearer "+c.token())
			return next(ctx, req)
		}
	})
	c.api = cloudv1connect.NewServerServiceClient(hc, c.base, connect.WithInterceptors(sign))
	return c
}

func (c *Client) token() string { return Token(c.key, c.base, c.now()) }

// ClaimServer registers the server under the account of a claim code.
func (c *Client) ClaimServer(ctx context.Context, req *cloudv1.ClaimServerRequest) (*cloudv1.ClaimServerResponse, error) {
	return c.api.ClaimServer(ctx, req)
}

// ListChanges gives the server's changes after a cursor.
func (c *Client) ListChanges(ctx context.Context, req *cloudv1.ListChangesRequest) (*cloudv1.ListChangesResponse, error) {
	return c.api.ListChanges(ctx, req)
}

// PublishLibraries replaces the libraries mavio.dev knows of the server.
func (c *Client) PublishLibraries(ctx context.Context, req *cloudv1.PublishLibrariesRequest) (*cloudv1.PublishLibrariesResponse, error) {
	return c.api.PublishLibraries(ctx, req)
}

// Notices opens the WebSocket on which mavio.dev sends Notices.
func (c *Client) Notices(ctx context.Context) (*websocket.Conn, error) {
	u, err := url.Parse(c.base + NoticesPath)
	if err != nil {
		return nil, fmt.Errorf("notices URL: %w", err)
	}
	origin := u.Scheme + "://" + u.Host
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	default:
		return nil, errors.New("mavio.dev's URL is neither HTTPS nor HTTP")
	}
	cfg, err := websocket.NewConfig(u.String(), origin)
	if err != nil {
		return nil, fmt.Errorf("notices: %w", err)
	}
	cfg.Header = http.Header{"Authorization": {"Bearer " + c.token()}}
	conn, err := cfg.DialContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("connect to mavio.dev: %w", err)
	}
	return conn, nil
}

// SetAcmeChallenge has mavio.dev publish the TXT values of the server's
// DNS-01 challenge.
func (c *Client) SetAcmeChallenge(ctx context.Context, values []string) error {
	req := &cloudv1.SetAcmeChallengeRequest{}
	req.SetValues(values)
	_, err := c.api.SetAcmeChallenge(ctx, req)
	return err
}

// ClearAcmeChallenge has mavio.dev remove the challenge's TXT values.
func (c *Client) ClearAcmeChallenge(ctx context.Context) error {
	_, err := c.api.ClearAcmeChallenge(ctx, &cloudv1.ClearAcmeChallengeRequest{})
	return err
}

// AcmeBinding gets an external account binding from mavio.dev.
func (c *Client) AcmeBinding(ctx context.Context) (*acme.ExternalAccountBinding, error) {
	resp, err := c.api.CreateAcmeBinding(ctx, &cloudv1.CreateAcmeBindingRequest{})
	if err != nil {
		return nil, err
	}
	return &acme.ExternalAccountBinding{KID: resp.GetKeyId(), Key: resp.GetHmacKey()}, nil
}

// ListRelays gives mavio.dev's relays, one per region.
func (c *Client) ListRelays(ctx context.Context) ([]*cloudv1.Relay, error) {
	resp, err := c.api.ListRelays(ctx, &cloudv1.ListRelaysRequest{})
	if err != nil {
		return nil, err
	}
	return resp.GetRelays(), nil
}

// SetRelayRegion records the region of the relay the server keeps a
// connection to, empty for none.
func (c *Client) SetRelayRegion(ctx context.Context, region string) error {
	req := &cloudv1.SetRelayRegionRequest{}
	req.SetRegion(region)
	_, err := c.api.SetRelayRegion(ctx, req)
	return err
}
