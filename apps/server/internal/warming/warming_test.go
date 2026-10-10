package warming

import (
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library/storage"
	"github.com/mavioai/mavio/libs/store"
)

func TestWarmer(t *testing.T) {
	ctx := t.Context()
	s, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	clock := time.Now().UTC().Truncate(time.Second)

	lib := core.Library{Name: "Movies", Kind: core.LibraryMovies, Paths: []string{"/nas/movies"}}
	if err := s.Libraries().Create(ctx, &lib); err != nil {
		t.Fatal(err)
	}
	user := core.User{Name: "alice", PasswordHash: "$argon2id$x"}
	if err := s.Users().Create(ctx, &user); err != nil {
		t.Fatal(err)
	}
	sess := core.AuthSession{UserID: user.ID, TokenHash: make([]byte, 32), DeviceID: "tv", LastSeenAt: clock}
	if err := s.AuthSessions().Create(ctx, &sess); err != nil {
		t.Fatal(err)
	}
	// Two movies in progress and one not started.
	for i, name := range []string{"Up", "Heat", "Alien"} {
		it := core.Item{ID: core.NewID(), LibraryID: lib.ID, Kind: core.KindMovie, Name: name, Path: "/nas/" + name + "/" + name + ".mkv"}
		if err := s.Items().Upsert(ctx, it); err != nil {
			t.Fatal(err)
		}
		ms := core.MediaSource{ID: core.NewID(), ItemID: it.ID, Path: it.Path, Container: "matroska"}
		if err := s.MediaSources().Replace(ctx, it.ID, []core.MediaSource{ms}); err != nil {
			t.Fatal(err)
		}
		if i < 2 {
			last := clock.Add(-time.Duration(i) * time.Hour)
			d := core.UserData{UserID: user.ID, ItemID: it.ID, Position: time.Minute, LastPlayedAt: &last}
			if err := s.UserData().Put(ctx, &d); err != nil {
				t.Fatal(err)
			}
		}
	}

	var mu sync.Mutex
	var reads []string
	keeper := &storage.Keeper{
		Devices: &storage.Detector{Detect: func(p string) (storage.DeviceInfo, error) {
			// Each folder on a volume of its own.
			return storage.DeviceInfo{ID: filepath.Dir(p), Kind: storage.KindRemoteNAS, Remote: true}, nil
		}},
		Now: func() time.Time { return clock },
		Read: func(p string) error {
			mu.Lock()
			reads = append(reads, p)
			mu.Unlock()
			return nil
		},
		Prefetch: func(string) error { return nil },
	}
	online := map[core.ID][]core.ID{}
	w := New(Config{Store: s, Keeper: keeper, Online: func() map[core.ID][]core.ID { return online }, Now: func() time.Time { return clock }})
	beat := func() []string {
		w.Tick(ctx)
		keeper.Wait()
		mu.Lock()
		defer mu.Unlock()
		out := slices.Sorted(slices.Values(reads))
		reads = nil
		return out
	}

	// No client online: nothing is read.
	if got := beat(); len(got) != 0 {
		t.Errorf("reads offline = %v, want none", got)
	}
	// A client in use: the media in progress is kept awake.
	online[user.ID] = []core.ID{sess.ID}
	clock = clock.Add(DefaultRefresh)
	if got, want := beat(), []string{"/nas/Heat/Heat.mkv", "/nas/Up/Up.mkv"}; !slices.Equal(got, want) {
		t.Errorf("reads = %v, want %v", got, want)
	}
	// A client left on without requests lets the disks sleep.
	clock = clock.Add(DefaultPresence + time.Second)
	if got := beat(); len(got) != 0 {
		t.Errorf("reads of an idle client = %v, want none", got)
	}

	// Opening an item wakes its volume.
	clock = clock.Add(time.Minute)
	w.Wake([]core.MediaSource{{Path: "/nas/Alien/Alien.mkv"}, {Path: "/nas/disc", Disc: core.DiscBluRay}})
	deadline := time.Now().Add(5 * time.Second)
	for {
		keeper.Wait()
		mu.Lock()
		got := slices.Clone(reads)
		mu.Unlock()
		if len(got) > 0 || time.Now().After(deadline) {
			if !slices.Equal(got, []string{"/nas/Alien/Alien.mkv"}) {
				t.Errorf("reads after waking = %v, want Alien", got)
			}
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
}
