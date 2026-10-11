package rpc

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/core"
	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"
)

// ProfileService implements mavio.acquisition.v1.ProfileService.
type ProfileService struct {
	store core.Store
}

var _ acquisitionv1connect.ProfileServiceHandler = (*ProfileService)(nil)

// NewProfileService returns a ProfileService backed by store.
func NewProfileService(store core.Store) *ProfileService { return &ProfileService{store: store} }

// invalid reports err as INVALID_ARGUMENT.
func invalid(err error) error { return connect.NewError(connect.CodeInvalidArgument, err) }

// failedPrecondition maps a conflict to FAILED_PRECONDITION and other
// errors as connectError does.
func failedPrecondition(ctx context.Context, err error) error {
	if errors.Is(err, core.ErrConflict) {
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return connectError(ctx, err)
}

var acquisitionKinds = map[acquisitionv1.MediaKind]core.ItemKind{
	acquisitionv1.MediaKind_MEDIA_KIND_MOVIE:  core.KindMovie,
	acquisitionv1.MediaKind_MEDIA_KIND_SERIES: core.KindSeries,
}

// mediaKind returns the item kind of a kind a request requires.
func mediaKind(k acquisitionv1.MediaKind) (core.ItemKind, error) {
	if kind, ok := acquisitionKinds[k]; ok {
		return kind, nil
	}
	return "", invalid(fmt.Errorf("unknown media kind %v", k))
}

var downloadProtocols = map[acquisitionv1.DownloadProtocol]core.Protocol{
	acquisitionv1.DownloadProtocol_DOWNLOAD_PROTOCOL_TORRENT: core.ProtocolTorrent,
	acquisitionv1.DownloadProtocol_DOWNLOAD_PROTOCOL_USENET:  core.ProtocolUsenet,
}

var specKinds = map[acquisitionv1.CustomFormatSpecKind]core.CustomFormatSpecKind{
	acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_RELEASE_TITLE:    core.SpecReleaseTitle,
	acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_EDITION:          core.SpecEdition,
	acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_LANGUAGE:         core.SpecLanguage,
	acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_INDEXER_FLAG:     core.SpecIndexerFlag,
	acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_SOURCE:           core.SpecSource,
	acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_RESOLUTION:       core.SpecResolution,
	acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_QUALITY_MODIFIER: core.SpecQualityModifier,
	acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_SIZE:             core.SpecSize,
	acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_RELEASE_GROUP:    core.SpecReleaseGroup,
	acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_YEAR:             core.SpecYear,
	acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_RELEASE_TYPE:     core.SpecReleaseType,
}

// ListQualityDefinitions lists a kind's definitions, the stored ones over
// the defaults.
func (s *ProfileService) ListQualityDefinitions(ctx context.Context, req *acquisitionv1.ListQualityDefinitionsRequest) (*acquisitionv1.ListQualityDefinitionsResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	kind, err := mediaKind(req.GetKind())
	if err != nil {
		return nil, err
	}
	defs, err := s.definitions(ctx, kind)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return acquisitionv1.ListQualityDefinitionsResponse_builder{Definitions: definitionsToProto(defs)}.Build(), nil
}

// definitions returns a kind's definitions in their default order, the
// stored ones over the defaults.
func (s *ProfileService) definitions(ctx context.Context, kind core.ItemKind) ([]core.QualityDefinition, error) {
	stored, err := s.store.QualityDefinitions().List(ctx, kind)
	if err != nil {
		return nil, err
	}
	merged, err := acquire.DefinitionsFromCore(kind, stored)
	if err != nil {
		return nil, fmt.Errorf("quality definitions of %s: %w", kind, err)
	}
	out := make([]core.QualityDefinition, len(merged))
	for i, d := range merged {
		if out[i], err = acquire.DefinitionToCore(d, kind); err != nil {
			return nil, fmt.Errorf("quality definitions of %s: %w", kind, err)
		}
	}
	return out, nil
}

func definitionsToProto(defs []core.QualityDefinition) []*acquisitionv1.QualityDefinition {
	out := make([]*acquisitionv1.QualityDefinition, len(defs))
	for i, d := range defs {
		out[i] = acquisitionv1.QualityDefinition_builder{
			Quality: new(int32(d.Quality)), Title: new(d.Title),
			MinSize: new(d.MinSize), MaxSize: new(d.MaxSize), PreferredSize: new(d.PreferredSize),
		}.Build()
	}
	return out
}

// UpdateQualityDefinitions changes the sizes, and titles when given, of
// the given qualities of a kind.
func (s *ProfileService) UpdateQualityDefinitions(ctx context.Context, req *acquisitionv1.UpdateQualityDefinitionsRequest) (*acquisitionv1.UpdateQualityDefinitionsResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	kind, err := mediaKind(req.GetKind())
	if err != nil {
		return nil, err
	}
	defs, err := s.definitions(ctx, kind)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	seen := map[int32]bool{}
	for _, d := range req.GetDefinitions() {
		if seen[d.GetQuality()] {
			return nil, invalid(fmt.Errorf("quality %d is given twice", d.GetQuality()))
		}
		seen[d.GetQuality()] = true
		i := slices.IndexFunc(defs, func(c core.QualityDefinition) bool { return c.Quality == int(d.GetQuality()) })
		if i < 0 {
			return nil, invalid(fmt.Errorf("quality %d is not one of %s", d.GetQuality(), kind))
		}
		if d.GetTitle() != "" {
			defs[i].Title = d.GetTitle()
		}
		defs[i].MinSize, defs[i].MaxSize, defs[i].PreferredSize = d.GetMinSize(), d.GetMaxSize(), d.GetPreferredSize()
		if err := defs[i].Validate(); err != nil {
			return nil, connectError(ctx, err)
		}
	}
	if err := s.store.QualityDefinitions().Replace(ctx, kind, defs); err != nil {
		return nil, connectError(ctx, err)
	}
	return acquisitionv1.UpdateQualityDefinitionsResponse_builder{Definitions: definitionsToProto(defs)}.Build(), nil
}

// ListQualityProfiles lists the quality profiles of a kind, or all; any
// user may.
func (s *ProfileService) ListQualityProfiles(ctx context.Context, req *acquisitionv1.ListQualityProfilesRequest) (*acquisitionv1.ListQualityProfilesResponse, error) {
	if _, err := principal(ctx); err != nil {
		return nil, err
	}
	var kind core.ItemKind
	if req.GetKind() != acquisitionv1.MediaKind_MEDIA_KIND_UNSPECIFIED {
		var err error
		if kind, err = mediaKind(req.GetKind()); err != nil {
			return nil, err
		}
	}
	profiles, err := s.store.QualityProfiles().List(ctx, kind)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := make([]*acquisitionv1.QualityProfile, len(profiles))
	for i := range profiles {
		out[i] = qualityProfileToProto(&profiles[i])
	}
	return acquisitionv1.ListQualityProfilesResponse_builder{Profiles: out}.Build(), nil
}

// CreateQualityProfile adds a quality profile.
func (s *ProfileService) CreateQualityProfile(ctx context.Context, req *acquisitionv1.CreateQualityProfileRequest) (*acquisitionv1.CreateQualityProfileResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	kind, err := mediaKind(req.GetProfile().GetKind())
	if err != nil {
		return nil, err
	}
	p, err := qualityProfileFromProto(req.GetProfile())
	if err != nil {
		return nil, err
	}
	p.ID, p.Kind = core.NewID(), kind
	if err := s.putQualityProfile(ctx, &p); err != nil {
		return nil, err
	}
	return acquisitionv1.CreateQualityProfileResponse_builder{Profile: qualityProfileToProto(&p)}.Build(), nil
}

// UpdateQualityProfile changes a quality profile but for its kind.
func (s *ProfileService) UpdateQualityProfile(ctx context.Context, req *acquisitionv1.UpdateQualityProfileRequest) (*acquisitionv1.UpdateQualityProfileResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	id, err := core.ParseID(req.GetProfile().GetId())
	if err != nil {
		return nil, connectError(ctx, err)
	}
	old, err := s.store.QualityProfiles().Get(ctx, id)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	if k := req.GetProfile().GetKind(); k != acquisitionv1.MediaKind_MEDIA_KIND_UNSPECIFIED && acquisitionKinds[k] != old.Kind {
		return nil, invalid(errors.New("the kind of a quality profile cannot change"))
	}
	p, err := qualityProfileFromProto(req.GetProfile())
	if err != nil {
		return nil, err
	}
	p.ID, p.Kind = id, old.Kind
	if err := s.putQualityProfile(ctx, &p); err != nil {
		return nil, err
	}
	return acquisitionv1.UpdateQualityProfileResponse_builder{Profile: qualityProfileToProto(&p)}.Build(), nil
}

// putQualityProfile checks a profile against its kind's qualities and the
// custom formats that exist, then stores it.
func (s *ProfileService) putQualityProfile(ctx context.Context, p *core.QualityProfile) error {
	ap, err := acquire.QualityProfileFromCore(*p)
	if err != nil {
		return invalid(err)
	}
	kind, err := acquire.KindFromCore(p.Kind)
	if err != nil {
		return invalid(err)
	}
	if err := ap.Validate(kind); err != nil {
		return invalid(fmt.Errorf("quality profile %q: %w", p.Name, err))
	}
	formats, err := s.store.CustomFormats().List(ctx)
	if err != nil {
		return connectError(ctx, err)
	}
	ids := make([]core.ID, len(formats))
	for i, f := range formats {
		ids[i] = f.ID
	}
	if err := ap.ValidateFormatScores(ids); err != nil {
		return invalid(fmt.Errorf("quality profile %q: %w", p.Name, err))
	}
	if err := s.store.QualityProfiles().Put(ctx, p); err != nil {
		return connectError(ctx, err)
	}
	return nil
}

// DeleteQualityProfile deletes a quality profile no item uses.
func (s *ProfileService) DeleteQualityProfile(ctx context.Context, req *acquisitionv1.DeleteQualityProfileRequest) (*acquisitionv1.DeleteQualityProfileResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	id, err := core.ParseID(req.GetId())
	if err != nil {
		return nil, connectError(ctx, err)
	}
	if err := s.store.QualityProfiles().Delete(ctx, id); err != nil {
		return nil, failedPrecondition(ctx, err)
	}
	return &acquisitionv1.DeleteQualityProfileResponse{}, nil
}

func qualityProfileFromProto(p *acquisitionv1.QualityProfile) (core.QualityProfile, error) {
	out := core.QualityProfile{
		Name: strings.TrimSpace(p.GetName()), UpgradeAllowed: p.GetUpgradeAllowed(), Cutoff: int(p.GetCutoff()),
		Items:          profileItemsFromProto(p.GetItems()),
		MinFormatScore: int(p.GetMinFormatScore()), CutoffFormatScore: int(p.GetCutoffFormatScore()),
		MinUpgradeFormatScore: int(p.GetMinUpgradeFormatScore()), Language: p.GetLanguage(),
	}
	for _, sc := range p.GetFormatScores() {
		id, err := core.ParseID(sc.GetFormatId())
		if err != nil {
			return core.QualityProfile{}, invalid(err)
		}
		out.FormatScores = append(out.FormatScores, core.FormatScore{FormatID: id, Score: int(sc.GetScore())})
	}
	return out, nil
}

func profileItemsFromProto(items []*acquisitionv1.QualityProfileItem) []core.QualityProfileItem {
	var out []core.QualityProfileItem
	for _, it := range items {
		out = append(out, core.QualityProfileItem{
			ID: int(it.GetId()), Name: it.GetName(), Quality: int(it.GetQuality()),
			Items: profileItemsFromProto(it.GetItems()), Allowed: it.GetAllowed(),
		})
	}
	return out
}

func qualityProfileToProto(p *core.QualityProfile) *acquisitionv1.QualityProfile {
	scores := make([]*acquisitionv1.FormatScore, len(p.FormatScores))
	for i, sc := range p.FormatScores {
		scores[i] = acquisitionv1.FormatScore_builder{FormatId: new(sc.FormatID.String()), Score: new(int32(sc.Score))}.Build()
	}
	return acquisitionv1.QualityProfile_builder{
		Id: new(p.ID.String()), Kind: new(keyOf(acquisitionKinds, p.Kind)), Name: new(p.Name), UpgradeAllowed: new(p.UpgradeAllowed),
		Cutoff: new(int32(p.Cutoff)), Items: profileItemsToProto(p.Items),
		MinFormatScore: new(int32(p.MinFormatScore)), CutoffFormatScore: new(int32(p.CutoffFormatScore)),
		MinUpgradeFormatScore: new(int32(p.MinUpgradeFormatScore)), FormatScores: scores, Language: new(p.Language),
	}.Build()
}

func profileItemsToProto(items []core.QualityProfileItem) []*acquisitionv1.QualityProfileItem {
	out := make([]*acquisitionv1.QualityProfileItem, len(items))
	for i, it := range items {
		out[i] = acquisitionv1.QualityProfileItem_builder{
			Id: new(int32(it.ID)), Name: new(it.Name), Quality: new(int32(it.Quality)),
			Items: profileItemsToProto(it.Items), Allowed: new(it.Allowed),
		}.Build()
	}
	return out
}

// ListCustomFormats lists the custom formats.
func (s *ProfileService) ListCustomFormats(ctx context.Context, _ *acquisitionv1.ListCustomFormatsRequest) (*acquisitionv1.ListCustomFormatsResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	formats, err := s.store.CustomFormats().List(ctx)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := make([]*acquisitionv1.CustomFormat, len(formats))
	for i := range formats {
		out[i] = customFormatToProto(&formats[i])
	}
	return acquisitionv1.ListCustomFormatsResponse_builder{Formats: out}.Build(), nil
}

// CreateCustomFormat adds a custom format.
func (s *ProfileService) CreateCustomFormat(ctx context.Context, req *acquisitionv1.CreateCustomFormatRequest) (*acquisitionv1.CreateCustomFormatResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	f, err := customFormatFromProto(req.GetFormat())
	if err != nil {
		return nil, err
	}
	f.ID = core.NewID()
	if err := s.putCustomFormat(ctx, &f); err != nil {
		return nil, err
	}
	return acquisitionv1.CreateCustomFormatResponse_builder{Format: customFormatToProto(&f)}.Build(), nil
}

// UpdateCustomFormat changes a custom format.
func (s *ProfileService) UpdateCustomFormat(ctx context.Context, req *acquisitionv1.UpdateCustomFormatRequest) (*acquisitionv1.UpdateCustomFormatResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	id, err := core.ParseID(req.GetFormat().GetId())
	if err != nil {
		return nil, connectError(ctx, err)
	}
	if _, err := s.store.CustomFormats().Get(ctx, id); err != nil {
		return nil, connectError(ctx, err)
	}
	f, err := customFormatFromProto(req.GetFormat())
	if err != nil {
		return nil, err
	}
	f.ID = id
	if err := s.putCustomFormat(ctx, &f); err != nil {
		return nil, err
	}
	return acquisitionv1.UpdateCustomFormatResponse_builder{Format: customFormatToProto(&f)}.Build(), nil
}

// putCustomFormat checks a format's conditions, compiling its
// expressions, then stores it.
func (s *ProfileService) putCustomFormat(ctx context.Context, f *core.CustomFormat) error {
	af, err := acquire.CustomFormatFromCore(*f)
	if err != nil {
		return invalid(err)
	}
	if err := af.Validate(); err != nil {
		return invalid(fmt.Errorf("custom format %q: %w", f.Name, err))
	}
	if err := s.store.CustomFormats().Put(ctx, f); err != nil {
		return connectError(ctx, err)
	}
	return nil
}

// DeleteCustomFormat deletes a custom format and its scores in profiles.
func (s *ProfileService) DeleteCustomFormat(ctx context.Context, req *acquisitionv1.DeleteCustomFormatRequest) (*acquisitionv1.DeleteCustomFormatResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	id, err := core.ParseID(req.GetId())
	if err != nil {
		return nil, connectError(ctx, err)
	}
	if err := s.store.CustomFormats().Delete(ctx, id); err != nil {
		return nil, connectError(ctx, err)
	}
	return &acquisitionv1.DeleteCustomFormatResponse{}, nil
}

func customFormatFromProto(f *acquisitionv1.CustomFormat) (core.CustomFormat, error) {
	out := core.CustomFormat{Name: strings.TrimSpace(f.GetName()), IncludeWhenRenaming: f.GetIncludeWhenRenaming()}
	for _, sp := range f.GetSpecifications() {
		kind, ok := specKinds[sp.GetKind()]
		if !ok {
			return core.CustomFormat{}, invalid(fmt.Errorf("condition %q has no kind", sp.GetName()))
		}
		out.Specifications = append(out.Specifications, core.CustomFormatSpec{
			Kind: kind, Name: sp.GetName(), Negate: sp.GetNegate(), Required: sp.GetRequired(), Value: sp.GetValue(),
			ExceptLanguage: sp.GetExceptLanguage(), Min: sp.GetMin(), Max: sp.GetMax(),
		})
	}
	return out, nil
}

func customFormatToProto(f *core.CustomFormat) *acquisitionv1.CustomFormat {
	specs := make([]*acquisitionv1.CustomFormatSpec, len(f.Specifications))
	for i, sp := range f.Specifications {
		specs[i] = acquisitionv1.CustomFormatSpec_builder{
			Kind: new(keyOf(specKinds, sp.Kind)), Name: new(sp.Name), Negate: new(sp.Negate), Required: new(sp.Required),
			Value: new(sp.Value), ExceptLanguage: new(sp.ExceptLanguage), Min: new(sp.Min), Max: new(sp.Max),
		}.Build()
	}
	return acquisitionv1.CustomFormat_builder{
		Id: new(f.ID.String()), Name: new(f.Name), IncludeWhenRenaming: new(f.IncludeWhenRenaming), Specifications: specs,
	}.Build()
}

// defaultDelayOrder is the order of the default delay profile, after all
// others as in Radarr.
const defaultDelayOrder = math.MaxInt32

// defaultDelayProfile is Radarr's default delay profile: both protocols,
// usenet preferred, no delay.
func defaultDelayProfile() core.DelayProfile {
	return core.DelayProfile{
		ID: core.NewID(), Default: true, Order: defaultDelayOrder,
		EnableUsenet: true, EnableTorrent: true, PreferredProtocol: core.ProtocolUsenet,
	}
}

// delayProfiles returns the delay profiles by order, storing the default
// one first when there is none.
func (s *ProfileService) delayProfiles(ctx context.Context, store core.Store) ([]core.DelayProfile, error) {
	profiles, err := store.DelayProfiles().List(ctx)
	if err != nil || slices.ContainsFunc(profiles, func(p core.DelayProfile) bool { return p.Default }) {
		return profiles, err
	}
	def := defaultDelayProfile()
	// A concurrent request may have stored it first.
	if err := store.DelayProfiles().Put(ctx, &def); err != nil && !errors.Is(err, core.ErrConflict) {
		return nil, err
	}
	return store.DelayProfiles().List(ctx)
}

// ListDelayProfiles lists the delay profiles by order, the default last.
func (s *ProfileService) ListDelayProfiles(ctx context.Context, _ *acquisitionv1.ListDelayProfilesRequest) (*acquisitionv1.ListDelayProfilesResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	profiles, err := s.delayProfiles(ctx, s.store)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return acquisitionv1.ListDelayProfilesResponse_builder{Profiles: delayProfilesToProto(profiles)}.Build(), nil
}

// CreateDelayProfile adds a delay profile before the default one.
func (s *ProfileService) CreateDelayProfile(ctx context.Context, req *acquisitionv1.CreateDelayProfileRequest) (*acquisitionv1.CreateDelayProfileResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	p := delayProfileFromProto(req.GetProfile())
	p.ID = core.NewID()
	err := s.store.InTx(ctx, func(tx core.Store) error {
		profiles, err := s.delayProfiles(ctx, tx)
		if err != nil {
			return err
		}
		for _, o := range profiles {
			if !o.Default {
				p.Order = max(p.Order, o.Order)
			}
		}
		p.Order++
		return tx.DelayProfiles().Put(ctx, &p)
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return acquisitionv1.CreateDelayProfileResponse_builder{Profile: delayProfileToProto(&p)}.Build(), nil
}

// UpdateDelayProfile changes a delay profile but for its order and
// whether it is the default.
func (s *ProfileService) UpdateDelayProfile(ctx context.Context, req *acquisitionv1.UpdateDelayProfileRequest) (*acquisitionv1.UpdateDelayProfileResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	id, err := core.ParseID(req.GetProfile().GetId())
	if err != nil {
		return nil, connectError(ctx, err)
	}
	old, err := s.store.DelayProfiles().Get(ctx, id)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	p := delayProfileFromProto(req.GetProfile())
	p.ID, p.Default, p.Order = id, old.Default, old.Order
	if err := s.store.DelayProfiles().Put(ctx, &p); err != nil {
		return nil, connectError(ctx, err)
	}
	return acquisitionv1.UpdateDelayProfileResponse_builder{Profile: delayProfileToProto(&p)}.Build(), nil
}

// DeleteDelayProfile deletes a delay profile other than the default one
// and renumbers the others.
func (s *ProfileService) DeleteDelayProfile(ctx context.Context, req *acquisitionv1.DeleteDelayProfileRequest) (*acquisitionv1.DeleteDelayProfileResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	id, err := core.ParseID(req.GetId())
	if err != nil {
		return nil, connectError(ctx, err)
	}
	err = s.store.InTx(ctx, func(tx core.Store) error {
		p, err := tx.DelayProfiles().Get(ctx, id)
		if err != nil {
			return err
		}
		if p.Default {
			return fmt.Errorf("delete delay profile %s: %w: it is the default", id, core.ErrConflict)
		}
		if err := tx.DelayProfiles().Delete(ctx, id); err != nil {
			return err
		}
		profiles, err := tx.DelayProfiles().List(ctx)
		if err != nil {
			return err
		}
		return renumberDelayProfiles(ctx, tx, profiles)
	})
	if err != nil {
		return nil, failedPrecondition(ctx, err)
	}
	return &acquisitionv1.DeleteDelayProfileResponse{}, nil
}

// renumberDelayProfiles numbers the profiles other than the default one
// from 1 in their order, storing those whose order changes.
func renumberDelayProfiles(ctx context.Context, tx core.Store, profiles []core.DelayProfile) error {
	n := 0
	for i := range profiles {
		p := &profiles[i]
		if p.Default {
			continue
		}
		n++
		if p.Order == n {
			continue
		}
		p.Order = n
		if err := tx.DelayProfiles().Put(ctx, p); err != nil {
			return err
		}
	}
	return nil
}

// ReorderDelayProfiles orders the profiles other than the default one as
// the request lists them.
func (s *ProfileService) ReorderDelayProfiles(ctx context.Context, req *acquisitionv1.ReorderDelayProfilesRequest) (*acquisitionv1.ReorderDelayProfilesResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	var out []core.DelayProfile
	err := s.store.InTx(ctx, func(tx core.Store) error {
		profiles, err := s.delayProfiles(ctx, tx)
		if err != nil {
			return err
		}
		others := slices.DeleteFunc(slices.Clone(profiles), func(p core.DelayProfile) bool { return p.Default })
		ids := req.GetIds()
		if len(ids) != len(others) {
			return invalid(fmt.Errorf("%d profiles to order, %d given", len(others), len(ids)))
		}
		ordered := make([]core.DelayProfile, 0, len(profiles))
		for _, raw := range ids {
			id, err := core.ParseID(raw)
			if err != nil {
				return err
			}
			i := slices.IndexFunc(others, func(p core.DelayProfile) bool { return p.ID == id })
			if i < 0 {
				return invalid(fmt.Errorf("delay profile %s is not to be ordered or is given twice", id))
			}
			ordered = append(ordered, others[i])
			others = slices.Delete(others, i, i+1)
		}
		ordered = append(ordered, profiles[slices.IndexFunc(profiles, func(p core.DelayProfile) bool { return p.Default })])
		if err := renumberDelayProfiles(ctx, tx, ordered); err != nil {
			return err
		}
		out = ordered
		return nil
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return acquisitionv1.ReorderDelayProfilesResponse_builder{Profiles: delayProfilesToProto(out)}.Build(), nil
}

func delayProfileFromProto(p *acquisitionv1.DelayProfile) core.DelayProfile {
	return core.DelayProfile{
		EnableUsenet: p.GetEnableUsenet(), EnableTorrent: p.GetEnableTorrent(),
		PreferredProtocol: downloadProtocols[p.GetPreferredProtocol()],
		UsenetDelay:       p.GetUsenetDelay().AsDuration(), TorrentDelay: p.GetTorrentDelay().AsDuration(),
		BypassIfHighestQuality:         p.GetBypassIfHighestQuality(),
		BypassIfAboveCustomFormatScore: p.GetBypassIfAboveCustomFormatScore(),
		MinimumCustomFormatScore:       int(p.GetMinimumCustomFormatScore()),
		Tags:                           p.GetTags(),
	}
}

func delayProfilesToProto(profiles []core.DelayProfile) []*acquisitionv1.DelayProfile {
	out := make([]*acquisitionv1.DelayProfile, len(profiles))
	for i := range profiles {
		out[i] = delayProfileToProto(&profiles[i])
	}
	return out
}

func delayProfileToProto(p *core.DelayProfile) *acquisitionv1.DelayProfile {
	return acquisitionv1.DelayProfile_builder{
		Id: new(p.ID.String()), Default: new(p.Default), Order: new(int32(min(p.Order, math.MaxInt32))),
		EnableUsenet: new(p.EnableUsenet), EnableTorrent: new(p.EnableTorrent),
		PreferredProtocol: new(keyOf(downloadProtocols, p.PreferredProtocol)),
		UsenetDelay:       durationpb.New(p.UsenetDelay), TorrentDelay: durationpb.New(p.TorrentDelay),
		BypassIfHighestQuality:         new(p.BypassIfHighestQuality),
		BypassIfAboveCustomFormatScore: new(p.BypassIfAboveCustomFormatScore),
		MinimumCustomFormatScore:       new(int32(p.MinimumCustomFormatScore)),
		Tags:                           p.Tags,
	}.Build()
}

// ListReleaseProfiles lists the release profiles.
func (s *ProfileService) ListReleaseProfiles(ctx context.Context, _ *acquisitionv1.ListReleaseProfilesRequest) (*acquisitionv1.ListReleaseProfilesResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	profiles, err := s.store.ReleaseProfiles().List(ctx)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := make([]*acquisitionv1.ReleaseProfile, len(profiles))
	for i := range profiles {
		out[i] = releaseProfileToProto(&profiles[i])
	}
	return acquisitionv1.ListReleaseProfilesResponse_builder{Profiles: out}.Build(), nil
}

// CreateReleaseProfile adds a release profile.
func (s *ProfileService) CreateReleaseProfile(ctx context.Context, req *acquisitionv1.CreateReleaseProfileRequest) (*acquisitionv1.CreateReleaseProfileResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	p := releaseProfileFromProto(req.GetProfile())
	p.ID = core.NewID()
	if err := s.putReleaseProfile(ctx, &p); err != nil {
		return nil, err
	}
	return acquisitionv1.CreateReleaseProfileResponse_builder{Profile: releaseProfileToProto(&p)}.Build(), nil
}

// UpdateReleaseProfile changes a release profile.
func (s *ProfileService) UpdateReleaseProfile(ctx context.Context, req *acquisitionv1.UpdateReleaseProfileRequest) (*acquisitionv1.UpdateReleaseProfileResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	id, err := core.ParseID(req.GetProfile().GetId())
	if err != nil {
		return nil, connectError(ctx, err)
	}
	if _, err := s.store.ReleaseProfiles().Get(ctx, id); err != nil {
		return nil, connectError(ctx, err)
	}
	p := releaseProfileFromProto(req.GetProfile())
	p.ID = id
	if err := s.putReleaseProfile(ctx, &p); err != nil {
		return nil, err
	}
	return acquisitionv1.UpdateReleaseProfileResponse_builder{Profile: releaseProfileToProto(&p)}.Build(), nil
}

// putReleaseProfile checks a profile's terms, then stores it.
func (s *ProfileService) putReleaseProfile(ctx context.Context, p *core.ReleaseProfile) error {
	ap := acquire.ReleaseProfileFromCore(*p)
	if err := ap.Validate(); err != nil {
		return invalid(err)
	}
	if err := s.store.ReleaseProfiles().Put(ctx, p); err != nil {
		return connectError(ctx, err)
	}
	return nil
}

// DeleteReleaseProfile deletes a release profile.
func (s *ProfileService) DeleteReleaseProfile(ctx context.Context, req *acquisitionv1.DeleteReleaseProfileRequest) (*acquisitionv1.DeleteReleaseProfileResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	id, err := core.ParseID(req.GetId())
	if err != nil {
		return nil, connectError(ctx, err)
	}
	if err := s.store.ReleaseProfiles().Delete(ctx, id); err != nil {
		return nil, connectError(ctx, err)
	}
	return &acquisitionv1.DeleteReleaseProfileResponse{}, nil
}

func releaseProfileFromProto(p *acquisitionv1.ReleaseProfile) core.ReleaseProfile {
	out := core.ReleaseProfile{
		Name: strings.TrimSpace(p.GetName()), Enabled: p.GetEnabled(), Required: p.GetRequired(), Ignored: p.GetIgnored(), Tags: p.GetTags(),
	}
	if p.HasIndexer() {
		out.IndexerID = p.GetIndexer().GetPluginId() + "/" + p.GetIndexer().GetIndexerId()
	}
	return out
}

func releaseProfileToProto(p *core.ReleaseProfile) *acquisitionv1.ReleaseProfile {
	b := acquisitionv1.ReleaseProfile_builder{
		Id: new(p.ID.String()), Name: new(p.Name), Enabled: new(p.Enabled), Required: p.Required, Ignored: p.Ignored, Tags: p.Tags,
	}
	if plugin, indexer, ok := strings.Cut(p.IndexerID, "/"); ok {
		b.Indexer = acquisitionv1.IndexerRef_builder{PluginId: new(plugin), IndexerId: new(indexer)}.Build()
	}
	return b.Build()
}

// ListRemotePathMappings lists the remote path mappings.
func (s *ProfileService) ListRemotePathMappings(ctx context.Context, _ *acquisitionv1.ListRemotePathMappingsRequest) (*acquisitionv1.ListRemotePathMappingsResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	mappings, err := s.store.RemotePathMappings().List(ctx)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := make([]*acquisitionv1.RemotePathMapping, len(mappings))
	for i := range mappings {
		out[i] = remotePathMappingToProto(&mappings[i])
	}
	return acquisitionv1.ListRemotePathMappingsResponse_builder{Mappings: out}.Build(), nil
}

// CreateRemotePathMapping adds a remote path mapping; a host's remote path
// is mapped once.
func (s *ProfileService) CreateRemotePathMapping(ctx context.Context, req *acquisitionv1.CreateRemotePathMappingRequest) (*acquisitionv1.CreateRemotePathMappingResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	m := req.GetMapping()
	out := core.RemotePathMapping{
		ID: core.NewID(), Host: strings.TrimSpace(m.GetHost()), RemotePath: strings.TrimSpace(m.GetRemotePath()), LocalPath: m.GetLocalPath(),
	}
	if err := s.store.RemotePathMappings().Put(ctx, &out); err != nil {
		return nil, connectError(ctx, err)
	}
	return acquisitionv1.CreateRemotePathMappingResponse_builder{Mapping: remotePathMappingToProto(&out)}.Build(), nil
}

// DeleteRemotePathMapping deletes a remote path mapping.
func (s *ProfileService) DeleteRemotePathMapping(ctx context.Context, req *acquisitionv1.DeleteRemotePathMappingRequest) (*acquisitionv1.DeleteRemotePathMappingResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	id, err := core.ParseID(req.GetId())
	if err != nil {
		return nil, connectError(ctx, err)
	}
	if err := s.store.RemotePathMappings().Delete(ctx, id); err != nil {
		return nil, connectError(ctx, err)
	}
	return &acquisitionv1.DeleteRemotePathMappingResponse{}, nil
}

func remotePathMappingToProto(m *core.RemotePathMapping) *acquisitionv1.RemotePathMapping {
	return acquisitionv1.RemotePathMapping_builder{
		Id: new(m.ID.String()), Host: new(m.Host), RemotePath: new(m.RemotePath), LocalPath: new(m.LocalPath),
	}.Build()
}
