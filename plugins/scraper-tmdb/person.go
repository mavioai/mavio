package main

import (
	"cmp"
	"context"
	"errors"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// tmdbPerson is a person of TMDB with their combined credits.
type tmdbPerson struct {
	ID                 int      `json:"id"`
	Name               string   `json:"name"`
	Biography          string   `json:"biography"`
	Birthday           string   `json:"birthday"`
	Deathday           string   `json:"deathday"`
	PlaceOfBirth       string   `json:"place_of_birth"`
	AlsoKnownAs        []string `json:"also_known_as"`
	KnownForDepartment string   `json:"known_for_department"`
	ProfilePath        string   `json:"profile_path"`
	IMDbID             string   `json:"imdb_id"`
	CombinedCredits    struct {
		Cast []tmdbPersonCredit `json:"cast"`
		Crew []tmdbPersonCredit `json:"crew"`
	} `json:"combined_credits"`
}

// tmdbPersonCredit is a title a person is credited on: the character
// they play, or their job.
type tmdbPersonCredit struct {
	tmdbListResult
	Character string `json:"character"`
	Job       string `json:"job"`
}

// GetPerson tells of a person by their TMDB ID, as Seerr's person page:
// their details and combined credits in the language asked for, the
// biography in English when it has no translation.
func (p *plugin) GetPerson(ctx context.Context, req *pluginv1.GetPersonRequest) (*pluginv1.GetPersonResponse, error) {
	s, err := p.session()
	if err != nil {
		return nil, err
	}
	id, err := strconv.Atoi(req.GetExternalIds()[keyTMDB])
	if err != nil || id <= 0 {
		return pluginv1.GetPersonResponse_builder{Found: proto.Bool(false)}.Build(), nil
	}
	q := url.Values{"append_to_response": {"combined_credits"}}
	setLanguage(q, req.GetLanguage())
	var tp tmdbPerson
	switch err := s.get(ctx, "/person/"+strconv.Itoa(id), q, &tp); {
	case errors.Is(err, errNotFound):
		return pluginv1.GetPersonResponse_builder{Found: proto.Bool(false)}.Build(), nil
	case err != nil:
		return nil, err
	}
	if tp.Biography == "" && req.GetLanguage() != "" && !strings.HasPrefix(req.GetLanguage(), "en") {
		var en tmdbPerson
		if err := s.get(ctx, "/person/"+strconv.Itoa(id), url.Values{"language": {"en-US"}}, &en); err == nil {
			tp.Biography = en.Biography
		} else if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	adult := req.GetIncludeAdult() && s.adult
	person := pluginv1.Person_builder{
		Name: proto.String(tp.Name), Biography: proto.String(tp.Biography), Birthday: proto.String(tp.Birthday),
		Deathday: proto.String(tp.Deathday), PlaceOfBirth: proto.String(tp.PlaceOfBirth), AlsoKnownAs: tp.AlsoKnownAs,
		KnownForDepartment: proto.String(tp.KnownForDepartment), ExternalIds: map[string]string{keyTMDB: strconv.Itoa(tp.ID)},
		Cast: personTitles(tp.CombinedCredits.Cast, func(c tmdbPersonCredit) string { return c.Character }, adult),
		Crew: personTitles(tp.CombinedCredits.Crew, func(c tmdbPersonCredit) string { return c.Job }, adult),
	}.Build()
	if tp.ProfilePath != "" {
		person.SetImageUrl(imageURL(tp.ProfilePath))
	}
	if tp.IMDbID != "" {
		person.GetExternalIds()[keyIMDb] = tp.IMDbID
	}
	return pluginv1.GetPersonResponse_builder{Found: proto.Bool(true), Person: person}.Build(), nil
}

// personTitles lists the movies and series of a person's credits each
// once, their roles joined, the most voted first, as Seerr's person page.
func personTitles(credits []tmdbPersonCredit, role func(tmdbPersonCredit) string, adult bool) []*pluginv1.PersonTitle {
	type entry struct {
		credit tmdbPersonCredit
		roles  []string
	}
	var order []string
	byTitle := map[string]*entry{}
	for _, c := range credits {
		if c.Adult && !adult || c.MediaType != "movie" && c.MediaType != "tv" {
			continue
		}
		key := c.MediaType + ":" + strconv.Itoa(c.ID)
		e := byTitle[key]
		if e == nil {
			e = &entry{credit: c}
			byTitle[key] = e
			order = append(order, key)
		}
		if r := role(c); r != "" && !slices.Contains(e.roles, r) {
			e.roles = append(e.roles, r)
		}
	}
	slices.SortStableFunc(order, func(a, b string) int { return cmp.Compare(byTitle[b].credit.VoteCount, byTitle[a].credit.VoteCount) })
	out := make([]*pluginv1.PersonTitle, 0, len(order))
	for _, key := range order {
		e := byTitle[key]
		out = append(out, pluginv1.PersonTitle_builder{
			Title: e.credit.title(e.credit.MediaType), Role: proto.String(strings.Join(e.roles, ", ")),
		}.Build())
	}
	return out
}
