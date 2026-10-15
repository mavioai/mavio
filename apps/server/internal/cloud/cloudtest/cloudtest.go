// Package cloudtest simulates mavio.dev's API for servers in tests: it
// checks the JWTs servers sign, claims servers with claim codes and claim
// requests, keeps their changes, libraries and connections, sends notices on their WebSockets, and
// signs sign-in assertions with the key it publishes.
package cloudtest

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/net/websocket"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/apps/server/internal/cloud"
	"github.com/mavioai/mavio/apps/server/internal/identity"
	cloudv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/cloud/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/cloud/v1/cloudv1connect"
)

// Server is a simulated mavio.dev.
type Server struct {
	// URL is the API's base URL.
	URL string

	key ed25519.PrivateKey

	mu       sync.Mutex
	codes    map[string]*cloudv1.Account
	requests map[string]*claimRequest
	servers  map[string]*server
	relays   []*cloudv1.Relay
}

// claimRequest is a claim request, with the account that confirmed it.
type claimRequest struct {
	serverID string
	key      ed25519.PublicKey
	request  *cloudv1.CreateClaimRequestRequest
	account  *cloudv1.Account
}

type server struct {
	key       ed25519.PublicKey
	changes   []*cloudv1.Change
	libraries []*cloudv1.Library
	urls      []string
	txt       []string
	region    string
	conns     map[*websocket.Conn]struct{}
}

// BindingKeyID is the key ID of the external account bindings the
// simulated mavio.dev gives.
const BindingKeyID = "test-binding"

// keyID is the ID of the key assertions are signed with.
const keyID = "test-key"

type tokenKey struct{}

// New starts a simulated mavio.dev, stopped at the end of the test.
func New(t testing.TB) *Server {
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	s := &Server{key: key, codes: map[string]*cloudv1.Account{}, requests: map[string]*claimRequest{}, servers: map[string]*server{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api"+cloud.JWKSPath, s.jwks)
	path, h := cloudv1connect.NewServerServiceHandler(s)
	mux.Handle("/api"+path, http.StripPrefix("/api", h))
	mux.Handle("/api"+cloud.NoticesPath, websocket.Server{Handshake: s.handshake, Handler: s.notices})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		mux.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), tokenKey{}, token)))
	}))
	t.Cleanup(srv.Close)
	s.URL = srv.URL + "/api"
	return s
}

// AddClaimCode adds a claim code of account, valid for one claim.
func (s *Server) AddClaimCode(code string, account *cloudv1.Account) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.codes[code] = account
}

// ClaimRequest returns the claim request with an ID, as the server asked
// for it, and whether it exists.
func (s *Server) ClaimRequest(id string) (*cloudv1.CreateClaimRequestRequest, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.requests[id]
	if !ok {
		return nil, false
	}
	return r.request, true
}

// ConfirmClaimRequest confirms the claim request with an ID for account,
// reporting whether it exists.
func (s *Server) ConfirmClaimRequest(id string, account *cloudv1.Account) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.requests[id]
	if ok {
		r.account = account
		s.register(r.serverID, r.key)
	}
	return ok
}

// URLs returns the connections the server with an ID published.
func (s *Server) URLs(serverID string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if srv := s.servers[serverID]; srv != nil {
		return srv.urls
	}
	return nil
}

// AddChange adds a change for the server with an ID, nil for one of no
// kind, and notices its connections, returning its cursor.
func (s *Server) AddChange(serverID string, change *cloudv1.Change) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	srv := s.servers[serverID]
	if srv == nil {
		return 0
	}
	cursor := int64(len(srv.changes) + 1)
	if change == nil {
		change = &cloudv1.Change{}
	} else {
		change = proto.CloneOf(change)
	}
	change.SetCursor(cursor)
	change.SetCreateTime(timestamppb.Now())
	srv.changes = append(srv.changes, change)
	frame, _ := proto.Marshal(cloudv1.Notice_builder{Cursor: &cursor}.Build())
	for conn := range srv.conns {
		_ = websocket.Message.Send(conn, frame)
	}
	return cursor
}

