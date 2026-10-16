package main

import (
	"slices"
	"testing"

	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

func TestGetPerson(t *testing.T) {
	p, fake := newDiscoveryPlugin(t)
	fake.routes["/3/person/6384"] = `{"id":6384,"name":"Keanu Reeves","biography":"","birthday":"1964-09-02","place_of_birth":"Beirut, Lebanon",
		"also_known_as":["Киану Ривз"],"known_for_department":"Acting","profile_path":"/k.jpg","imdb_id":"nm0000206",
		"combined_credits":{
			"cast":[
				{"id":603,"media_type":"movie","title":"The Matrix","release_date":"1999-03-30","vote_count":25000,"character":"Neo"},
				{"id":1,"media_type":"tv","name":"Talk Show","first_air_date":"1990-01-01","vote_count":10,"character":"Himself"},
				{"id":1,"media_type":"tv","name":"Talk Show","vote_count":10,"character":"Guest"},
				{"id":9,"media_type":"movie","title":"Adult","adult":true,"vote_count":99999,"character":"X"},
				{"id":245891,"media_type":"movie","title":"John Wick","vote_count":20000,"character":"John Wick"}],
			"crew":[
				{"id":245891,"media_type":"movie","title":"John Wick","vote_count":20000,"job":"Executive Producer"},
				{"id":245891,"media_type":"movie","title":"John Wick","vote_count":20000,"job":"Stunts"}]}}`
	req := pluginv1.GetPersonRequest_builder{ExternalIds: map[string]string{keyTMDB: "6384"}, Language: proto.String("de")}.Build()
	resp, err := p.GetPerson(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.GetFound() {
		t.Fatalf("GetPerson() found = false, want = true")
	}
	got := resp.GetPerson()
	if got.GetName() != "Keanu Reeves" || got.GetBirthday() != "1964-09-02" || got.GetPlaceOfBirth() != "Beirut, Lebanon" ||
		got.GetKnownForDepartment() != "Acting" || got.GetImageUrl() != "https://image.tmdb.org/t/p/original/k.jpg" ||
		got.GetExternalIds()[keyIMDb] != "nm0000206" || !slices.Equal(got.GetAlsoKnownAs(), []string{"Киану Ривз"}) {
		t.Errorf("GetPerson() = %v", got)
	}
	titles := func(ts []*pluginv1.PersonTitle) []string {
		var out []string
		for _, pt := range ts {
			out = append(out, pt.GetTitle().GetExternalIds()[keyTMDB]+" "+pt.GetTitle().GetName()+": "+pt.GetRole())
		}
		return out
	}
	// Each title once, roles joined, the most voted first; adult titles
	// left out.
	if got, want := titles(got.GetCast()), []string{"603 The Matrix: Neo", "245891 John Wick: John Wick", "1 Talk Show: Himself, Guest"}; !slices.Equal(got, want) {
		t.Errorf("cast = %q, want = %q", got, want)
	}
	if got, want := titles(got.GetCrew()), []string{"245891 John Wick: Executive Producer, Stunts"}; !slices.Equal(got, want) {
		t.Errorf("crew = %q, want = %q", got, want)
	}
	// Without a translated biography, the English one is asked for.
	var langs []string
	for _, r := range fake.requests {
		if r.URL.Path == "/3/person/6384" {
			langs = append(langs, r.URL.Query().Get("language")+"/"+r.URL.Query().Get("append_to_response"))
		}
	}
	if want := []string{"de/combined_credits", "en-US/"}; !slices.Equal(langs, want) {
		t.Errorf("person requests = %q, want = %q", langs, want)
	}

	resp, err = p.GetPerson(t.Context(), pluginv1.GetPersonRequest_builder{ExternalIds: map[string]string{keyTMDB: "404"}}.Build())
	if err != nil || resp.GetFound() {
		t.Errorf("GetPerson(unknown) = %v, %v, want not found", resp, err)
	}
}
