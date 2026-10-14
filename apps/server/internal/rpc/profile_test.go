package rpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/apps/server/internal/settings"
	"github.com/mavioai/mavio/libs/core"
	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
	"github.com/mavioai/mavio/libs/store"
)

// roleHeader names the role a test request is made as: "admin" or "user".
const roleHeader = "Test-Role"

// testHandlers serves a handler as the server does, validating requests,
// with the caller's role taken from roleHeader.
func testHandlers(t *testing.T, mount func(opts connect.HandlerOption) (string, http.Handler)) string {
	t.Helper()
	validate, err := NewValidateInterceptor()
	if err != nil {
		t.Fatal(err)
	}
	authn := connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			switch role := req.Header().Get(roleHeader); role {
			case "admin", "user":
				ctx = auth.WithPrincipal(ctx, auth.Principal{User: core.User{ID: core.NewID(), Name: role, Kind: map[bool]core.UserKind{true: core.UserAdmin, false: core.UserMember}[role == "admin"]}})
			default:
				return nil, connect.NewError(connect.CodeUnauthenticated, nil)
			}
			return next(ctx, req)
		}
	})
	mux := http.NewServeMux()
	mux.Handle(mount(connect.WithInterceptors(authn, validate)))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

// as makes requests as a role.
func as(role string) connect.ClientOption {
	return connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			req.Header().Set(roleHeader, role)
			return next(ctx, req)
		}
	}))
}

func openStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.Open(t.Context(), "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func wantCode(t *testing.T, what string, err error, want connect.Code) {
	t.Helper()
	if got := connect.CodeOf(err); err == nil || got != want {
		t.Errorf("%s: err = %v, want %v", what, err, want)
	}
}

// movieProfile returns a valid movie profile of the definitions: each
// quality allowed on its own but the last two, grouped; the cutoff the
// group.
func movieProfile(name string, defs []*acquisitionv1.QualityDefinition) *acquisitionv1.QualityProfile {
	var items []*acquisitionv1.QualityProfileItem
	for _, d := range defs[:len(defs)-2] {
		items = append(items, acquisitionv1.QualityProfileItem_builder{Quality: new(d.GetQuality()), Allowed: new(true)}.Build())
	}
	var grouped []*acquisitionv1.QualityProfileItem
	for _, d := range defs[len(defs)-2:] {
		grouped = append(grouped, acquisitionv1.QualityProfileItem_builder{Quality: new(d.GetQuality()), Allowed: new(true)}.Build())
	}
	items = append(items, acquisitionv1.QualityProfileItem_builder{Id: new(int32(1000)), Name: new("Best"), Items: grouped, Allowed: new(true)}.Build())
	return acquisitionv1.QualityProfile_builder{
		Kind: new(acquisitionv1.MediaKind_MEDIA_KIND_MOVIE), Name: new(name), UpgradeAllowed: new(true), Cutoff: new(int32(1000)), Items: items,
	}.Build()
}

