package smoke_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/plugin/host"
	"github.com/mavioai/mavio/libs/plugin/host/process"
	"github.com/mavioai/mavio/libs/plugin/host/wasm"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/store"
)

// binaries holds the smoke plugin built for each runtime.
var (
	binaries = map[string]string{}
	buildErr error
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "mavio-smoke-")
	if err != nil {
		panic(err)
	}
	buildErr = buildPlugins(dir)
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func buildPlugins(dir string) error {
	pkg := "github.com/mavioai/mavio/apps/server/internal/smoke/smokeplugin"
	binaries["wasm"] = filepath.Join(dir, "plugin.wasm")
	wasmBuild := exec.Command("go", "build", "-buildmode=c-shared", "-o", binaries["wasm"], pkg)
	wasmBuild.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
	if out, err := wasmBuild.CombinedOutput(); err != nil {
		return fmt.Errorf("build wasm plugin: %w\n%s", err, out)
	}
	binaries["process"] = filepath.Join(dir, "plugin")
	if runtime.GOOS == "windows" {
		binaries["process"] += ".exe"
	}
	if out, err := exec.Command("go", "build", "-o", binaries["process"], pkg).CombinedOutput(); err != nil {
		return fmt.Errorf("build native plugin: %w\n%s", err, out)
	}
	return nil
}

// openPlugin lays out a plugin directory for the runtime and starts it.
func openPlugin(t *testing.T, rt string) host.Plugin {
	t.Helper()
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	dir := t.TempDir()
	src := binaries[rt]
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, filepath.Base(src)), data, 0o755); err != nil {
		t.Fatal(err)
	}
	runtimeName := map[string]string{"wasm": "RUNTIME_WASM", "process": "RUNTIME_PROCESS"}[rt]
	manifest := fmt.Sprintf(`{"id":"org.mavio.smoke","name":"Smoke","version":"0.1.0","runtime":%q,
		"capabilities":["CAPABILITY_METADATA_PROVIDER"],"apiVersion":"1.0"}`, runtimeName)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	quiet := slog.New(slog.DiscardHandler)
	p, err := host.Open(t.Context(), dir, host.Options{
		WASM:    wasm.Options{CacheDir: t.TempDir(), CallTimeout: 10 * time.Second, Logger: quiet},
		Process: process.Options{HealthInterval: -1, Logger: quiet},
	})
	if err != nil {
		t.Fatalf("open %s plugin: %v", rt, err)
	}
	t.Cleanup(func() { _ = p.Close(context.Background()) })
	return p
}

// refreshPayload is the payload of a metadata.refresh job.
type refreshPayload struct {
	ItemID core.ID `json:"itemId"`
}

// TestMetadataRefresh runs P1 end to end: a scanned item is queued for a
// metadata refresh, a worker leases the job, asks a plugin for metadata and
// stores it with the credited people, and the result is searchable.
func TestMetadataRefresh(t *testing.T) {
	for _, rt := range []string{"wasm", "process"} {
		t.Run(rt, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			s, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = s.Close() })

			// What a scan would produce.
			lib := core.Library{Name: "Films", Kind: core.LibraryMovies, Paths: []string{"/media/films"}}
			if err := s.Libraries().Create(ctx, &lib); err != nil {
				t.Fatal(err)
			}
			film := core.Item{
				ID: core.NewID(), LibraryID: lib.ID, Kind: core.KindMovie, Name: "Farewell My Concubine",
				ProductionYear: 1993, Path: "/media/films/Farewell My Concubine (1993).mkv", DateAdded: time.Now(),
			}
			if err := s.Items().Upsert(ctx, film); err != nil {
				t.Fatal(err)
			}
			payload, _ := json.Marshal(refreshPayload{ItemID: film.ID})
			job := core.Job{ID: core.NewID(), Kind: "metadata.refresh", Payload: payload, UniqueKey: "refresh:" + film.ID.String()}
			if added, err := s.Jobs().Enqueue(ctx, &job); err != nil || !added {
				t.Fatalf("enqueue = %v, %v", added, err)
			}

			// A worker runs the job.
			p := openPlugin(t, rt)
			leased, err := s.Jobs().Lease(ctx, "worker-1", []string{"metadata.refresh"}, time.Minute)
			if err != nil {
				t.Fatalf("lease: %v", err)
			}
			if err := refresh(ctx, s, p, leased); err != nil {
				_ = s.Jobs().Fail(ctx, leased.ID, "worker-1", err)
				t.Fatalf("refresh: %v", err)
			}
			if err := s.Jobs().Complete(ctx, leased.ID, "worker-1"); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Jobs().Lease(ctx, "worker-1", []string{"metadata.refresh"}, time.Minute); !errors.Is(err, core.ErrNotFound) {
				t.Errorf("second lease: %v, want ErrNotFound", err)
			}

			// The stored metadata is searchable.
			page, err := s.Items().Query(ctx, core.ItemQuery{Search: "霸王"})
			if err != nil || page.Total != 1 || page.Items[0].ExternalIDs[core.ProviderIMDb] != "tt0106332" {
				t.Errorf("search by original title = %+v, %v", page.Items, err)
			}
			genres, err := s.Items().Values(ctx, core.ValueQuery{Kind: core.ValueGenre})
			if err != nil || !slices.Equal(genres, []core.ValueCount{{Value: "Drama", Count: 1}, {Value: "Romance", Count: 1}}) {
				t.Errorf("genres = %v, %v", genres, err)
			}
			people, err := s.People().Search(ctx, core.PersonQuery{Search: "zhang guo rong"})
			if err != nil || len(people) != 1 || people[0].Person.Name != "张国荣" {
				t.Errorf("person search by pinyin = %+v, %v", people, err)
			}
			credits, err := s.People().CreditsForItem(ctx, film.ID)
			if err != nil || len(credits) != 2 || credits[0].Kind != core.CreditActor || credits[0].Role != "Cheng Dieyi" {
				t.Errorf("credits = %+v, %v", credits, err)
			}
		})
	}
}

