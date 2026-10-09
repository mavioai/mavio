package httpserver_test

import (
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"

	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1/userv1connect"
)

func TestLibraryService(t *testing.T) {
	ctx := t.Context()
	url := newServer(t)
	token := signUp(t, url)
	asAdmin := libraryv1connect.NewLibraryServiceClient(http.DefaultClient, url, withToken(token))
	movies, shows := t.TempDir(), t.TempDir()
	spec := func(name string, kind libraryv1.LibraryKind, paths ...string) *libraryv1.LibrarySpec {
		return libraryv1.LibrarySpec_builder{Name: &name, Kind: &kind, Paths: paths}.Build()
	}
	create := func(s *libraryv1.LibrarySpec) (*libraryv1.Library, error) {
		resp, err := asAdmin.CreateLibrary(ctx, libraryv1.CreateLibraryRequest_builder{Spec: s}.Build())
		return resp.GetLibrary(), err
	}

	films, err := create(spec("Films", libraryv1.LibraryKind_LIBRARY_KIND_MOVIES, movies))
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	if films.GetName() != "Films" || len(films.GetPaths()) != 1 || films.GetKind() != libraryv1.LibraryKind_LIBRARY_KIND_MOVIES {
		t.Errorf("created = %v", films)
	}
	tv, err := create(spec("TV", libraryv1.LibraryKind_LIBRARY_KIND_SHOWS, shows))
	if err != nil {
		t.Fatal(err)
	}
	for name, s := range map[string]*libraryv1.LibrarySpec{
		"relative path": spec("X", libraryv1.LibraryKind_LIBRARY_KIND_MOVIES, "media/films"),
		"missing path":  spec("X", libraryv1.LibraryKind_LIBRARY_KIND_MOVIES, filepath.Join(movies, "missing")),
		"no kind":       libraryv1.LibrarySpec_builder{Name: new("X"), Paths: []string{movies}}.Build(),
	} {
		_, err := create(s)
		wantCode(t, name, err, connect.CodeInvalidArgument)
	}
	_, err = create(spec("Again", libraryv1.LibraryKind_LIBRARY_KIND_MOVIES, movies))
	wantCode(t, "path of another library", err, connect.CodeAlreadyExists)

	// Creating queued a scan, so asking for one adds nothing.
	scan, err := asAdmin.ScanLibrary(ctx, libraryv1.ScanLibraryRequest_builder{Id: new(films.GetId())}.Build())
	if err != nil || scan.GetEnqueued() || scan.HasJobId() {
		t.Errorf("ScanLibrary with a scan pending = %v, %v", scan, err)
	}

	updated, err := asAdmin.UpdateLibrary(ctx, libraryv1.UpdateLibraryRequest_builder{
		Id: new(films.GetId()), Spec: libraryv1.LibrarySpec_builder{
			Name: new("Movies"), Kind: new(libraryv1.LibraryKind_LIBRARY_KIND_MOVIES), Paths: []string{movies},
			ScanInterval: durationpb.New(6 * time.Hour), PreferredLanguage: new("de"), MetadataCountry: new("DE"),
		}.Build(),
	}.Build())
	if err != nil || updated.GetLibrary().GetName() != "Movies" || updated.GetLibrary().GetScanInterval().AsDuration() != 6*time.Hour ||
		updated.GetLibrary().GetMetadataCountry() != "DE" {
		t.Errorf("UpdateLibrary = %v, %v", updated, err)
	}

	// A user limited to the TV library sees it, without paths.
	users := userv1connect.NewUserServiceClient(http.DefaultClient, url, withToken(token))
	if _, err := users.CreateUser(ctx, userv1.CreateUserRequest_builder{
		Name: new("kid"), Password: new("pw"), Policy: userv1.UserPolicy_builder{LibraryIds: []string{tv.GetId()}}.Build(),
	}.Build()); err != nil {
		t.Fatal(err)
	}
	asKid := libraryv1connect.NewLibraryServiceClient(http.DefaultClient, url, withToken(login(t, url, "kid", "pw", "tablet")))
	list, err := asKid.ListLibraries(ctx, &libraryv1.ListLibrariesRequest{})
	if err != nil || len(list.GetLibraries()) != 1 || list.GetLibraries()[0].GetName() != "TV" || len(list.GetLibraries()[0].GetPaths()) != 0 {
		t.Errorf("ListLibraries as kid = %v, %v", list, err)
	}
	_, err = asKid.GetLibrary(ctx, libraryv1.GetLibraryRequest_builder{Id: new(films.GetId())}.Build())
	wantCode(t, "GetLibrary of a forbidden library", err, connect.CodeNotFound)
	_, err = asKid.ScanLibrary(ctx, libraryv1.ScanLibraryRequest_builder{Id: new(tv.GetId())}.Build())
	wantCode(t, "ScanLibrary as kid", err, connect.CodePermissionDenied)

	if _, err := asAdmin.DeleteLibrary(ctx, libraryv1.DeleteLibraryRequest_builder{Id: new(films.GetId())}.Build()); err != nil {
		t.Fatalf("DeleteLibrary: %v", err)
	}
	_, err = asAdmin.GetLibrary(ctx, libraryv1.GetLibraryRequest_builder{Id: new(films.GetId())}.Build())
	wantCode(t, "GetLibrary after delete", err, connect.CodeNotFound)
}
