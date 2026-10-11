package acquire

import (
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"
)

// The paths Radarr's DiskScanService.FilterPaths leaves out of imports:
// files in extras, sample and hidden folders, extras by their suffix and
// system files.
var (
	excludedExtrasSubFolderRegex = regexp.MustCompile(`(?i)(?:\\|/|^)(?:extras|extrafanart|behind the scenes|deleted scenes|featurettes|interviews|other|scenes|sample[s]?|shorts|trailers)(?:\\|/)`)
	excludedSubFoldersRegex      = regexp.MustCompile(`(?i)(?:\\|/|^)(?:@eadir|\.@__thumb|plex versions|\.[^\\/]+)(?:\\|/)`)
	excludedExtraFilesRegex      = regexp.MustCompile(`(?i)(-(trailer|other|behindthescenes|deleted|featurette|interview|scene|short)\.[^.]+$)`)
	excludedFilesRegex           = regexp.MustCompile(`(?i)^\.(_|unmanic|DS_Store$)|^Thumbs\.db$`)
)

// FilterImportPaths returns the paths, below base, that may be imported,
// as Radarr's DiskScanService.FilterPaths. Paths use "/" or "\".
func FilterImportPaths(base string, paths []string) []string {
	var out []string
	for _, p := range paths {
		rel := relativePath(base, p)
		name := fileName(p)
		if excludedSubFoldersRegex.MatchString(rel) || excludedExtrasSubFolderRegex.MatchString(rel) ||
			excludedExtraFilesRegex.MatchString(name) || excludedFilesRegex.MatchString(name) {
			continue
		}
		out = append(out, p)
	}
	return out
}