// TestProfileService runs ProfileService against a store: definitions,
// quality profiles, custom formats, delay and release profiles, remote
// path mappings.
func TestProfileService(t *testing.T) {
	ctx := t.Context()
	db := openStore(t)
	url := testHandlers(t, func(opts connect.HandlerOption) (string, http.Handler) {
		return acquisitionv1connect.NewProfileServiceHandler(NewProfileService(db), opts)
	})
	admin := acquisitionv1connect.NewProfileServiceClient(http.DefaultClient, url, as("admin"))
	user := acquisitionv1connect.NewProfileServiceClient(http.DefaultClient, url, as("user"))
	movieKind, seriesKind := new(acquisitionv1.MediaKind_MEDIA_KIND_MOVIE), new(acquisitionv1.MediaKind_MEDIA_KIND_SERIES)

	// Quality definitions: Radarr's defaults until changed.
	listDefs := func(kind *acquisitionv1.MediaKind) []*acquisitionv1.QualityDefinition {
		t.Helper()
		res, err := admin.ListQualityDefinitions(ctx, acquisitionv1.ListQualityDefinitionsRequest_builder{Kind: kind}.Build())
		if err != nil {
			t.Fatalf("ListQualityDefinitions: %v", err)
		}
		return res.GetDefinitions()
	}
	defs, seriesDefs := listDefs(movieKind), listDefs(seriesKind)
	if len(defs) < 4 || len(seriesDefs) < 4 || len(defs) == len(seriesDefs) {
		t.Fatalf("definitions: %d of movies, %d of series", len(defs), len(seriesDefs))
	}
	changed := defs[3]
	if _, err := admin.UpdateQualityDefinitions(ctx, acquisitionv1.UpdateQualityDefinitionsRequest_builder{
		Kind: movieKind,
		Definitions: []*acquisitionv1.QualityDefinition{acquisitionv1.QualityDefinition_builder{
			Quality: new(changed.GetQuality()), MinSize: new(5.0), MaxSize: new(50.0), PreferredSize: new(20.0),
		}.Build()},
	}.Build()); err != nil {
		t.Fatalf("UpdateQualityDefinitions: %v", err)
	}
	after := listDefs(movieKind)
	if len(after) != len(defs) {
		t.Fatalf("definitions after update: %d, want %d", len(after), len(defs))
	}
	for i, d := range after {
		switch {
		case d.GetQuality() != defs[i].GetQuality() || d.GetTitle() != defs[i].GetTitle():
			t.Errorf("definition %d = %v, want quality %d %q", i, d, defs[i].GetQuality(), defs[i].GetTitle())
		case i == 3 && (d.GetMinSize() != 5 || d.GetMaxSize() != 50 || d.GetPreferredSize() != 20):
			t.Errorf("changed definition = %v, want sizes 5, 50, 20", d)
		case i != 3 && (d.GetMinSize() != defs[i].GetMinSize() || d.GetMaxSize() != defs[i].GetMaxSize()):
			t.Errorf("definition %d = %v, want %v kept", i, d, defs[i])
		}
	}
	if got := listDefs(seriesKind); got[0].GetMinSize() != seriesDefs[0].GetMinSize() {
		t.Errorf("series definitions changed with movies'")
	}
	for name, d := range map[string]*acquisitionv1.QualityDefinition{
		"unknown quality":   acquisitionv1.QualityDefinition_builder{Quality: new(int32(9999)), MinSize: new(1.0)}.Build(),
		"minimum above max": acquisitionv1.QualityDefinition_builder{Quality: new(changed.GetQuality()), MinSize: new(60.0), MaxSize: new(50.0)}.Build(),
	} {
		_, err := admin.UpdateQualityDefinitions(ctx, acquisitionv1.UpdateQualityDefinitionsRequest_builder{
			Kind: movieKind, Definitions: []*acquisitionv1.QualityDefinition{d},
		}.Build())
		wantCode(t, name, err, connect.CodeInvalidArgument)
	}
	_, err := user.ListQualityDefinitions(ctx, acquisitionv1.ListQualityDefinitionsRequest_builder{Kind: movieKind}.Build())
	wantCode(t, "definitions as a user", err, connect.CodePermissionDenied)

	// Custom formats are checked by libs/acquire.
	format := acquisitionv1.CustomFormat_builder{Name: new("x265"), Specifications: []*acquisitionv1.CustomFormatSpec{
		acquisitionv1.CustomFormatSpec_builder{Kind: new(acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_RELEASE_TITLE), Name: new("x265"), Value: new(`[xh]\.?265`)}.Build(),
		acquisitionv1.CustomFormatSpec_builder{Kind: new(acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_SOURCE), Name: new("Bluray"), Value: new("bluray")}.Build(),
	}}.Build()
	created, err := admin.CreateCustomFormat(ctx, acquisitionv1.CreateCustomFormatRequest_builder{Format: format}.Build())
	if err != nil {
		t.Fatalf("CreateCustomFormat: %v", err)
	}
	x265 := created.GetFormat()
	if x265.GetId() == "" || len(x265.GetSpecifications()) != 2 || x265.GetSpecifications()[1].GetValue() != "bluray" {
		t.Errorf("created format = %v", x265)
	}
	for name, spec := range map[string]*acquisitionv1.CustomFormatSpec{
		"bad expression": acquisitionv1.CustomFormatSpec_builder{Kind: new(acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_RELEASE_TITLE), Name: new("bad"), Value: new("(")}.Build(),
		"unknown source": acquisitionv1.CustomFormatSpec_builder{Kind: new(acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_SOURCE), Name: new("bad"), Value: new("laserdisc")}.Build(),
		"no kind":        acquisitionv1.CustomFormatSpec_builder{Name: new("bad"), Value: new("x")}.Build(),
	} {
		_, err := admin.CreateCustomFormat(ctx, acquisitionv1.CreateCustomFormatRequest_builder{Format: acquisitionv1.CustomFormat_builder{
			Name: new(name), Specifications: []*acquisitionv1.CustomFormatSpec{spec},
		}.Build()}.Build())
		wantCode(t, name, err, connect.CodeInvalidArgument)
	}
	_, err = admin.CreateCustomFormat(ctx, acquisitionv1.CreateCustomFormatRequest_builder{Format: format}.Build())
	wantCode(t, "format of the same name", err, connect.CodeAlreadyExists)
	x265.SetIncludeWhenRenaming(true)
	if res, err := admin.UpdateCustomFormat(ctx, acquisitionv1.UpdateCustomFormatRequest_builder{Format: x265}.Build()); err != nil || !res.GetFormat().GetIncludeWhenRenaming() {
		t.Errorf("UpdateCustomFormat = %v, %v", res, err)
	}
	missing := acquisitionv1.CustomFormat_builder{Id: new(core.NewID().String()), Name: new("Missing")}.Build()
	_, err = admin.UpdateCustomFormat(ctx, acquisitionv1.UpdateCustomFormatRequest_builder{Format: missing}.Build())
	wantCode(t, "update of a missing format", err, connect.CodeNotFound)

	// Quality profiles are checked against the kind's qualities.
	hd := movieProfile("HD", defs)
	hd.SetFormatScores([]*acquisitionv1.FormatScore{acquisitionv1.FormatScore_builder{FormatId: new(x265.GetId()), Score: new(int32(10))}.Build()})
	res, err := admin.CreateQualityProfile(ctx, acquisitionv1.CreateQualityProfileRequest_builder{Profile: hd}.Build())
	if err != nil {
		t.Fatalf("CreateQualityProfile: %v", err)
	}
	hd = res.GetProfile()
	if hd.GetId() == "" || len(hd.GetItems()) != len(defs)-1 || len(hd.GetFormatScores()) != 1 {
		t.Errorf("created profile = %v", hd)
	}
	invalidProfiles := map[string]func(p *acquisitionv1.QualityProfile){
		"missing qualities": func(p *acquisitionv1.QualityProfile) { p.SetItems(p.GetItems()[1:]) },
		"cutoff not in it":  func(p *acquisitionv1.QualityProfile) { p.SetCutoff(1001) },
		"unknown format": func(p *acquisitionv1.QualityProfile) {
			p.SetFormatScores([]*acquisitionv1.FormatScore{acquisitionv1.FormatScore_builder{FormatId: new(core.NewID().String())}.Build()})
		},
		"format scored twice": func(p *acquisitionv1.QualityProfile) {
			score := acquisitionv1.FormatScore_builder{FormatId: new(x265.GetId()), Score: new(int32(10))}.Build()
			p.SetFormatScores([]*acquisitionv1.FormatScore{score, score})
		},
		"unsatisfiable minimum score": func(p *acquisitionv1.QualityProfile) {
			p.SetFormatScores([]*acquisitionv1.FormatScore{acquisitionv1.FormatScore_builder{FormatId: new(x265.GetId()), Score: new(int32(10))}.Build()})
			p.SetMinFormatScore(11)
		},
		"unknown language": func(p *acquisitionv1.QualityProfile) { p.SetLanguage("Klingon") },
		"no kind":          func(p *acquisitionv1.QualityProfile) { p.ClearKind() },
	}
	for name, change := range invalidProfiles {
		p := movieProfile(name, defs)
		change(p)
		_, err := admin.CreateQualityProfile(ctx, acquisitionv1.CreateQualityProfileRequest_builder{Profile: p}.Build())
		wantCode(t, name, err, connect.CodeInvalidArgument)
	}
	_, err = admin.CreateQualityProfile(ctx, acquisitionv1.CreateQualityProfileRequest_builder{Profile: movieProfile("HD", defs)}.Build())
	wantCode(t, "profile of the same name", err, connect.CodeAlreadyExists)
	_, err = user.CreateQualityProfile(ctx, acquisitionv1.CreateQualityProfileRequest_builder{Profile: movieProfile("Mine", defs)}.Build())
	wantCode(t, "profile created by a user", err, connect.CodePermissionDenied)

	hd.SetName("HD-1080p")
	hd.SetLanguage("Original")
	if res, err := admin.UpdateQualityProfile(ctx, acquisitionv1.UpdateQualityProfileRequest_builder{Profile: hd}.Build()); err != nil ||
		res.GetProfile().GetName() != "HD-1080p" || res.GetProfile().GetLanguage() != "Original" {
		t.Errorf("UpdateQualityProfile = %v, %v", res, err)
	}
	asSeries := movieProfile("HD-1080p", defs)
	asSeries.SetId(hd.GetId())
	asSeries.SetKind(acquisitionv1.MediaKind_MEDIA_KIND_SERIES)
	_, err = admin.UpdateQualityProfile(ctx, acquisitionv1.UpdateQualityProfileRequest_builder{Profile: asSeries}.Build())
	wantCode(t, "kind change", err, connect.CodeInvalidArgument)

	// Users list them, by kind or all.
	listProfiles := func(c acquisitionv1connect.ProfileServiceClient, kind acquisitionv1.MediaKind) []string {
		t.Helper()
		res, err := c.ListQualityProfiles(ctx, acquisitionv1.ListQualityProfilesRequest_builder{Kind: new(kind)}.Build())
		if err != nil {
			t.Fatalf("ListQualityProfiles: %v", err)
		}
		var names []string
		for _, p := range res.GetProfiles() {
			names = append(names, p.GetName())
		}
		return names
	}
	if got := listProfiles(user, acquisitionv1.MediaKind_MEDIA_KIND_UNSPECIFIED); !slices.Equal(got, []string{"HD-1080p"}) {
		t.Errorf("profiles = %q", got)
	}
	if got := listProfiles(user, acquisitionv1.MediaKind_MEDIA_KIND_SERIES); len(got) != 0 {
		t.Errorf("series profiles = %q", got)
	}

	// Deleting a custom format removes its scores.
	if _, err := admin.DeleteCustomFormat(ctx, acquisitionv1.DeleteCustomFormatRequest_builder{Id: new(x265.GetId())}.Build()); err != nil {
		t.Fatalf("DeleteCustomFormat: %v", err)
	}
	if res, err := admin.ListQualityProfiles(ctx, &acquisitionv1.ListQualityProfilesRequest{}); err != nil || len(res.GetProfiles()[0].GetFormatScores()) != 0 {
		t.Errorf("profiles after the format's deletion = %v, %v", res, err)
	}

	// A profile wanted items use cannot be deleted.
	lib := core.Library{Name: "Movies", Kind: core.LibraryMixed, Paths: []string{"/movies"}}
	if err := db.Libraries().Create(ctx, &lib); err != nil {
		t.Fatal(err)
	}
	movie := core.Item{ID: core.NewID(), LibraryID: lib.ID, Kind: core.KindMovie, Name: "Heat", DateAdded: time.Now()}
	if err := db.Items().Upsert(ctx, movie); err != nil {
		t.Fatal(err)
	}
	hdID, _ := core.ParseID(hd.GetId())
	if err := db.Wanted().Put(ctx, &core.Wanted{ItemID: movie.ID, QualityProfileID: hdID}); err != nil {
		t.Fatal(err)
	}
	deleteProfile := func() error {
		_, err := admin.DeleteQualityProfile(ctx, acquisitionv1.DeleteQualityProfileRequest_builder{Id: new(hd.GetId())}.Build())
		return err
	}
	wantCode(t, "deletion of a profile in use", deleteProfile(), connect.CodeFailedPrecondition)
	if err := db.Wanted().Delete(ctx, movie.ID); err != nil {
		t.Fatal(err)
	}
	if err := deleteProfile(); err != nil {
		t.Errorf("DeleteQualityProfile: %v", err)
	}
	wantCode(t, "second deletion", deleteProfile(), connect.CodeNotFound)
}

