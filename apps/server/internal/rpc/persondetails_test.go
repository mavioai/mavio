package rpc

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/libs/core"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

func TestGetPersonFetchesDetails(t *testing.T) {
	ctx := auth.WithPrincipal(t.Context(), auth.Principal{User: core.User{ID: core.NewID(), Kind: core.UserMember}})
	db := openStore(t)
	person := core.Person{ID: core.NewID(), Name: "Pierce Brosnan", ExternalIDs: map[core.Provider]string{core.ProviderTMDB: "517"}}
	if err := db.People().Upsert(ctx, person); err != nil {
		t.Fatal(err)
	}
	svc := NewItemService(db)
	var asked []string
	fail := true
	svc.PersonDetails = func(_ context.Context, ids map[core.Provider]string, language string) (*pluginv1.Person, error) {
		asked = append(asked, ids[core.ProviderTMDB]+"/"+language)
		if fail {
			return nil, errors.New("offline")
		}
		return pluginv1.Person_builder{
			Biography: new("An Irish actor."), Birthday: new("1953-05-16"), PlaceOfBirth: new("Drogheda, Ireland"),
			ImageUrl: new("https://image.example/pb.jpg"), ExternalIds: map[string]string{"imdb": "nm0000112"},
		}.Build(), nil
	}
	get := func() *libraryv1.Person {
		t.Helper()
		res, err := svc.GetPerson(ctx, libraryv1.GetPersonRequest_builder{Id: new(person.ID.String())}.Build())
		if err != nil {
			t.Fatal(err)
		}
		return res.GetPerson()
	}

	// A provider failing leaves the person as stored, asked for again.
	if got := get(); got.GetOverview() != "" {
		t.Errorf("overview = %q, want = none", got.GetOverview())
	}
	fail = false
	got := get()
	if got.GetOverview() != "An Irish actor." || got.GetBirthPlace() != "Drogheda, Ireland" || len(got.GetImages()) != 1 {
		t.Errorf("person = %v, want = their biography, birth place and headshot", got)
	}
	stored, err := db.People().Get(ctx, person.ID)
	if err != nil || stored.DetailsFetchedAt == nil || stored.BirthDate == nil || stored.BirthDate.Year() != 1953 ||
		stored.ExternalIDs[core.ProviderIMDb] != "nm0000112" || stored.ExternalIDs[core.ProviderTMDB] != "517" {
		t.Errorf("stored = %+v, %v, want = details kept with when they were fetched", stored, err)
	}

	// Fresh details are read from the store.
	get()
	if want := []string{"517/en", "517/en"}; !slices.Equal(asked, want) {
		t.Errorf("asked = %v, want = %v", asked, want)
	}
}
