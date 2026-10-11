package acquire

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/release"
)

// Conversions between the acquisition model libs/core stores and the
// types decisions are made with. Qualities are Radarr's quality IDs in
// core, languages their release names, and custom format values strings.

// ErrUnknownValue is wrapped by conversion errors on a value, such as a
// quality ID, language name or kind, that has no counterpart.
var ErrUnknownValue = errors.New("unknown value")

func unknown(what string, v any) error {
	return fmt.Errorf("%w: %s %v", ErrUnknownValue, what, v)
}

// KindFromCore returns the release kind of a movie or series kind.
func KindFromCore(k core.ItemKind) (release.Kind, error) {
	switch k {
	case core.KindMovie:
		return release.Movie, nil
	case core.KindSeries:
		return release.Series, nil
	}
	return 0, unknown("item kind", k)
}

// KindToCore returns the item kind of a release kind.
func KindToCore(k release.Kind) (core.ItemKind, error) {
	switch k {
	case release.Movie:
		return core.KindMovie, nil
	case release.Series:
		return core.KindSeries, nil
	}
	return "", unknown("release kind", k)
}

// ProtocolFromCore returns the protocol of a stored one.
func ProtocolFromCore(p core.Protocol) (Protocol, error) {
	switch p {
	case core.ProtocolUsenet:
		return ProtocolUsenet, nil
	case core.ProtocolTorrent:
		return ProtocolTorrent, nil
	}
	return ProtocolUnknown, unknown("protocol", p)
}

// ProtocolToCore returns the stored protocol; ProtocolUnknown has none.
func ProtocolToCore(p Protocol) (core.Protocol, error) {
	switch p {
	case ProtocolUsenet:
		return core.ProtocolUsenet, nil
	case ProtocolTorrent:
		return core.ProtocolTorrent, nil
	}
	return "", unknown("protocol", int(p))
}

// ProperDownloadFromCore returns the proper download setting of a stored
// one.
func ProperDownloadFromCore(p core.ProperDownload) (ProperDownload, error) {
	switch p {
	case core.PropersPreferAndUpgrade:
		return PropersPreferAndUpgrade, nil
	case core.PropersDoNotUpgrade:
		return PropersDoNotUpgrade, nil
	case core.PropersDoNotPrefer:
		return PropersDoNotPrefer, nil
	}
	return 0, unknown("proper download setting", p)
}

// ProperDownloadToCore returns the stored proper download setting.
func ProperDownloadToCore(p ProperDownload) (core.ProperDownload, error) {
	switch p {
	case PropersPreferAndUpgrade:
		return core.PropersPreferAndUpgrade, nil
	case PropersDoNotUpgrade:
		return core.PropersDoNotUpgrade, nil
	case PropersDoNotPrefer:
		return core.PropersDoNotPrefer, nil
	}
	return "", unknown("proper download setting", int(p))
}

// qualityFromCore returns the quality of Radarr's quality ID.
func qualityFromCore(id int) (release.Quality, error) {
	q, ok := QualityByID(id)
	if !ok {
		return release.Unknown, unknown("quality ID", id)
	}
	return q, nil
}

// qualityToCore returns Radarr's ID of a quality.
func qualityToCore(q release.Quality) (int, error) {
	id, ok := qualityIDs[q]
	if !ok {
		return 0, unknown("quality", int(q))
	}
	return id, nil
}

// languageFromCore returns the language of a name.
func languageFromCore(name string) (release.Language, error) {
	l, ok := release.LanguageByName(name)
	if !ok {
		return release.LanguageUnknown, unknown("language", strconv.Quote(name))
	}
	return l, nil
}

// languageToCore returns the name of a language.
func languageToCore(l release.Language) (string, error) {
	if l < release.LanguageUnknown || l > release.Original {
		return "", unknown("language", int(l))
	}
	return l.String(), nil
}

