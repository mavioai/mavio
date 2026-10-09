package httpserver_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1/userv1connect"
)

func TestAPIKeys(t *testing.T) {
	ctx := t.Context()
	url := newServer(t)
	admin := authv1connect.NewAuthServiceClient(http.DefaultClient, url, withToken(signUp(t, url)))
	created, err := admin.CreateApiKey(ctx, authv1.CreateApiKeyRequest_builder{Name: new("Home Assistant")}.Build())
	if err != nil || created.GetToken() == "" || created.GetApiKey().GetName() != "Home Assistant" {
		t.Fatalf("CreateApiKey = %v, %v", created, err)
	}
	// The key acts as the administrator who made it.
	users := userv1connect.NewUserServiceClient(http.DefaultClient, url, withToken(created.GetToken()))
	if list, err := users.ListUsers(ctx, &userv1.ListUsersRequest{}); err != nil || len(list.GetUsers()) != 1 {
		t.Errorf("ListUsers with the key = %v, %v", list, err)
	}
	keys, err := admin.ListApiKeys(ctx, &authv1.ListApiKeysRequest{})
	if err != nil || len(keys.GetApiKeys()) != 1 || !keys.GetApiKeys()[0].HasLastUseTime() {
		t.Errorf("ListApiKeys = %v, %v", keys, err)
	}
	if _, err := admin.RevokeApiKey(ctx, authv1.RevokeApiKeyRequest_builder{Id: new(created.GetApiKey().GetId())}.Build()); err != nil {
		t.Fatal(err)
	}
	if _, err := users.ListUsers(ctx, &userv1.ListUsersRequest{}); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Errorf("ListUsers with a revoked key: %v, want unauthenticated", err)
	}
}

func TestLocalizationAndDirectories(t *testing.T) {
	ctx := t.Context()
	url := newServer(t)
	token := withToken(signUp(t, url))
	loc := systemv1connect.NewLocalizationServiceClient(http.DefaultClient, url, token)
	countries, err := loc.ListCountries(ctx, &systemv1.ListCountriesRequest{})
	if err != nil || len(countries.GetCountries()) != 140 {
		t.Errorf("ListCountries = %d, %v", len(countries.GetCountries()), err)
	}
	languages, err := loc.ListLanguages(ctx, &systemv1.ListLanguagesRequest{})
	found := false
	for _, l := range languages.GetLanguages() {
		found = found || l.GetCode() == "de" && l.GetThreeLetterCode() == "ger" && l.GetName() == "German"
	}
	if err != nil || !found {
		t.Errorf("ListLanguages lacks German: %v", err)
	}
	ratings, err := loc.ListRatings(ctx, systemv1.ListRatingsRequest_builder{Country: new("DE")}.Build())
	if err != nil || len(ratings.GetRatings()) == 0 || ratings.GetRatings()[0].GetScore() != 0 {
		t.Errorf("ListRatings(DE) = %v, %v", ratings, err)
	}
	dir := t.TempDir()
	system := systemv1connect.NewSystemServiceClient(http.DefaultClient, url, token)
	if err := mkdirs(dir, "Movies", "Shows", ".hidden"); err != nil {
		t.Fatal(err)
	}
	list, err := system.ListDirectory(ctx, systemv1.ListDirectoryRequest_builder{Path: &dir}.Build())
	if err != nil || len(list.GetEntries()) != 2 || list.GetEntries()[0].GetName() != "Movies" || list.GetParent() == "" {
		t.Errorf("ListDirectory = %v, %v", list, err)
	}
	if roots, err := system.ListDirectory(ctx, &systemv1.ListDirectoryRequest{}); err != nil || len(roots.GetEntries()) == 0 {
		t.Errorf("ListDirectory of the roots = %v, %v", roots, err)
	}
	if _, err := system.ListDirectory(ctx, systemv1.ListDirectoryRequest_builder{Path: new("relative")}.Build()); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("ListDirectory of a relative path: %v", err)
	}
}

func mkdirs(root string, names ...string) error {
	for _, n := range names {
		if err := os.MkdirAll(filepath.Join(root, n), 0o755); err != nil {
			return err
		}
	}
	return nil
}
