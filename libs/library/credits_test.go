package library

import (
	"maps"
	"testing"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/metadata"
)

func TestReplaceCreditsKeepsIDsAndHeadshots(t *testing.T) {
	ctx := t.Context()
	db := newMemStore()
	known := core.Person{ID: core.NewID(), Name: "Pierce Brosnan", Overview: "Kept."}
	if err := db.People().Upsert(ctx, known); err != nil {
		t.Fatal(err)
	}
	people := []metadata.Person{
		{
			Name: "Pierce Brosnan", Kind: core.CreditActor, ImageURL: "https://image.example/pb.jpg",
			ExternalIDs: map[core.Provider]string{core.ProviderTMDB: "517"},
		},
		{Name: "Izabella Scorupco", Kind: core.CreditActor, ExternalIDs: map[core.Provider]string{core.ProviderTMDB: "10695"}},
	}
	item := core.NewID()
	if err := replaceCredits(ctx, db, item, people); err != nil {
		t.Fatal(err)
	}
	got, err := db.People().Get(ctx, known.ID)
	if want := map[core.Provider]string{core.ProviderTMDB: "517"}; err != nil || got.Overview != "Kept." || !maps.Equal(got.ExternalIDs, want) {
		t.Errorf("known person = %+v, %v, want = external IDs %v and their overview kept", got, err, want)
	}
	images, _ := db.Images().ListForOwner(ctx, known.ID)
	if len(images) != 1 || images[0].RemoteURL != "https://image.example/pb.jpg" || images[0].Kind != core.ImagePrimary {
		t.Errorf("images = %+v, want = the headshot", images)
	}
	added, err := db.People().FindByName(ctx, "Izabella Scorupco")
	if err != nil || added.ExternalIDs[core.ProviderTMDB] != "10695" {
		t.Errorf("added person = %+v, %v, want = their TMDB ID", added, err)
	}

	// A headshot already kept is not replaced.
	people[0].ImageURL = "https://image.example/other.jpg"
	if err := replaceCredits(ctx, db, item, people); err != nil {
		t.Fatal(err)
	}
	if images, _ := db.Images().ListForOwner(ctx, known.ID); len(images) != 1 || images[0].RemoteURL != "https://image.example/pb.jpg" {
		t.Errorf("images = %+v, want = the first headshot kept", images)
	}
}