// QualityProfileFromCore returns the profile of a stored one; its kind is
// left to the caller, who validates the profile against it.
func QualityProfileFromCore(p core.QualityProfile) (QualityProfile, error) {
	out := QualityProfile{
		ID:                    p.ID,
		Name:                  p.Name,
		UpgradeAllowed:        p.UpgradeAllowed,
		Cutoff:                p.Cutoff,
		MinFormatScore:        p.MinFormatScore,
		CutoffFormatScore:     p.CutoffFormatScore,
		MinUpgradeFormatScore: p.MinUpgradeFormatScore,
	}
	var err error
	if out.Items, err = profileItemsFromCore(p.Items); err != nil {
		return QualityProfile{}, fmt.Errorf("quality profile %q: %w", p.Name, err)
	}
	for _, s := range p.FormatScores {
		out.FormatItems = append(out.FormatItems, FormatScore{FormatID: s.FormatID, Score: s.Score})
	}
	if p.Language != "" {
		if out.Language, err = languageFromCore(p.Language); err != nil {
			return QualityProfile{}, fmt.Errorf("quality profile %q: %w", p.Name, err)
		}
	}
	return out, nil
}

func profileItemsFromCore(items []core.QualityProfileItem) ([]ProfileItem, error) {
	var out []ProfileItem
	for _, it := range items {
		pi := ProfileItem{ID: it.ID, Name: it.Name, Allowed: it.Allowed}
		if it.ID != 0 || len(it.Items) > 0 {
			var err error
			if pi.Items, err = profileItemsFromCore(it.Items); err != nil {
				return nil, fmt.Errorf("group %q: %w", it.Name, err)
			}
		} else {
			q, err := qualityFromCore(it.Quality)
			if err != nil {
				return nil, err
			}
			pi.Quality = q
		}
		out = append(out, pi)
	}
	return out, nil
}

// QualityProfileToCore returns the stored form of a profile of a kind;
// release.LanguageUnknown is stored as any language.
func QualityProfileToCore(p QualityProfile, kind core.ItemKind) (core.QualityProfile, error) {
	out := core.QualityProfile{
		ID:                    p.ID,
		Kind:                  kind,
		Name:                  p.Name,
		UpgradeAllowed:        p.UpgradeAllowed,
		Cutoff:                p.Cutoff,
		MinFormatScore:        p.MinFormatScore,
		CutoffFormatScore:     p.CutoffFormatScore,
		MinUpgradeFormatScore: p.MinUpgradeFormatScore,
	}
	var err error
	if out.Items, err = profileItemsToCore(p.Items); err != nil {
		return core.QualityProfile{}, fmt.Errorf("quality profile %q: %w", p.Name, err)
	}
	for _, s := range p.FormatItems {
		out.FormatScores = append(out.FormatScores, core.FormatScore{FormatID: s.FormatID, Score: s.Score})
	}
	if p.Language != release.LanguageUnknown {
		if out.Language, err = languageToCore(p.Language); err != nil {
			return core.QualityProfile{}, fmt.Errorf("quality profile %q: %w", p.Name, err)
		}
	}
	return out, nil
}

func profileItemsToCore(items []ProfileItem) ([]core.QualityProfileItem, error) {
	var out []core.QualityProfileItem
	for _, it := range items {
		ci := core.QualityProfileItem{ID: it.ID, Name: it.Name, Allowed: it.Allowed}
		if it.IsGroup() {
			var err error
			if ci.Items, err = profileItemsToCore(it.Items); err != nil {
				return nil, fmt.Errorf("group %q: %w", it.Name, err)
			}
		} else {
			id, err := qualityToCore(it.Quality)
			if err != nil {
				return nil, err
			}
			ci.Quality = id
		}
		out = append(out, ci)
	}
	return out, nil
}

// specKinds maps the stored kinds of custom format conditions.
var specKinds = map[core.CustomFormatSpecKind]SpecKind{
	core.SpecReleaseTitle:    SpecReleaseTitle,
	core.SpecEdition:         SpecEdition,
	core.SpecLanguage:        SpecLanguage,
	core.SpecIndexerFlag:     SpecIndexerFlag,
	core.SpecSource:          SpecSource,
	core.SpecResolution:      SpecResolution,
	core.SpecQualityModifier: SpecQualityModifier,
	core.SpecSize:            SpecSize,
	core.SpecReleaseGroup:    SpecReleaseGroup,
	core.SpecYear:            SpecYear,
	core.SpecReleaseType:     SpecReleaseType,
}

