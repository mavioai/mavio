package acquisition

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/apps/server/internal/downloads"
	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// listedClients is a download client provider that only lists clients.
type listedClients struct {
	pluginv1connect.DownloadClientProviderServiceClient
	clients []*pluginv1.DownloadClient
}

func (l *listedClients) ListDownloadClients(context.Context, *pluginv1.ListDownloadClientsRequest) (*pluginv1.ListDownloadClientsResponse, error) {
	return pluginv1.ListDownloadClientsResponse_builder{Clients: l.clients}.Build(), nil
}

// clientSetup builds the clients of Radarr's DownloadClientProviderFixture:
// their IDs are "1", "2"... in the order they are added.
type clientSetup struct {
	now     time.Time
	clients []*pluginv1.DownloadClient
}

func (s *clientSetup) add(protocol pluginv1.DownloadProtocol, priority int32, tags ...string) {
	id := fmt.Sprint(len(s.clients) + 1)
	s.clients = append(s.clients, pluginv1.DownloadClient_builder{
		Id: new(id), Name: new("Client " + id), Protocol: new(protocol), Enabled: new(true), Priority: new(priority), Tags: tags,
	}.Build())
}

func (s *clientSetup) usenet(priority int32, tags ...string) {
	s.add(pluginv1.DownloadProtocol_DOWNLOAD_PROTOCOL_USENET, priority, tags...)
}

func (s *clientSetup) torrent(priority int32, tags ...string) {
	s.add(pluginv1.DownloadProtocol_DOWNLOAD_PROTOCOL_TORRENT, priority, tags...)
}

func (s *clientSetup) blocked(id int) {
	s.clients[id-1].SetDisabledUntil(timestamppb.New(s.now.Add(3 * time.Hour)))
}

func (s *clientSetup) runtime() *Runtime {
	p := &listedClients{clients: s.clients}
	return &Runtime{
		Clients: func() []downloads.Provider { return []downloads.Provider{{ID: "dc", Client: p}} },
		Now:     func() time.Time { return s.now },
	}
}

// TestChooseClient ports Radarr's DownloadClientProviderFixture.
func TestChooseClient(t *testing.T) {
	usenet, torrent := core.ProtocolUsenet, core.ProtocolTorrent
	type pick struct {
		protocol core.Protocol
		tags     []string
	}
	repeat := func(n int, p pick) []pick { return slices.Repeat([]pick{p}, n) }
	tests := []struct {
		name  string
		setup func(s *clientSetup)
		picks []pick
		want  []string
	}{
		{
			"round robin over usenet clients", func(s *clientSetup) { s.usenet(0); s.usenet(0); s.usenet(0); s.torrent(0) },
			repeat(5, pick{protocol: usenet}),
			[]string{"1", "2", "3", "1", "2"},
		},
		{
			"round robin over torrent clients", func(s *clientSetup) { s.usenet(0); s.torrent(0); s.torrent(0); s.torrent(0) },
			repeat(5, pick{protocol: torrent}),
			[]string{"2", "3", "4", "2", "3"},
		},
		{
			"round robin over protocols separately", func(s *clientSetup) { s.usenet(0); s.torrent(0); s.torrent(0) },
			[]pick{{protocol: usenet}, {protocol: torrent}, {protocol: torrent}, {protocol: torrent}},
			[]string{"1", "2", "3", "2"},
		},
		{
			"round robin over clients with matching tags", func(s *clientSetup) { s.torrent(0); s.torrent(0, "a"); s.torrent(0); s.torrent(0, "a") },
			repeat(4, pick{torrent, []string{"a"}}),
			[]string{"2", "4", "2", "4"},
		},
		{
			"round robin over untagged clients when no tags match", func(s *clientSetup) { s.torrent(0); s.torrent(0, "a"); s.torrent(0); s.torrent(0, "a") },
			repeat(4, pick{torrent, []string{"b"}}),
			[]string{"1", "3", "1", "3"},
		},
		{
			"skip blocked torrent client", func(s *clientSetup) { s.usenet(0); s.torrent(0); s.torrent(0); s.torrent(0); s.blocked(3) },
			repeat(4, pick{protocol: torrent}),
			[]string{"2", "4", "2", "4"},
		},
		{
			"skip secondary priority torrent client", func(s *clientSetup) { s.usenet(0); s.torrent(2); s.torrent(0); s.torrent(0) },
			repeat(4, pick{protocol: torrent}),
			[]string{"3", "4", "3", "4"},
		},
		{"secondary priority torrent clients when the primary is blocked", func(s *clientSetup) {
			s.usenet(0)
			s.torrent(2)
			s.torrent(2)
			s.torrent(0)
			s.blocked(4)
		}, repeat(4, pick{protocol: torrent}), []string{"2", "3", "2", "3"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &clientSetup{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
			tt.setup(s)
			r := s.runtime()
			var got []string
			for _, p := range tt.picks {
				c, err := r.chooseClient(t.Context(), p.protocol, p.tags, "", "")
				if err != nil {
					t.Fatalf("chooseClient(%s, %v) error = %v", p.protocol, p.tags, err)
				}
				got = append(got, c.GetId())
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("chosen clients = %v, want = %v", got, tt.want)
			}
		})
	}
}

func TestChooseClientFails(t *testing.T) {
	tests := []struct {
		name  string
		setup func(s *clientSetup)
	}{
		// Radarr's should_fail_to_choose_when_clients_have_tags_but_no_match.
		{"clients have tags but none matches", func(s *clientSetup) { s.torrent(0, "a") }},
		{"no client of the protocol", func(s *clientSetup) { s.usenet(0) }},
		// Radarr tries blocked clients when all are, outside pending
		// releases; here the release goes pending until one is back.
		{"all clients blocked", func(s *clientSetup) { s.torrent(0); s.blocked(1) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &clientSetup{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
			tt.setup(s)
			_, err := s.runtime().chooseClient(t.Context(), core.ProtocolTorrent, []string{"b"}, "", "")
			if !errors.Is(err, errClientUnavailable) {
				t.Errorf("chooseClient error = %v, want = %v", err, errClientUnavailable)
			}
		})
	}
}

func TestChooseRequestedClient(t *testing.T) {
	s := &clientSetup{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	s.torrent(0)
	s.torrent(0, "a")
	r := s.runtime()
	// A requested client is taken whatever its tags, as Radarr's
	// ReleaseController with a download client ID.
	c, err := r.chooseClient(t.Context(), core.ProtocolTorrent, nil, "dc", "2")
	if err != nil || c.GetId() != "2" {
		t.Errorf("chooseClient(dc/2) = %v, %v, want = 2", c.GetId(), err)
	}
	if _, err := r.chooseClient(t.Context(), core.ProtocolTorrent, nil, "dc", "9"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("chooseClient(dc/9) error = %v, want = %v", err, core.ErrNotFound)
	}
}
