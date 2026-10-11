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
	"github.com/mavioai/mavio/apps/server/internal/releases"
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
		// indexerClient is the download client of the release's indexer.
		indexerClient string
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
			repeat(4, pick{protocol: torrent, tags: []string{"a"}}),
			[]string{"2", "4", "2", "4"},
		},
		{
			"round robin over untagged clients when no tags match", func(s *clientSetup) { s.torrent(0); s.torrent(0, "a"); s.torrent(0); s.torrent(0, "a") },
			repeat(4, pick{protocol: torrent, tags: []string{"b"}}),
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
		{"blocked torrent clients when all are", func(s *clientSetup) {
			s.usenet(0)
			s.torrent(0)
			s.torrent(0)
			s.torrent(0)
			s.blocked(2)
			s.blocked(3)
			s.blocked(4)
		}, repeat(4, pick{protocol: torrent}), []string{"2", "3", "4", "2"}},
		{
			"always the indexer's client", func(s *clientSetup) { s.usenet(0); s.torrent(0); s.torrent(0); s.torrent(0) },
			repeat(5, pick{protocol: torrent, indexerClient: "3"}),
			[]string{"3", "3", "3", "3", "3"},
		},
		{
			"the indexer's client named with its plugin", func(s *clientSetup) { s.torrent(0); s.torrent(0) },
			repeat(2, pick{protocol: torrent, indexerClient: "dc/2"}),
			[]string{"2", "2"},
		},
		{
			"the indexer's client even when blocked", func(s *clientSetup) { s.torrent(0); s.torrent(0); s.blocked(2) },
			repeat(2, pick{protocol: torrent, indexerClient: "2"}),
			[]string{"2", "2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &clientSetup{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
			tt.setup(s)
			r := s.runtime()
			var got []string
			for _, p := range tt.picks {
				c, err := r.chooseClient(t.Context(), clientChoice{protocol: p.protocol, tags: p.tags, indexerClient: p.indexerClient})
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
		// ch is the choice, of a torrent release for tag "b".
		ch clientChoice
	}{
		// Radarr's should_fail_to_choose_when_clients_have_tags_but_no_match.
		{"clients have tags but none matches", func(s *clientSetup) { s.torrent(0, "a") }, clientChoice{}},
		{"no client of the protocol", func(s *clientSetup) { s.usenet(0) }, clientChoice{}},
		// A release pending until a client is back waits while all are
		// blocked, as Radarr's DownloadService with
		// PendingReleaseReason.DownloadClientUnavailable.
		{"all clients blocked for a release waiting for one", func(s *clientSetup) { s.torrent(0); s.blocked(1) }, clientChoice{filterBlocked: true}},
		// Radarr's should_fail_to_choose_client_when_indexer_reference_does_not_exist.
		{"the indexer's client does not exist", func(s *clientSetup) { s.usenet(0); s.torrent(0); s.torrent(0); s.torrent(0) }, clientChoice{indexerClient: "5"}},
		{"the indexer's client is of another protocol", func(s *clientSetup) { s.usenet(0); s.torrent(0) }, clientChoice{indexerClient: "1"}},
		{"the indexer's client of another plugin", func(s *clientSetup) { s.torrent(0) }, clientChoice{indexerClient: "other/1"}},
		{"the indexer's client lacks the tags", func(s *clientSetup) { s.torrent(0, "b"); s.torrent(0) }, clientChoice{indexerClient: "2"}},
		{"the indexer's client blocked for a release waiting for one", func(s *clientSetup) { s.torrent(0); s.torrent(0); s.blocked(2) }, clientChoice{indexerClient: "2", filterBlocked: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &clientSetup{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
			tt.setup(s)
			ch := tt.ch
			ch.protocol, ch.tags = core.ProtocolTorrent, []string{"b"}
			_, err := s.runtime().chooseClient(t.Context(), ch)
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
	c, err := r.chooseClient(t.Context(), clientChoice{protocol: core.ProtocolTorrent, pluginID: "dc", clientID: "2", indexerClient: "1"})
	if err != nil || c.GetId() != "2" {
		t.Errorf("chooseClient(dc/2) = %v, %v, want = 2", c.GetId(), err)
	}
	if _, err := r.chooseClient(t.Context(), clientChoice{protocol: core.ProtocolTorrent, pluginID: "dc", clientID: "9"}); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("chooseClient(dc/9) error = %v, want = %v", err, core.ErrNotFound)
	}
}

// blockClient leaves the client out after failing for an hour.
func (e *env) blockClient() {
	e.client.mu.Lock()
	e.client.disabledUntil = e.clock.Add(time.Hour)
	e.client.mu.Unlock()
}

// TestGrabWithBlockedClient checks a grab tries a client left out after
// failing when every client is, but a release pending until a client is
// back waits, as Radarr's DownloadService.
func TestGrabWithBlockedClient(t *testing.T) {
	t.Run("search", func(t *testing.T) {
		e := newEnv(t)
		it := e.addMovie()
		e.blockClient()
		e.indexer.set(e.both(), nil)
		if err := e.rt.SearchMovie(e.ctx, it.ID, false); err != nil {
			t.Fatal(err)
		}
		if got := e.client.titles(); !slices.Equal(got, []string{title1080}) {
			t.Errorf("grabbed = %v, want = [%s] through the blocked client", got, title1080)
		}
	})
	t.Run("pending", func(t *testing.T) {
		e := newEnv(t)
		it := e.addMovie()
		e.blockClient()
		rel := FromSearch(releases.Release{
			Provider: "idx", IndexerName: "Indexer",
			Release: torrent("g1080", title1080, 8*gb, hash1080, e.clock.Add(-time.Hour)),
		})
		rel.IndexerID = "ix"
		if err := e.rt.addPending(e.ctx, []pendingAdd{{itemID: it.ID, rel: rel, reason: core.PendingClientUnavailable}}); err != nil {
			t.Fatal(err)
		}
		if err := e.rt.SyncRSS(e.ctx); err != nil {
			t.Fatal(err)
		}
		if got := e.client.titles(); len(got) != 0 {
			t.Fatalf("grabbed = %v while the client is blocked, want = none", got)
		}
		if pending, _ := e.db.PendingReleases().List(e.ctx, it.ID); len(pending) != 1 || pending[0].Reason != core.PendingClientUnavailable {
			t.Fatalf("pending = %+v, want = the release, waiting for a client", pending)
		}
		e.clock = e.clock.Add(2 * time.Hour)
		if err := e.rt.SyncRSS(e.ctx); err != nil {
			t.Fatal(err)
		}
		if got := e.client.titles(); !slices.Equal(got, []string{title1080}) {
			t.Errorf("grabbed = %v once the client is back, want = [%s]", got, title1080)
		}
	})
}

// TestGrabWithIndexerClient checks an indexer's own download client
// takes its releases, and a release whose indexer's client is missing
// waits pending, as Radarr's DownloadClientProvider.
func TestGrabWithIndexerClient(t *testing.T) {
	tests := []struct {
		client  string
		grabbed bool
	}{
		{"qb", true},
		{"dl/qb", true},
		{"dl/other", false},
	}
	for _, tt := range tests {
		t.Run(tt.client, func(t *testing.T) {
			e := newEnv(t)
			it := e.addMovie()
			e.indexer.settings = func(b *pluginv1.Indexer_builder) { b.DownloadClientId = new(tt.client) }
			e.indexer.set(e.both(), nil)
			if err := e.rt.SearchMovie(e.ctx, it.ID, false); err != nil {
				t.Fatal(err)
			}
			if got := len(e.client.titles()) == 1; got != tt.grabbed {
				t.Errorf("grabbed = %v, want grabbed = %v", e.client.titles(), tt.grabbed)
			}
			pending, _ := e.db.PendingReleases().List(e.ctx, it.ID)
			if got := len(pending) > 0; got == tt.grabbed {
				t.Errorf("pending = %+v, want pending = %v", pending, !tt.grabbed)
			}
		})
	}
}