// The stored values of sources, modifiers and release types: Radarr's and
// Sonarr's enum names in snake case.
var (
	sourceNames = map[release.Source]string{
		release.SourceUnknown:   "unknown",
		release.SourceCam:       "cam",
		release.SourceTelesync:  "telesync",
		release.SourceTelecine:  "telecine",
		release.SourceWorkprint: "workprint",
		release.SourceDVD:       "dvd",
		release.SourceTV:        "tv",
		release.SourceWebDL:     "webdl",
		release.SourceWebRip:    "webrip",
		release.SourceBluray:    "bluray",
	}
	modifierNames = map[release.Modifier]string{
		release.ModifierNone:     "none",
		release.ModifierRegional: "regional",
		release.ModifierScreener: "screener",
		release.ModifierRawHD:    "rawhd",
		release.ModifierBRDisk:   "brdisk",
		release.ModifierRemux:    "remux",
	}
	releaseTypeNames = map[release.ReleaseType]string{
		release.ReleaseUnknown: "unknown",
		release.SingleEpisode:  "single_episode",
		release.MultiEpisode:   "multi_episode",
		release.SeasonPack:     "season_pack",
	}
)

// byName returns the key of a name in names.
func byName[K comparable](names map[K]string, what, name string) (K, error) {
	for k, n := range names {
		if n == name {
			return k, nil
		}
	}
	var zero K
	return zero, unknown(what, strconv.Quote(name))
}

// nameOf returns the name of a key in names.
func nameOf[K ~int](names map[K]string, what string, k K) (string, error) {
	n, ok := names[k]
	if !ok {
		return "", unknown(what, int(k))
	}
	return n, nil
}

// CustomFormatFromCore returns the custom format of a stored one. Its
// expressions are compiled by Validate or on first use.
func CustomFormatFromCore(f core.CustomFormat) (CustomFormat, error) {
	out := CustomFormat{ID: f.ID, Name: f.Name, IncludeWhenRenaming: f.IncludeWhenRenaming}
	for _, cs := range f.Specifications {
		s, err := specFromCore(cs)
		if err != nil {
			return CustomFormat{}, fmt.Errorf("custom format %q: condition %q: %w", f.Name, cs.Name, err)
		}
		out.Specifications = append(out.Specifications, s)
	}
	return out, nil
}

func specFromCore(cs core.CustomFormatSpec) (Specification, error) {
	kind, ok := specKinds[cs.Kind]
	if !ok {
		return Specification{}, unknown("condition kind", strconv.Quote(string(cs.Kind)))
	}
	s := Specification{Kind: kind, Name: cs.Name, Negate: cs.Negate, Required: cs.Required}
	var err error
	switch kind {
	case SpecReleaseTitle, SpecEdition, SpecReleaseGroup:
		s.Pattern = cs.Value
	case SpecLanguage:
		var l release.Language
		l, err = languageFromCore(cs.Value)
		s.Value, s.ExceptLanguage = int(l), cs.ExceptLanguage
	case SpecIndexerFlag:
		s.Flag = cs.Value
	case SpecSource:
		var v release.Source
		v, err = byName(sourceNames, "source", cs.Value)
		s.Value = int(v)
	case SpecResolution:
		if s.Value, err = strconv.Atoi(cs.Value); err != nil {
			err = fmt.Errorf("%w: resolution %q", ErrUnknownValue, cs.Value)
		}
	case SpecQualityModifier:
		var v release.Modifier
		v, err = byName(modifierNames, "quality modifier", cs.Value)
		s.Value = int(v)
	case SpecSize, SpecYear:
		s.Min, s.Max = cs.Min, cs.Max
	case SpecReleaseType:
		var v release.ReleaseType
		v, err = byName(releaseTypeNames, "release type", cs.Value)
		s.Value = int(v)
	}
	if err != nil {
		return Specification{}, err
	}
	return s, nil
}

