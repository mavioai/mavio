package requests

import (
	"context"
	"fmt"
	"strconv"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// Person is a person as a discovery provider tells of them, as Seerr's
// person page, with the titles they are credited on marked as Discover's.
type Person struct {
	Person *pluginv1.Person
	// Cast and Crew are the titles they play in and work on, the most
	// voted first, without adult titles and those above the user's
	// rating limit.
	Cast, Crew []PersonTitle
	// Hidden counts the titles left out.
	Hidden int
}

// PersonTitle is a title a person is credited on.
type PersonTitle struct {
	DiscoveredTitle
	// Role is the characters they play, or their jobs.
	Role string
}

// Person tells of a person by their TMDB ID through the first discovery
// provider knowing people, in a language (ISO 639-1), for a user. People
// no provider knows fail with core.ErrNotFound.
func (s *Service) Person(ctx context.Context, caller core.User, tmdbID int, language string) (Person, error) {
	req := pluginv1.GetPersonRequest_builder{
		ExternalIds: map[string]string{string(core.ProviderTMDB): strconv.Itoa(tmdbID)}, Language: &language,
	}.Build()
	for _, p := range s.discoveryProviders() {
		res, err := p.Client.GetPerson(ctx, req)
		switch {
		case connect.CodeOf(err) == connect.CodeUnimplemented:
			continue
		case err != nil:
			return Person{}, fmt.Errorf("person %s: %w", p.ID, err)
		case !res.GetFound():
			continue
		}
		person := res.GetPerson()
		out := Person{Person: person}
		rate := caller.EffectiveAccess().MaxParentalRating != nil
		if out.Cast, err = s.personTitles(ctx, &caller, p.ID, person.GetCast(), rate, &out.Hidden); err != nil {
			return Person{}, err
		}
		if out.Crew, err = s.personTitles(ctx, &caller, p.ID, person.GetCrew(), rate, &out.Hidden); err != nil {
			return Person{}, err
		}
		return out, nil
	}
	return Person{}, fmt.Errorf("person with TMDB ID %d: %w", tmdbID, core.ErrNotFound)
}

// personTitles marks a person's titles as a page of Discover, counting
// those left out in hidden.
func (s *Service) personTitles(ctx context.Context, u *core.User, pluginID string, credits []*pluginv1.PersonTitle, rate bool, hidden *int) ([]PersonTitle, error) {
	var titles []*pluginv1.DiscoveredTitle
	roles := map[*pluginv1.DiscoveredTitle]string{}
	for _, c := range credits {
		if t := c.GetTitle(); t != nil {
			titles = append(titles, t)
			roles[t] = c.GetRole()
		}
	}
	page, err := s.page(ctx, u, pluginID, titles, 1, 1, len(titles), rate)
	if err != nil {
		return nil, err
	}
	*hidden += page.Hidden
	out := make([]PersonTitle, len(page.Titles))
	for i, d := range page.Titles {
		out[i] = PersonTitle{DiscoveredTitle: d, Role: roles[d.Title]}
	}
	return out, nil
}
