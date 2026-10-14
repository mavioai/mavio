// Package cloudtest simulates mavio.dev's API for servers in tests: it
// checks the JWTs servers sign, claims servers with claim codes, keeps
// their changes and sends notices on their WebSockets.
package cloudtest

import (
	"context"
	"crypto/ed25519"
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

	mu      sync.Mutex
	codes   map[string]*cloudv1.Account
	servers map[string]*server
}

type server struct {
	key     ed25519.PublicKey
	changes []*cloudv1.Change
	conns   map[*websocket.Conn]struct{}
}

type tokenKey struct{}

// New starts a simulated mavio.dev, stopped at the end of the test.
func New(t testing.TB) *Server {
	s := &Server{codes: map[string]*cloudv1.Account{}, servers: map[string]*server{}}
	mux := http.NewServeMux()
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

// AddChange adds a change for the server with an ID and notices its
// connections, returning its cursor.
func (s *Server) AddChange(serverID string) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	srv := s.servers[serverID]
	if srv == nil {
		return 0
	}
	cursor := int64(len(srv.changes) + 1)
	srv.changes = append(srv.changes, cloudv1.Change_builder{Cursor: &cursor, CreateTime: timestamppb.Now()}.Build())
	frame, _ := proto.Marshal(cloudv1.Notice_builder{Cursor: &cursor}.Build())
	for conn := range srv.conns {
		_ = websocket.Message.Send(conn, frame)
	}
	return cursor
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

// ClaimServer claims a server with a claim code.
func (s *Server) ClaimServer(ctx context.Context, req *cloudv1.ClaimServerRequest) (*cloudv1.ClaimServerResponse, error) {
	id, err := s.verify(ctx, req.GetPublicKey())
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	account, ok := s.codes[req.GetClaimCode()]
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("unknown claim code"))
	}
	delete(s.codes, req.GetClaimCode())
	srv := s.servers[id]
	if srv == nil {
		srv = &server{key: req.GetPublicKey(), conns: map[*websocket.Conn]struct{}{}}
		s.servers[id] = srv
	}
	cursor := int64(len(srv.changes))
	return cloudv1.ClaimServerResponse_builder{Administrator: account, Cursor: &cursor}.Build(), nil
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