// refresh fetches the metadata of the job's item from the plugin and stores
// it in one transaction.
func refresh(ctx context.Context, s *store.Store, p host.Plugin, job core.Job) error {
	var payload refreshPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return err
	}
	it, err := s.Items().Get(ctx, payload.ItemID)
	if err != nil {
		return err
	}
	lookup := pluginv1.Lookup_builder{
		Kind: pluginv1.MediaKind_MEDIA_KIND_MOVIE.Enum(),
		Name: proto.String(it.Name),
		Year: proto.Int32(int32(it.ProductionYear)),
	}.Build()
	found, err := p.Metadata().Search(ctx, pluginv1.SearchRequest_builder{Lookup: lookup}.Build())
	if err != nil {
		return err
	}
	if len(found.GetResults()) == 0 {
		return fmt.Errorf("no match for %q", it.Name)
	}
	lookup.SetExternalIds(found.GetResults()[0].GetExternalIds())
	resp, err := p.Metadata().GetMetadata(ctx, pluginv1.GetMetadataRequest_builder{Lookup: lookup}.Build())
	if err != nil {
		return err
	}
	if !resp.GetFound() {
		return fmt.Errorf("no metadata for %q", it.Name)
	}
	md := resp.GetMetadata()
	it.OriginalTitle = md.GetOriginalTitle()
	it.Overview = md.GetOverview()
	it.Genres = md.GetGenres()
	it.ExternalIDs = map[core.Provider]string{}
	for k, v := range md.GetExternalIds() {
		it.ExternalIDs[core.Provider(k)] = v
	}
	it.MetadataRefreshedAt = time.Now()

	return s.InTx(ctx, func(tx core.Store) error {
		if err := tx.Items().Upsert(ctx, it); err != nil {
			return err
		}
		var credits []core.Credit
		for i, pc := range md.GetPeople() {
			person, err := tx.People().FindByName(ctx, pc.GetName())
			if errors.Is(err, core.ErrNotFound) {
				person = core.Person{ID: core.NewID(), Name: pc.GetName()}
				err = tx.People().Upsert(ctx, person)
			}
			if err != nil {
				return err
			}
			credits = append(credits, core.Credit{PersonID: person.ID, Kind: creditKind(pc.GetKind()), Role: pc.GetRole(), Order: i})
		}
		return tx.People().ReplaceCredits(ctx, it.ID, credits)
	})
}

func creditKind(k pluginv1.CreditKind) core.CreditKind {
	switch k {
	case pluginv1.CreditKind_CREDIT_KIND_ACTOR:
		return core.CreditActor
	case pluginv1.CreditKind_CREDIT_KIND_DIRECTOR:
		return core.CreditDirector
	default:
		return core.CreditOther
	}
}