// Libraries returns the libraries the server with an ID published.
func (s *Server) Libraries(serverID string) []*cloudv1.Library {
	s.mu.Lock()
	defer s.mu.Unlock()
	if srv := s.servers[serverID]; srv != nil {
		return srv.libraries
	}
	return nil
}

// Assertion returns a sign-in assertion for an account on the server
// with an ID, for a device's public key, valid for two minutes.
func (s *Server) Assertion(serverID, accountID string, device ed25519.PublicKey) string {
	now := time.Now()
	return s.Sign(cloud.AssertionClaims{
		Issuer: s.URL, Audience: serverID, Subject: accountID,
		IssuedAt: now.Unix(), ExpiresAt: now.Add(cloud.AssertionLifetime).Unix(),
		DeviceKey: base64.RawURLEncoding.EncodeToString(device), Nonce: rand.Text(),
	})
}

// Sign signs claims as a JWT with the key mavio.dev publishes.
func (s *Server) Sign(claims any) string {
	header, _ := json.Marshal(cloud.Header{Algorithm: "EdDSA", Type: "JWT", KeyID: keyID})
	body, _ := json.Marshal(claims)
	enc := base64.RawURLEncoding
	signed := enc.EncodeToString(header) + "." + enc.EncodeToString(body)
	return signed + "." + enc.EncodeToString(ed25519.Sign(s.key, []byte(signed)))
}

func (s *Server) jwks(w http.ResponseWriter, _ *http.Request) {
	x := base64.RawURLEncoding.EncodeToString(s.key.Public().(ed25519.PublicKey))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"keys": []map[string]string{{"kty": "OKP", "crv": "Ed25519", "x": x, "kid": keyID, "alg": "EdDSA"}},
	})
}