// TestDelayProfiles checks the default delay profile and the order of the
// others.
func TestDelayProfiles(t *testing.T) {
	ctx := t.Context()
	db := openStore(t)
	url := testHandlers(t, func(opts connect.HandlerOption) (string, http.Handler) {
		return acquisitionv1connect.NewProfileServiceHandler(NewProfileService(db), opts)
	})
	admin := acquisitionv1connect.NewProfileServiceClient(http.DefaultClient, url, as("admin"))
	list := func() []*acquisitionv1.DelayProfile {
		t.Helper()
		res, err := admin.ListDelayProfiles(ctx, &acquisitionv1.ListDelayProfilesRequest{})
		if err != nil {
			t.Fatalf("ListDelayProfiles: %v", err)
		}
		return res.GetProfiles()
	}
	tagsOf := func(profiles []*acquisitionv1.DelayProfile) []string {
		var out []string
		for _, p := range profiles {
			tag := "-"
			if len(p.GetTags()) > 0 {
				tag = p.GetTags()[0]
			}
			out = append(out, tag)
		}
		return out
	}

	// Radarr's default profile is there from the start, once.
	profiles := list()
	if len(profiles) != 1 || len(list()) != 1 {
		t.Fatalf("delay profiles = %v, want the default", profiles)
	}
	def := profiles[0]
	if !def.GetDefault() || len(def.GetTags()) != 0 || !def.GetEnableUsenet() || !def.GetEnableTorrent() ||
		def.GetPreferredProtocol() != acquisitionv1.DownloadProtocol_DOWNLOAD_PROTOCOL_USENET || def.GetUsenetDelay().AsDuration() != 0 {
		t.Errorf("default delay profile = %v", def)
	}

	create := func(tag string) (*acquisitionv1.DelayProfile, error) {
		p := acquisitionv1.DelayProfile_builder{
			EnableTorrent: new(true), PreferredProtocol: new(acquisitionv1.DownloadProtocol_DOWNLOAD_PROTOCOL_TORRENT),
			TorrentDelay: durationpb.New(time.Hour),
		}.Build()
		if tag != "" {
			p.SetTags([]string{tag})
		}
		res, err := admin.CreateDelayProfile(ctx, acquisitionv1.CreateDelayProfileRequest_builder{Profile: p}.Build())
		return res.GetProfile(), err
	}
	_, err := create("")
	wantCode(t, "profile without tags", err, connect.CodeInvalidArgument)
	anime, err := create("anime")
	if err != nil {
		t.Fatalf("CreateDelayProfile: %v", err)
	}
	kids, err := create("kids")
	if err != nil {
		t.Fatalf("CreateDelayProfile: %v", err)
	}
	if anime.GetOrder() != 1 || kids.GetOrder() != 2 || anime.GetTorrentDelay().AsDuration() != time.Hour {
		t.Errorf("created profiles = %v, %v", anime, kids)
	}
	if got := tagsOf(list()); !slices.Equal(got, []string{"anime", "kids", "-"}) {
		t.Errorf("profiles = %q, want anime, kids, the default", got)
	}

	// Reordering takes every profile but the default.
	reorder := func(ids ...string) ([]*acquisitionv1.DelayProfile, error) {
		res, err := admin.ReorderDelayProfiles(ctx, acquisitionv1.ReorderDelayProfilesRequest_builder{Ids: ids}.Build())
		return res.GetProfiles(), err
	}
	got, err := reorder(kids.GetId(), anime.GetId())
	if err != nil {
		t.Fatalf("ReorderDelayProfiles: %v", err)
	}
	if !slices.Equal(tagsOf(got), []string{"kids", "anime", "-"}) || got[0].GetOrder() != 1 || got[1].GetOrder() != 2 {
		t.Errorf("reordered = %v", got)
	}
	if !slices.Equal(tagsOf(list()), []string{"kids", "anime", "-"}) {
		t.Errorf("profiles after reorder = %q", tagsOf(list()))
	}
	for name, ids := range map[string][]string{
		"one missing":  {kids.GetId()},
		"the default":  {kids.GetId(), anime.GetId(), def.GetId()},
		"one repeated": {kids.GetId(), kids.GetId()},
	} {
		_, err := reorder(ids...)
		wantCode(t, "reorder with "+name, err, connect.CodeInvalidArgument)
	}

	// The default keeps no tags and cannot be deleted.
	def.SetTags([]string{"anime"})
	_, err = admin.UpdateDelayProfile(ctx, acquisitionv1.UpdateDelayProfileRequest_builder{Profile: def}.Build())
	wantCode(t, "default with tags", err, connect.CodeInvalidArgument)
	def.SetTags(nil)
	def.SetUsenetDelay(durationpb.New(time.Minute))
	def.SetOrder(1)
	if res, err := admin.UpdateDelayProfile(ctx, acquisitionv1.UpdateDelayProfileRequest_builder{Profile: def}.Build()); err != nil ||
		!res.GetProfile().GetDefault() || res.GetProfile().GetUsenetDelay().AsDuration() != time.Minute || res.GetProfile().GetOrder() == 1 {
		t.Errorf("UpdateDelayProfile of the default = %v, %v", res, err)
	}
	_, err = admin.DeleteDelayProfile(ctx, acquisitionv1.DeleteDelayProfileRequest_builder{Id: new(def.GetId())}.Build())
	wantCode(t, "deletion of the default", err, connect.CodeFailedPrecondition)

	// Deleting renumbers the others.
	if _, err := admin.DeleteDelayProfile(ctx, acquisitionv1.DeleteDelayProfileRequest_builder{Id: new(kids.GetId())}.Build()); err != nil {
		t.Fatalf("DeleteDelayProfile: %v", err)
	}
	if got := list(); len(got) != 2 || got[0].GetId() != anime.GetId() || got[0].GetOrder() != 1 || !got[1].GetDefault() {
		t.Errorf("profiles after deletion = %v", got)
	}
}