// CustomFormatToCore returns the stored form of a custom format.
func CustomFormatToCore(f CustomFormat) (core.CustomFormat, error) {
	out := core.CustomFormat{ID: f.ID, Name: f.Name, IncludeWhenRenaming: f.IncludeWhenRenaming}
	for _, s := range f.Specifications {
		cs, err := specToCore(s)
		if err != nil {
			return core.CustomFormat{}, fmt.Errorf("custom format %q: condition %q: %w", f.Name, s.Name, err)
		}
		out.Specifications = append(out.Specifications, cs)
	}
	return out, nil
}

func specToCore(s Specification) (core.CustomFormatSpec, error) {
	cs := core.CustomFormatSpec{Name: s.Name, Negate: s.Negate, Required: s.Required}
	for k, v := range specKinds {
		if v == s.Kind {
			cs.Kind = k
		}
	}
	var err error
	switch s.Kind {
	case SpecReleaseTitle, SpecEdition, SpecReleaseGroup:
		cs.Value = s.Pattern
	case SpecLanguage:
		cs.Value, err = languageToCore(release.Language(s.Value))
		cs.ExceptLanguage = s.ExceptLanguage
	case SpecIndexerFlag:
		cs.Value = s.Flag
	case SpecSource:
		cs.Value, err = nameOf(sourceNames, "source", release.Source(s.Value))
	case SpecResolution:
		cs.Value = strconv.Itoa(s.Value)
	case SpecQualityModifier:
		cs.Value, err = nameOf(modifierNames, "quality modifier", release.Modifier(s.Value))
	case SpecSize, SpecYear:
		cs.Min, cs.Max = s.Min, s.Max
	case SpecReleaseType:
		cs.Value, err = nameOf(releaseTypeNames, "release type", release.ReleaseType(s.Value))
	default:
		err = unknown("condition kind", int(s.Kind))
	}
	if err != nil {
		return core.CustomFormatSpec{}, err
	}
	return cs, nil
}

// DelayProfileFromCore returns the delay profile of a stored one.
func DelayProfileFromCore(p core.DelayProfile) (DelayProfile, error) {
	preferred, err := ProtocolFromCore(p.PreferredProtocol)
	if err != nil {
		return DelayProfile{}, fmt.Errorf("delay profile %s: %w", p.ID, err)
	}
	return DelayProfile{
		ID:                             p.ID,
		Default:                        p.Default,
		EnableUsenet:                   p.EnableUsenet,
		EnableTorrent:                  p.EnableTorrent,
		PreferredProtocol:              preferred,
		UsenetDelay:                    p.UsenetDelay,
		TorrentDelay:                   p.TorrentDelay,
		Order:                          p.Order,
		BypassIfHighestQuality:         p.BypassIfHighestQuality,
		BypassIfAboveCustomFormatScore: p.BypassIfAboveCustomFormatScore,
		MinimumCustomFormatScore:       p.MinimumCustomFormatScore,
		Tags:                           slices.Clone(p.Tags),
	}, nil
}

// DelayProfileToCore returns the stored form of a delay profile.
func DelayProfileToCore(p DelayProfile) (core.DelayProfile, error) {
	preferred, err := ProtocolToCore(p.PreferredProtocol)
	if err != nil {
		return core.DelayProfile{}, fmt.Errorf("delay profile %s: %w", p.ID, err)
	}
	return core.DelayProfile{
		ID:                             p.ID,
		Default:                        p.Default,
		Order:                          p.Order,
		EnableUsenet:                   p.EnableUsenet,
		EnableTorrent:                  p.EnableTorrent,
		PreferredProtocol:              preferred,
		UsenetDelay:                    p.UsenetDelay,
		TorrentDelay:                   p.TorrentDelay,
		BypassIfHighestQuality:         p.BypassIfHighestQuality,
		BypassIfAboveCustomFormatScore: p.BypassIfAboveCustomFormatScore,
		MinimumCustomFormatScore:       p.MinimumCustomFormatScore,
		Tags:                           slices.Clone(p.Tags),
	}, nil
}