// PublishLibraries replaces a claimed server's libraries.
func (s *Server) PublishLibraries(ctx context.Context, req *cloudv1.PublishLibrariesRequest) (*cloudv1.PublishLibrariesResponse, error) {
	srv, err := s.claimed(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	srv.libraries = req.GetLibraries()
	return &cloudv1.PublishLibrariesResponse{}, nil
}

// PublishConnections replaces a claimed server's URLs.
func (s *Server) PublishConnections(ctx context.Context, req *cloudv1.PublishConnectionsRequest) (*cloudv1.PublishConnectionsResponse, error) {
	srv, err := s.claimed(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	srv.urls = req.GetUrls()
	return &cloudv1.PublishConnectionsResponse{}, nil
}

// SetAcmeChallenge keeps a claimed server's TXT values.
func (s *Server) SetAcmeChallenge(ctx context.Context, req *cloudv1.SetAcmeChallengeRequest) (*cloudv1.SetAcmeChallengeResponse, error) {
	srv, err := s.claimed(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	srv.txt = req.GetValues()
	return &cloudv1.SetAcmeChallengeResponse{}, nil
}

// ClearAcmeChallenge removes a claimed server's TXT values.
func (s *Server) ClearAcmeChallenge(ctx context.Context, _ *cloudv1.ClearAcmeChallengeRequest) (*cloudv1.ClearAcmeChallengeResponse, error) {
	srv, err := s.claimed(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	srv.txt = nil
	return &cloudv1.ClearAcmeChallengeResponse{}, nil
}

// CreateAcmeBinding gives a binding with BindingKeyID.
func (s *Server) CreateAcmeBinding(ctx context.Context, _ *cloudv1.CreateAcmeBindingRequest) (*cloudv1.CreateAcmeBindingResponse, error) {
	if _, err := s.claimed(ctx); err != nil {
		return nil, err
	}
	resp := &cloudv1.CreateAcmeBindingResponse{}
	resp.SetKeyId(BindingKeyID)
	resp.SetHmacKey([]byte("binding mac key"))
	return resp, nil
}

// SetRelays sets the relays ListRelays gives, base URLs by region.
func (s *Server) SetRelays(urls map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.relays = nil
	for region, url := range urls {
		s.relays = append(s.relays, cloudv1.Relay_builder{Region: &region, Url: &url}.Build())
	}
}

// ListRelays gives the relays SetRelays set.
func (s *Server) ListRelays(ctx context.Context, _ *cloudv1.ListRelaysRequest) (*cloudv1.ListRelaysResponse, error) {
	if _, err := s.claimed(ctx); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloudv1.ListRelaysResponse_builder{Relays: s.relays}.Build(), nil
}

// SetRelayRegion records a claimed server's relay region.
func (s *Server) SetRelayRegion(ctx context.Context, req *cloudv1.SetRelayRegionRequest) (*cloudv1.SetRelayRegionResponse, error) {
	srv, err := s.claimed(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	srv.region = req.GetRegion()
	return &cloudv1.SetRelayRegionResponse{}, nil
}

// RelayRegion returns the relay region the server with an ID recorded.
func (s *Server) RelayRegion(serverID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if srv := s.servers[serverID]; srv != nil {
		return srv.region
	}
	return ""
}

// TXT returns the TXT values at a name of mavio.site, as its name servers
// answer them through mavio.dev: _acme-challenge.<ID>.mavio.site.
func (s *Server) TXT(name string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := strings.CutPrefix(strings.TrimSuffix(name, ".mavio.site"), "_acme-challenge.")
	if srv := s.servers[id]; ok && srv != nil {
		return srv.txt
	}
	return nil
}

// Connections returns the number of WebSockets of the server with an ID.
func (s *Server) Connections(serverID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if srv := s.servers[serverID]; srv != nil {
		return len(srv.conns)
	}
	return 0
}

// Disconnect closes the WebSockets of the server with an ID.
func (s *Server) Disconnect(serverID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if srv := s.servers[serverID]; srv != nil {
		for conn := range srv.conns {
			_ = conn.Close()
		}
	}
}

// CreateClaimRequest keeps a claim request until a test confirms it.
func (s *Server) CreateClaimRequest(ctx context.Context, req *cloudv1.CreateClaimRequestRequest) (*cloudv1.CreateClaimRequestResponse, error) {
	id, err := s.verify(ctx, req.GetPublicKey())
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	requestID := rand.Text()
	s.requests[requestID] = &claimRequest{serverID: id, key: req.GetPublicKey(), request: req}
	return cloudv1.CreateClaimRequestResponse_builder{
		ClaimRequest: &requestID,
		Url:          new(strings.TrimSuffix(s.URL, "/api") + "/claim?request=" + requestID),
		ExpireTime:   timestamppb.New(time.Now().Add(15 * time.Minute)),
	}.Build(), nil
}

// ClaimServer claims a server with a claim code or a confirmed claim
// request.
func (s *Server) ClaimServer(ctx context.Context, req *cloudv1.ClaimServerRequest) (*cloudv1.ClaimServerResponse, error) {
	id, err := s.verify(ctx, req.GetPublicKey())
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var account *cloudv1.Account
	if r := req.GetClaimRequest(); r != "" {
		cr, ok := s.requests[r]
		if !ok || cr.serverID != id {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("unknown claim request"))
		}
		if cr.account == nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the claim request is not confirmed yet"))
		}
		account = cr.account
	} else {
		var ok bool
		if account, ok = s.codes[req.GetClaimCode()]; !ok {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("unknown claim code"))
		}
		delete(s.codes, req.GetClaimCode())
	}
	srv := s.register(id, req.GetPublicKey())
	cursor := int64(len(srv.changes))
	return cloudv1.ClaimServerResponse_builder{Administrator: account, Cursor: &cursor}.Build(), nil
}

// register returns the server with an ID, registering it first.
func (s *Server) register(id string, key ed25519.PublicKey) *server {
	srv := s.servers[id]
	if srv == nil {
		srv = &server{key: key, conns: map[*websocket.Conn]struct{}{}}
		s.servers[id] = srv
	}
	return srv
}

// ListChanges gives a claimed server's changes after a cursor.
func (s *Server) ListChanges(ctx context.Context, req *cloudv1.ListChangesRequest) (*cloudv1.ListChangesResponse, error) {
	srv, err := s.claimed(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	size := int(req.GetPageSize())
	if size == 0 {
		size = 100
	}
	cursor := req.GetAfter()
	var changes []*cloudv1.Change
	for _, c := range srv.changes {
		if c.GetCursor() > req.GetAfter() && len(changes) < size {
			changes = append(changes, c)
			cursor = c.GetCursor()
		}
	}
	more := int64(len(srv.changes)) > cursor
	return cloudv1.ListChangesResponse_builder{Changes: changes, More: &more, Cursor: &cursor}.Build(), nil
}

func (s *Server) handshake(cfg *websocket.Config, r *http.Request) error {
	_, err := s.claimed(r.Context())
	return err
}

func (s *Server) notices(conn *websocket.Conn) {
	srv, err := s.claimed(conn.Request().Context())
	if err != nil {
		return
	}
	s.mu.Lock()
	srv.conns[conn] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(srv.conns, conn)
		s.mu.Unlock()
	}()
	// Reading answers the server's pings until it disconnects.
	for {
		var frame []byte
		if err := websocket.Message.Receive(conn, &frame); err != nil {
			return
		}
	}
}

// claimed returns the claimed server whose JWT signed the request.
func (s *Server) claimed(ctx context.Context) (*server, error) {
	header, _, err := parse(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	srv := s.servers[header.KeyID]
	s.mu.Unlock()
	if srv == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unknown server"))
	}
	if _, err := s.verify(ctx, srv.key); err != nil {
		return nil, err
	}
	return srv, nil
}

// verify checks the request's JWT against the server's public key and
// returns the server's ID.
func (s *Server) verify(ctx context.Context, key ed25519.PublicKey) (string, error) {
	header, claims, err := parse(ctx)
	if err != nil {
		return "", err
	}
	token, _ := ctx.Value(tokenKey{}).(string)
	i := strings.LastIndexByte(token, '.')
	raw, err := base64.RawURLEncoding.DecodeString(token[i+1:])
	id := identity.IDOf(key)
	now := time.Now().Unix()
	switch {
	case err != nil || len(key) != ed25519.PublicKeySize || !ed25519.Verify(key, []byte(token[:i]), raw):
		return "", connect.NewError(connect.CodeUnauthenticated, errors.New("bad signature"))
	case header.Algorithm != "EdDSA" || header.KeyID != id || claims.Issuer != id || claims.Subject != id:
		return "", connect.NewError(connect.CodeUnauthenticated, errors.New("token of another server"))
	case claims.Audience != s.URL:
		return "", connect.NewError(connect.CodeUnauthenticated, errors.New("token for another audience"))
	case claims.ExpiresAt < now || claims.ExpiresAt-claims.IssuedAt > int64(cloud.TokenLifetime/time.Second) || claims.ID == "":
		return "", connect.NewError(connect.CodeUnauthenticated, errors.New("expired token"))
	}
	return id, nil
}

func parse(ctx context.Context) (cloud.Header, cloud.Claims, error) {
	var (
		header cloud.Header
		claims cloud.Claims
	)
	token, _ := ctx.Value(tokenKey{}).(string)
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return header, claims, connect.NewError(connect.CodeUnauthenticated, errors.New("no token"))
	}
	for i, v := range []any{&header, &claims} {
		raw, err := base64.RawURLEncoding.DecodeString(parts[i])
		if err == nil {
			err = json.Unmarshal(raw, v)
		}
		if err != nil {
			return header, claims, connect.NewError(connect.CodeUnauthenticated, err)
		}
	}
	return header, claims, nil
}