// relativePath returns p relative to base when p is below it, else p.
func relativePath(base, p string) string {
	b := strings.TrimRight(strings.ReplaceAll(base, `\`, "/"), "/")
	q := strings.ReplaceAll(p, `\`, "/")
	if b != "" && strings.HasPrefix(strings.ToLower(q), strings.ToLower(b)+"/") {
		return q[len(b)+1:]
	}
	return q
}

// fileName returns the last element of a path using "/" or "\".
func fileName(p string) string {
	return path.Base(strings.ReplaceAll(p, `\`, "/"))
}

// SampleResult is whether a file is a sample, as Radarr's
// DetectSampleResult.
type SampleResult int

// The sample results.
const (
	SampleNo SampleResult = iota
	SampleYes
	// SampleIndeterminate is for files whose runtime is unknown.
	SampleIndeterminate
)

// IsSample decides whether a video file is a sample by its runtime, as
// Radarr's DetectSample: disc images, .flv and .strm files never are; a
// file that is shorter than the least runtime for the movie's runtime,
// in minutes, is. runtime is nil when it could not be read.
func IsSample(file string, runtime *time.Duration, movieRuntime int) SampleResult {
	switch strings.ToLower(path.Ext(fileName(file))) {
	case ".flv", ".strm", ".iso", ".img", ".m2ts":
		return SampleNo
	}
	if runtime == nil {
		return SampleIndeterminate
	}
	if *runtime == 0 {
		return SampleYes
	}
	if runtime.Seconds() < float64(minimumRuntime(movieRuntime)) {
		return SampleYes
	}
	return SampleNo
}

// minimumRuntime is the least runtime, in seconds, of a file that is not
// a sample of a movie of a runtime in minutes.
func minimumRuntime(minutes int) int {
	switch {
	case minutes <= 3:
		return 15
	case minutes <= 10:
		return 90
	case minutes <= 30:
		return 300
	}
	return 600
}

// ImportRejection is why a file is not imported over the existing one,
// as Radarr's ImportRejectionReason.
type ImportRejection string

// The import rejections.
const (
	ImportNotQualityUpgrade      ImportRejection = "notQualityUpgrade"
	ImportNotRevisionUpgrade     ImportRejection = "notRevisionUpgrade"
	ImportNotCustomFormatUpgrade ImportRejection = "notCustomFormatUpgrade"
)

// ImportUpgrade decides whether a downloaded file of quality q and
// custom formats replaces the existing file, nil when there is none, as
// the UpgradeSpecification of Radarr's imports. It returns why not and a
// message, or "".
func (u Upgrades) ImportUpgrade(p *QualityProfile, existing *File, q QualityModel, formats []CustomFormat) (ImportRejection, string) {
	if existing == nil {
		return "", ""
	}
	qualityCompare := p.CompareQualities(q.Quality, existing.Quality.Quality, false)
	if qualityCompare < 0 {
		return ImportNotQualityUpgrade, fmt.Sprintf("Not an upgrade for existing movie file. Existing quality: %s. New Quality %s.", existing.Quality.Quality, q.Quality)
	}
	if qualityCompare == 0 && u.Propers != PropersDoNotPrefer && q.Revision.Compare(existing.Quality.Revision) < 0 {
		return ImportNotRevisionUpgrade, "Not a quality revision upgrade for existing movie file(s)"
	}
	currentScore, newScore := p.FormatScore(existing.Formats), p.FormatScore(formats)
	if qualityCompare == 0 && newScore < currentScore {
		return ImportNotCustomFormatUpgrade, fmt.Sprintf("Not a Custom Format upgrade for existing movie file(s). New: [%s] (%d) do not improve on Existing: [%s] (%d)",
			formatNames(formats), newScore, formatNames(existing.Formats), currentScore)
	}
	return "", ""
}

// Recent reports whether a movie came out lately at now, as Radarr's
// Movie.IsRecentMovie: on disc or digitally in the last 21 days, or in
// cinemas in the last 120.
func (a *Availability) Recent(now time.Time) bool {
	switch {
	case !a.Physical.IsZero() && !a.Physical.Before(now.AddDate(0, 0, -21)):
		return true
	case !a.Digital.IsZero() && !a.Digital.Before(now.AddDate(0, 0, -21)):
		return true
	}
	return !a.InCinemas.IsZero() && !a.InCinemas.Before(now.AddDate(0, 0, -120))
}

// RemotePathMapping maps the folders a download client on a host sees to
// the server's.
type RemotePathMapping struct {
	Host, RemotePath, LocalPath string
}

// RemapRemoteToLocal returns the server's path of a path a download
// client on host reports, by the first mapping of the host whose remote
// path holds it, as Radarr's RemotePathMappingService; paths no mapping
// holds are returned as they are. Hosts compare ignoring case, as do
// Windows paths.
func RemapRemoteToLocal(mappings []RemotePathMapping, host, remote string) string {
	if remote == "" {
		return remote
	}
	for _, m := range mappings {
		if !strings.EqualFold(m.Host, host) {
			continue
		}
		rel, ok := below(m.RemotePath, remote)
		if !ok {
			continue
		}
		sep := "/"
		if strings.Contains(m.LocalPath, `\`) && !strings.Contains(m.LocalPath, "/") {
			sep = `\`
		}
		local := strings.TrimRight(m.LocalPath, `/\`)
		if rel == "" {
			return local + sep
		}
		return local + sep + strings.Join(strings.FieldsFunc(rel, isSep), sep)
	}
	return remote
}

func isSep(r rune) bool { return r == '/' || r == '\\' }

// below returns the path of p relative to parent, ok when p is parent or
// below it, as OsPath.Contains. Windows paths, those with a drive letter
// or of a share, compare ignoring case.
func below(parent, p string) (string, bool) {
	windows := isWindowsPath(parent) || isWindowsPath(p)
	norm := func(s string) []string {
		parts := strings.FieldsFunc(s, isSep)
		if windows {
			for i := range parts {
				parts[i] = strings.ToLower(parts[i])
			}
		}
		return parts
	}
	pp, cp := norm(parent), norm(p)
	if len(pp) == 0 || len(cp) < len(pp) {
		return "", false
	}
	for i := range pp {
		if pp[i] != cp[i] {
			return "", false
		}
	}
	return strings.Join(strings.FieldsFunc(p, isSep)[len(pp):], "/"), true
}

func isWindowsPath(p string) bool {
	return strings.HasPrefix(p, `\\`) || len(p) >= 2 && p[1] == ':'
}