// TestReleaseProfilesAndMappings checks release profiles and remote path
// mappings.
func TestReleaseProfilesAndMappings(t *testing.T) {
	ctx := t.Context()
	db := openStore(t)
	url := testHandlers(t, func(opts connect.HandlerOption) (string, http.Handler) {
		return acquisitionv1connect.NewProfileServiceHandler(NewProfileService(db), opts)
	})
	admin := acquisitionv1connect.NewProfileServiceClient(http.DefaultClient, url, as("admin"))

	indexer := acquisitionv1.IndexerRef_builder{PluginId: new("org.mavio.indexers"), IndexerId: new("nzbgeek")}.Build()
	res, err := admin.CreateReleaseProfile(ctx, acquisitionv1.CreateReleaseProfileRequest_builder{Profile: acquisitionv1.ReleaseProfile_builder{
		Name: new("No HDCAM"), Enabled: new(true), Ignored: []string{"HDCAM", `/\bTS\b/i`}, Indexer: indexer, Tags: []string{"new"},
	}.Build()}.Build())
	if err != nil {
		t.Fatalf("CreateReleaseProfile: %v", err)
	}
	p := res.GetProfile()
	if p.GetIndexer().GetPluginId() != "org.mavio.indexers" || p.GetIndexer().GetIndexerId() != "nzbgeek" || len(p.GetIgnored()) != 2 {
		t.Errorf("created release profile = %v", p)
	}
	stored, err := db.ReleaseProfiles().List(ctx)
	if err != nil || len(stored) != 1 || stored[0].IndexerID != "org.mavio.indexers/nzbgeek" {
		t.Errorf("stored release profiles = %+v, %v", stored, err)
	}
	p.ClearIndexer()
	p.SetRequired([]string{"PROPER"})
	if res, err := admin.UpdateReleaseProfile(ctx, acquisitionv1.UpdateReleaseProfileRequest_builder{Profile: p}.Build()); err != nil ||
		res.GetProfile().HasIndexer() || len(res.GetProfile().GetRequired()) != 1 {
		t.Errorf("UpdateReleaseProfile = %v, %v", res, err)
	}
	for name, rp := range map[string]*acquisitionv1.ReleaseProfile{
		"no terms":       acquisitionv1.ReleaseProfile_builder{Name: new("Empty")}.Build(),
		"bad expression": acquisitionv1.ReleaseProfile_builder{Name: new("Bad"), Required: []string{"/(/"}}.Build(),
	} {
		_, err := admin.CreateReleaseProfile(ctx, acquisitionv1.CreateReleaseProfileRequest_builder{Profile: rp}.Build())
		wantCode(t, name, err, connect.CodeInvalidArgument)
	}
	if _, err := admin.DeleteReleaseProfile(ctx, acquisitionv1.DeleteReleaseProfileRequest_builder{Id: new(p.GetId())}.Build()); err != nil {
		t.Errorf("DeleteReleaseProfile: %v", err)
	}
	if res, err := admin.ListReleaseProfiles(ctx, &acquisitionv1.ListReleaseProfilesRequest{}); err != nil || len(res.GetProfiles()) != 0 {
		t.Errorf("ListReleaseProfiles = %v, %v", res, err)
	}

	mapping := acquisitionv1.RemotePathMapping_builder{Host: new("seedbox"), RemotePath: new("/downloads/"), LocalPath: new("/mnt/seedbox/")}.Build()
	created, err := admin.CreateRemotePathMapping(ctx, acquisitionv1.CreateRemotePathMappingRequest_builder{Mapping: mapping}.Build())
	if err != nil {
		t.Fatalf("CreateRemotePathMapping: %v", err)
	}
	_, err = admin.CreateRemotePathMapping(ctx, acquisitionv1.CreateRemotePathMappingRequest_builder{Mapping: mapping}.Build())
	wantCode(t, "duplicate mapping", err, connect.CodeAlreadyExists)
	relative := acquisitionv1.RemotePathMapping_builder{Host: new("seedbox"), RemotePath: new("/other/"), LocalPath: new("seedbox")}.Build()
	_, err = admin.CreateRemotePathMapping(ctx, acquisitionv1.CreateRemotePathMappingRequest_builder{Mapping: relative}.Build())
	wantCode(t, "relative local path", err, connect.CodeInvalidArgument)
	if res, err := admin.ListRemotePathMappings(ctx, &acquisitionv1.ListRemotePathMappingsRequest{}); err != nil ||
		len(res.GetMappings()) != 1 || res.GetMappings()[0].GetLocalPath() != "/mnt/seedbox/" {
		t.Errorf("ListRemotePathMappings = %v, %v", res, err)
	}
	id := created.GetMapping().GetId()
	if _, err := admin.DeleteRemotePathMapping(ctx, acquisitionv1.DeleteRemotePathMappingRequest_builder{Id: new(id)}.Build()); err != nil {
		t.Errorf("DeleteRemotePathMapping: %v", err)
	}
	_, err = admin.DeleteRemotePathMapping(ctx, acquisitionv1.DeleteRemotePathMappingRequest_builder{Id: new(id)}.Build())
	wantCode(t, "second deletion", err, connect.CodeNotFound)
}