// ReleaseProfileFromCore returns the release profile of a stored one.
func ReleaseProfileFromCore(p core.ReleaseProfile) ReleaseProfile {
	return ReleaseProfile{
		ID:        p.ID,
		Name:      p.Name,
		Enabled:   p.Enabled,
		Required:  slices.Clone(p.Required),
		Ignored:   slices.Clone(p.Ignored),
		IndexerID: p.IndexerID,
		Tags:      slices.Clone(p.Tags),
	}
}

// ReleaseProfileToCore returns the stored form of a release profile.
func ReleaseProfileToCore(p ReleaseProfile) core.ReleaseProfile {
	return core.ReleaseProfile{
		ID:        p.ID,
		Name:      p.Name,
		Enabled:   p.Enabled,
		Required:  slices.Clone(p.Required),
		Ignored:   slices.Clone(p.Ignored),
		IndexerID: p.IndexerID,
		Tags:      slices.Clone(p.Tags),
	}
}

// DefinitionFromCore returns the definition of a stored one, its group
// name and weight those of the kind's default definition of the quality.
// A quality the kind does not have is an error.
func DefinitionFromCore(d core.QualityDefinition) (Definition, error) {
	kind, err := KindFromCore(d.Kind)
	if err != nil {
		return Definition{}, err
	}
	q, err := qualityFromCore(d.Quality)
	if err != nil {
		return Definition{}, err
	}
	i := slices.IndexFunc(DefaultDefinitions(kind), func(def Definition) bool { return def.Quality == q })
	if i < 0 {
		return Definition{}, fmt.Errorf("%w: quality %s of %s", ErrUnknownValue, q, d.Kind)
	}
	out := DefaultDefinitions(kind)[i]
	out.Title, out.MinSize, out.MaxSize, out.PreferredSize = d.Title, d.MinSize, d.MaxSize, d.PreferredSize
	return out, nil
}

// DefinitionsFromCore returns a kind's default definitions with the
// titles and sizes of the stored ones, which may be none.
func DefinitionsFromCore(kind core.ItemKind, defs []core.QualityDefinition) ([]Definition, error) {
	rk, err := KindFromCore(kind)
	if err != nil {
		return nil, err
	}
	out := DefaultDefinitions(rk)
	for _, d := range defs {
		if d.Kind != kind {
			return nil, fmt.Errorf("%w: definition of %s among those of %s", ErrUnknownValue, d.Kind, kind)
		}
		def, err := DefinitionFromCore(d)
		if err != nil {
			return nil, err
		}
		out[slices.IndexFunc(out, func(o Definition) bool { return o.Quality == def.Quality })] = def
	}
	return out, nil
}

// DefinitionToCore returns the stored form of a definition of a kind.
func DefinitionToCore(d Definition, kind core.ItemKind) (core.QualityDefinition, error) {
	id, err := qualityToCore(d.Quality)
	if err != nil {
		return core.QualityDefinition{}, err
	}
	return core.QualityDefinition{
		Kind: kind, Quality: id, Title: d.Title,
		MinSize: d.MinSize, MaxSize: d.MaxSize, PreferredSize: d.PreferredSize,
	}, nil
}

// ConfigFromCore returns the decision settings of the acquisition
// settings, in bytes and durations where core keeps megabytes and
// minutes.
func ConfigFromCore(s core.AcquisitionSettings) (Config, error) {
	propers, err := ProperDownloadFromCore(s.Propers)
	if err != nil {
		return Config{}, err
	}
	const mb = 1 << 20
	return Config{
		Propers:                   propers,
		PreferIndexerFlags:        s.PreferIndexerFlags,
		CompletedDownloadHandling: s.CompletedDownloadHandling,
		MaximumSize:               int64(s.MaximumSize) * mb,
		MinimumAge:                time.Duration(s.MinimumAge) * time.Minute,
		Retention:                 s.Retention,
		AllowHardcodedSubs:        s.AllowHardcodedSubs,
		WhitelistedHardcodedSubs:  slices.Clone(s.WhitelistedHardcodedSubs),
		SkipFreeSpaceCheck:        s.SkipFreeSpaceCheck,
		MinimumFreeSpace:          int64(s.MinimumFreeSpace) * mb,
		AvailabilityDelay:         s.AvailabilityDelay,
	}, nil
}
