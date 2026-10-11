package acquire

// ProperDownload is what to do with propers and repacks, as Radarr's
// ProperDownloadTypes.
type ProperDownload int

// The proper download settings.
const (
	// PropersPreferAndUpgrade prefers propers and repacks and upgrades
	// to them.
	PropersPreferAndUpgrade ProperDownload = iota
	// PropersDoNotUpgrade prefers them but does not upgrade to them.
	PropersDoNotUpgrade
	// PropersDoNotPrefer leaves revisions to custom formats.
	PropersDoNotPrefer
)

// UpgradeRejection is why a release is not an upgrade, as Radarr's
// UpgradeableRejectReason.
type UpgradeRejection int

// The upgrade rejections.
const (
	UpgradeAccepted UpgradeRejection = iota
	UpgradeBetterQuality
	UpgradeBetterRevision
	UpgradeQualityCutoff
	UpgradeCustomFormatScore
	UpgradeCustomFormatCutoff
	UpgradeMinCustomFormatScore
	UpgradeNotAllowed
)

// Upgrades decides whether a release upgrades what is already there, as
// Radarr's UpgradableSpecification.
type Upgrades struct {
	Propers ProperDownload
}

// IsUpgradable returns why newQuality with newFormats is not an upgrade
// of current with currentFormats under p, or UpgradeAccepted.
func (u Upgrades) IsUpgradable(p *QualityProfile, current QualityModel, currentFormats []CustomFormat, newQuality QualityModel, newFormats []CustomFormat) UpgradeRejection {
	qualityCompare := p.CompareQualities(newQuality.Quality, current.Quality, false)
	if qualityCompare > 0 && u.QualityCutoffNotMet(p, current, &newQuality) {
		return UpgradeAccepted
	}
	if qualityCompare < 0 {
		return UpgradeBetterQuality
	}
	revisionCompare := newQuality.Revision.Compare(current.Revision)
	if u.Propers != PropersDoNotPrefer && revisionCompare > 0 {
		return UpgradeAccepted
	}
	if !p.UpgradeAllowed {
		return UpgradeNotAllowed
	}
	if u.Propers != PropersDoNotPrefer && revisionCompare < 0 {
		return UpgradeBetterRevision
	}
	if qualityCompare > 0 {
		return UpgradeQualityCutoff
	}
	currentScore := p.FormatScore(currentFormats)
	newScore := p.FormatScore(newFormats)
	if newScore <= currentScore {
		return UpgradeCustomFormatScore
	}
	if currentScore >= p.CutoffFormatScore {
		return UpgradeCustomFormatCutoff
	}
	if newScore < currentScore+p.MinUpgradeFormatScore {
		return UpgradeMinCustomFormatScore
	}
	return UpgradeAccepted
}

// QualityCutoffNotMet reports whether current is below the profile's
// cutoff, or newQuality, when given, is a revision upgrade of it.
func (u Upgrades) QualityCutoffNotMet(p *QualityProfile, current QualityModel, newQuality *QualityModel) bool {
	if p.Index(current.Quality, false).Compare(p.IndexOf(p.effectiveCutoff(), false), false) < 0 {
		return true
	}
	return newQuality != nil && IsRevisionUpgrade(current, *newQuality)
}

// effectiveCutoff is the ID of the quality or group that meets the
// cutoff: the first allowed quality when upgrades are not allowed.
func (p *QualityProfile) effectiveCutoff() int {
	if p.UpgradeAllowed {
		return p.Cutoff
	}
	return QualityID(p.FirstAllowedQuality())
}

// cutoffItem returns the profile item the cutoff ID is in.
func (p *QualityProfile) cutoffItem(id int) ProfileItem {
	i := p.IndexOf(id, false).Index
	if i < 0 || i >= len(p.Items) {
		return ProfileItem{}
	}
	return p.Items[i]
}

// CutoffNotMet reports whether current with currentFormats is below the
// profile's quality or custom format cutoff.
func (u Upgrades) CutoffNotMet(p *QualityProfile, current QualityModel, currentFormats []CustomFormat, newQuality *QualityModel) bool {
	if u.QualityCutoffNotMet(p, current, newQuality) {
		return true
	}
	cutoff := p.MinFormatScore
	if p.UpgradeAllowed {
		cutoff = p.CutoffFormatScore
	}
	return p.FormatScore(currentFormats) < cutoff
}

// IsRevisionUpgrade reports whether newQuality is the same quality as
// current in a later revision. A proper of a WEBRip does not upgrade a
// WEBDL.
func IsRevisionUpgrade(current, newQuality QualityModel) bool {
	return current.Quality == newQuality.Quality && newQuality.Revision.Compare(current.Revision) > 0
}

// IsUpgradeAllowed reports whether the profile allows replacing current
// with newQuality: revision upgrades always are, other upgrades only
// when the profile allows upgrades.
func (u Upgrades) IsUpgradeAllowed(p *QualityProfile, current QualityModel, currentFormats []CustomFormat, newQuality QualityModel, newFormats []CustomFormat) bool {
	if IsRevisionUpgrade(current, newQuality) {
		return true
	}
	isQualityUpgrade := p.Compare(newQuality, current, false) > 0
	isFormatUpgrade := p.FormatScore(newFormats) > p.FormatScore(currentFormats)
	if isQualityUpgrade || isFormatUpgrade {
		return p.UpgradeAllowed
	}
	return true
}
