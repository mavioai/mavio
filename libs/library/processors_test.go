package library

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/metadata"
)

// shouting upper-cases overviews and adds a tag, and names the director
// when asked.
type shouting struct {
	fail, credit bool
	saw          []*metadata.Result
}

func (*shouting) Name() string { return "shouting" }

func (s *shouting) Process(_ context.Context, res *metadata.Result) (*metadata.Result, error) {
	if s.fail {
		return nil, errors.New("hoarse")
	}
	saw := *res
	s.saw = append(s.saw, &saw)
	out := &metadata.Result{}
	out.Item.Overview = strings.ToUpper(res.Item.Overview)
	out.Item.Tags = append(slices.Clone(res.Item.Tags), "loud")
	if s.credit {
		out.People = []metadata.Person{{Name: "Michael Mann", Kind: core.CreditDirector}}
	}
	return out, nil
}

func TestProcessors(t *testing.T) {
	f := newManage(t, false)
	first, second := &shouting{fail: true}, &shouting{credit: true}
	third := &shouting{}
	f.r.Processors = func() []Processor { return []Processor{first, second, third} }
	it := f.refresh(RefreshOptions{})
	// Each processor sees what the ones before did; a failing one changes
	// nothing.
	if it.Overview != "A LAS VEGAS BODYGUARD." || !slices.Equal(it.Tags, []string{"loud", "loud"}) {
		t.Errorf("overview = %q, tags = %q", it.Overview, it.Tags)
	}
	if len(third.saw) != 1 || third.saw[0].Item.Tags[0] != "loud" || len(third.saw[0].People) != 1 {
		t.Errorf("third processor saw %+v", third.saw)
	}
	credits, err := f.store.People().CreditsForItem(t.Context(), it.ID)
	if err != nil || len(credits) != 1 || credits[0].Kind != core.CreditDirector {
		t.Errorf("credits = %+v, %v", credits, err)
	}

	// Locked fields are kept.
	it.LockedFields = []core.MetadataField{core.FieldOverview, core.FieldTags}
	it.Overview, it.Tags = "Quiet.", []string{"calm"}
	if err := f.store.Items().Upsert(t.Context(), it); err != nil {
		t.Fatal(err)
	}
	if it = f.refresh(RefreshOptions{}); it.Overview != "Quiet." || !slices.Equal(it.Tags, []string{"calm"}) {
		t.Errorf("locked: overview = %q, tags = %q", it.Overview, it.Tags)
	}
}