// TestAcquisitionSettings checks the acquisition settings' round trip
// through the server settings.
func TestAcquisitionSettings(t *testing.T) {
	ctx := t.Context()
	db := openStore(t)
	m, err := settings.Open(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	url := testHandlers(t, func(opts connect.HandlerOption) (string, http.Handler) {
		return systemv1connect.NewSystemServiceHandler(&SystemService{Settings: m}, opts)
	})
	system := systemv1connect.NewSystemServiceClient(http.DefaultClient, url, as("admin"))
	get := func() *systemv1.ServerSettings {
		t.Helper()
		res, err := system.GetServerSettings(ctx, &systemv1.GetServerSettingsRequest{})
		if err != nil {
			t.Fatalf("GetServerSettings: %v", err)
		}
		return res.GetSettings()
	}
	update := func(s *systemv1.ServerSettings) error {
		_, err := system.UpdateServerSettings(ctx, systemv1.UpdateServerSettingsRequest_builder{Settings: s}.Build())
		return err
	}

	set := get()
	a := set.GetAcquisition()
	if a.GetPropers() != systemv1.ProperDownload_PROPER_DOWNLOAD_PREFER_AND_UPGRADE || a.GetColonReplacement() != systemv1.ColonReplacement_COLON_REPLACEMENT_SMART ||
		a.GetRssSyncInterval() != 30 || a.GetMovieFolderFormat() == "" || !slices.Equal(a.GetExtraFileExtensions(), []string{"srt"}) {
		t.Errorf("default acquisition settings = %v", a)
	}
	a.SetPropers(systemv1.ProperDownload_PROPER_DOWNLOAD_DO_NOT_PREFER)
	a.SetColonReplacement(systemv1.ColonReplacement_COLON_REPLACEMENT_SPACE_DASH)
	a.SetMinimumAge(15)
	a.SetRecycleBin("/trash")
	a.SetWhitelistedHardcodedSubs([]string{"kor"})
	a.SetRenameMovies(true)
	a.SetRenameEpisodes(true)
	a.SetMultiEpisodeStyle(systemv1.MultiEpisodeStyle_MULTI_EPISODE_STYLE_SCENE)
	a.SetSeasonFolderFormat("S{season:00}")
	a.SetAutoUnmonitorPreviouslyDownloadedEpisodes(true)
	a.SetEpisodeTitleRequired(systemv1.EpisodeTitleRequired_EPISODE_TITLE_REQUIRED_NEVER)
	if err := update(set); err != nil {
		t.Fatalf("UpdateServerSettings: %v", err)
	}
	stored := m.Get().Acquisition
	if stored.Propers != core.PropersDoNotPrefer || stored.Naming.ColonReplacement != core.ColonSpaceDash || stored.MinimumAge != 15 ||
		stored.RecycleBin != "/trash" || !slices.Equal(stored.WhitelistedHardcodedSubs, []string{"kor"}) || !stored.Naming.RenameMovies ||
		!stored.Naming.RenameEpisodes || stored.Naming.MultiEpisodeStyle != core.MultiEpisodeScene || stored.Naming.SeasonFolderFormat != "S{season:00}" ||
		!stored.AutoUnmonitorPreviouslyDownloadedEpisodes || stored.EpisodeTitleRequired != core.EpisodeTitleNever {
		t.Errorf("stored acquisition settings = %+v", stored)
	}

	// Settings without acquisition keep the current ones.
	set = get()
	set.ClearAcquisition()
	set.SetPluginCatalogs([]string{"https://example.com/catalog.json"})
	if err := update(set); err != nil {
		t.Fatalf("UpdateServerSettings without acquisition: %v", err)
	}
	if got := get().GetAcquisition(); got.GetMinimumAge() != 15 || got.GetPropers() != systemv1.ProperDownload_PROPER_DOWNLOAD_DO_NOT_PREFER {
		t.Errorf("acquisition settings after an update without them = %v", got)
	}

	for name, change := range map[string]func(*systemv1.AcquisitionSettings){
		"RSS sync too often":   func(a *systemv1.AcquisitionSettings) { a.SetRssSyncInterval(5) },
		"relative recycle bin": func(a *systemv1.AcquisitionSettings) { a.SetRecycleBin("trash") },
		"no colon replacement": func(a *systemv1.AcquisitionSettings) {
			a.SetColonReplacement(systemv1.ColonReplacement_COLON_REPLACEMENT_UNSPECIFIED)
		},
		"no movie file format":  func(a *systemv1.AcquisitionSettings) { a.SetStandardMovieFormat("") },
		"availability too late": func(a *systemv1.AcquisitionSettings) { a.SetAvailabilityDelay(400) },
	} {
		s := get()
		change(s.GetAcquisition())
		wantCode(t, name, update(s), connect.CodeInvalidArgument)
	}
}

// TestAcquisitionEnumsMapped checks that every enum value of the
// acquisition contracts but the unspecified one has a core counterpart.
func TestAcquisitionEnumsMapped(t *testing.T) {
	check := func(enum string, names map[int32]string, mapped func(int32) bool) {
		for v, name := range names {
			if v != 0 && !mapped(v) {
				t.Errorf("%s %s has no core counterpart", enum, name)
			}
		}
	}
	check("MediaKind", acquisitionv1.MediaKind_name, func(v int32) bool { _, ok := acquisitionKinds[acquisitionv1.MediaKind(v)]; return ok })
	check("DownloadProtocol", acquisitionv1.DownloadProtocol_name, func(v int32) bool {
		_, ok := downloadProtocols[acquisitionv1.DownloadProtocol(v)]
		return ok
	})
	check("CustomFormatSpecKind", acquisitionv1.CustomFormatSpecKind_name, func(v int32) bool {
		_, ok := specKinds[acquisitionv1.CustomFormatSpecKind(v)]
		return ok
	})
	check("ProperDownload", systemv1.ProperDownload_name, func(v int32) bool {
		return keyOf(properDownloads, systemv1.ProperDownload(v)) != ""
	})
	check("ColonReplacement", systemv1.ColonReplacement_name, func(v int32) bool {
		return keyOf(colonReplacements, systemv1.ColonReplacement(v)) != ""
	})
	if len(specKinds) != len(core.CustomFormatSpecKinds) || len(properDownloads) != len(core.ProperDownloads) ||
		len(colonReplacements) != len(core.ColonReplacements) || len(downloadProtocols) != len(core.Protocols) {
		t.Error("an enum of core has no contract counterpart")
	}
}
