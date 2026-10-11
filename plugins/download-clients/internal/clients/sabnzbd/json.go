package sabnzbd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// The JSON types of SABnzbd's answers, which are loose about types: as
// Newtonsoft.Json reads them for Radarr, numbers and bools may come as
// strings, and with the converters of Radarr's Sabnzbd.

// isNull reports whether data is JSON's null.
func isNull(data []byte) bool { return string(bytes.TrimSpace(data)) == "null" }

// unquote returns a JSON string's value, or a scalar's text.
func unquote(data []byte) string {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		return s
	}
	return string(bytes.TrimSpace(data))
}

// flexBool is a bool from true, false, 1, 0 or those as strings.
type flexBool bool

// UnmarshalJSON implements json.Unmarshaler.
func (b *flexBool) UnmarshalJSON(data []byte) error {
	if isNull(data) {
		return nil
	}
	s := strings.TrimSpace(unquote(data))
	if s == "" {
		*b = false
		return nil
	}
	if v, err := strconv.ParseBool(s); err == nil {
		*b = flexBool(v)
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("sabnzbd: %q is not a bool", s)
	}
	*b = f != 0
	return nil
}

// flexFloat is a number, which may come as a string.
type flexFloat float64

// UnmarshalJSON implements json.Unmarshaler.
func (f *flexFloat) UnmarshalJSON(data []byte) error {
	if isNull(data) {
		return nil
	}
	s := strings.TrimSpace(unquote(data))
	if s == "" {
		*f = 0
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("sabnzbd: %q is not a number", s)
	}
	*f = flexFloat(v)
	return nil
}

// flexInt is an integer, which may come as a string.
type flexInt int64

// UnmarshalJSON implements json.Unmarshaler.
func (n *flexInt) UnmarshalJSON(data []byte) error {
	var f flexFloat
	if err := f.UnmarshalJSON(data); err != nil {
		return err
	}
	*n = flexInt(f)
	return nil
}

// flexString is a string, which may come as a number or bool.
type flexString string

// UnmarshalJSON implements json.Unmarshaler.
func (s *flexString) UnmarshalJSON(data []byte) error {
	if isNull(data) {
		return nil
	}
	*s = flexString(unquote(data))
	return nil
}

// timeLeft is the time left of a queued download, as Radarr's
// SabnzbdQueueTimeConverter reads "H:M:S" or "D:H:M:S".
type timeLeft time.Duration

// UnmarshalJSON implements json.Unmarshaler.
func (t *timeLeft) UnmarshalJSON(data []byte) error {
	if isNull(data) {
		return nil
	}
	d, err := parseTimeLeft(unquote(data))
	if err != nil {
		return err
	}
	*t = timeLeft(d)
	return nil
}

func parseTimeLeft(s string) (time.Duration, error) {
	parts := strings.Split(s, ":")
	n := make([]int64, len(parts))
	for i, p := range parts {
		v, err := strconv.ParseInt(strings.TrimSpace(p), 10, 32)
		if err != nil {
			return 0, fmt.Errorf("sabnzbd: time left %q: %w", s, err)
		}
		n[i] = v
	}
	h := func(v int64) time.Duration { return time.Duration(v) * time.Hour }
	m := func(v int64) time.Duration { return time.Duration(v) * time.Minute }
	sec := func(v int64) time.Duration { return time.Duration(v) * time.Second }
	switch len(n) {
	case 4:
		return h(n[0]*24+n[1]) + m(n[2]) + sec(n[3]), nil
	case 3:
		return h(n[0]) + m(n[1]) + sec(n[2]), nil
	}
	return 0, fmt.Errorf("sabnzbd: expected either 0:0:0:0 or 0:0:0 format, but received: %s", s)
}

// priority is SABnzbd's priority of a download.
type priority int

// The priorities, as Radarr's SabnzbdPriority.
const (
	priorityDefault priority = -100
	priorityPaused  priority = -2
	priorityLow     priority = -1
	priorityNormal  priority = 0
	priorityHigh    priority = 1
	priorityForce   priority = 2
)

var priorityNames = map[string]priority{
	"Default": priorityDefault, "Paused": priorityPaused, "Low": priorityLow,
	"Normal": priorityNormal, "High": priorityHigh, "Force": priorityForce,
}

// UnmarshalJSON implements json.Unmarshaler as Radarr's
// SabnzbdPriorityTypeConverter: a name or number, else Normal.
func (p *priority) UnmarshalJSON(data []byte) error {
	*p = parsePriority(unquote(data))
	return nil
}

// parsePriority reads a priority as .NET's Enum.TryParse: by its exact
// name or as a number; zero (Normal) otherwise.
func parsePriority(s string) priority {
	s = strings.TrimSpace(s)
	if v, ok := priorityNames[s]; ok {
		return v
	}
	if v, err := strconv.Atoi(s); err == nil {
		return priority(v)
	}
	return priorityNormal
}

// stringArray is a list of strings, which SABnzbd sends as a plain string
// when it has one, as Radarr's SabnzbdStringArrayConverter reads it: a
// string or null is a list of it.
type stringArray []string

// UnmarshalJSON implements json.Unmarshaler.
func (a *stringArray) UnmarshalJSON(data []byte) error {
	switch d := bytes.TrimSpace(data); {
	case isNull(d):
		// A list of null, which holds no category: categories are never
		// empty, an empty one being "Default".
		*a = stringArray{""}
	case len(d) > 0 && d[0] == '"':
		var s string
		if err := json.Unmarshal(d, &s); err != nil {
			return err
		}
		*a = stringArray{s}
	case len(d) > 0 && d[0] == '[':
		var l []string
		if err := json.Unmarshal(d, &l); err != nil {
			return err
		}
		*a = l
	default:
		return fmt.Errorf("sabnzbd: expected array, got %s", d)
	}
	return nil
}

// downloadStatus is the status of a download, as Radarr's
// SabnzbdDownloadStatus.
type downloadStatus int

// The statuses; unknown is one SABnzbd added since.
const (
	statusUnknown downloadStatus = iota - 1
	statusGrabbing
	statusQueued
	statusPaused
	statusChecking
	statusDownloading
	statusQuickCheck
	statusVerifying
	statusRepairing
	statusFetching
	statusExtracting
	statusMoving
	statusRunning
	statusCompleted
	statusFailed
	statusDeleted
	statusPropagating
)

var statusNames = []string{
	"Grabbing", "Queued", "Paused", "Checking", "Downloading", "QuickCheck", "Verifying", "Repairing",
	"Fetching", "Extracting", "Moving", "Running", "Completed", "Failed", "Deleted", "Propagating",
}

// UnmarshalJSON implements json.Unmarshaler: a name, in any case, or a
// number.
func (s *downloadStatus) UnmarshalJSON(data []byte) error {
	v := strings.TrimSpace(unquote(data))
	for i, name := range statusNames {
		if strings.EqualFold(v, name) {
			*s = downloadStatus(i)
			return nil
		}
	}
	if n, err := strconv.Atoi(v); err == nil && n >= 0 && n < len(statusNames) {
		*s = downloadStatus(n)
		return nil
	}
	*s = statusUnknown
	return nil
}

// queueItem is a slot of mode=queue.
type queueItem struct {
	Status   downloadStatus `json:"status"`
	TimeLeft timeLeft       `json:"timeleft"`
	Size     flexFloat      `json:"mb"`
	SizeLeft flexFloat      `json:"mbleft"`
	Title    string         `json:"filename"`
	Priority priority       `json:"priority"`
	Category string         `json:"cat"`
	ID       string         `json:"nzo_id"`
}

// queue is the answer of mode=queue.
type queue struct {
	// DefaultRootFolder is SABnzbd's home before 2.0, whence complete_dir
	// is relative.
	DefaultRootFolder string      `json:"my_home"`
	Paused            flexBool    `json:"paused"`
	Items             []queueItem `json:"slots"`
}

// historyItem is a slot of mode=history.
type historyItem struct {
	FailMessage string         `json:"fail_message"`
	Size        flexInt        `json:"bytes"`
	Category    string         `json:"category"`
	Storage     string         `json:"storage"`
	Status      downloadStatus `json:"status"`
	ID          string         `json:"nzo_id"`
	Title       string         `json:"name"`
}

// history is the answer of mode=history.
type history struct {
	Items []historyItem `json:"slots"`
}

// category is a category of mode=get_config.
type category struct {
	Name string `json:"name"`
	Dir  string `json:"dir"`

	// FullPath is the category's folder, complete_dir and dir combined.
	FullPath string `json:"-"`
}

// sorter is a sorter of SABnzbd 4.1 and later.
type sorter struct {
	SortCats []string `json:"sort_cats"`
	IsActive flexBool `json:"is_active"`
}

// misc are the general settings of mode=get_config.
type misc struct {
	CompleteDir            string      `json:"complete_dir"`
	TVCategories           []string    `json:"tv_categories"`
	EnableTVSorting        flexBool    `json:"enable_tv_sorting"`
	MovieCategories        []string    `json:"movie_categories"`
	EnableMovieSorting     flexBool    `json:"enable_movie_sorting"`
	DateCategories         stringArray `json:"date_categories"`
	EnableDateSorting      flexBool    `json:"enable_date_sorting"`
	PreCheck               flexBool    `json:"pre_check"`
	HistoryRetention       flexString  `json:"history_retention"`
	HistoryRetentionOption flexString  `json:"history_retention_option"`
	HistoryRetentionNumber flexInt     `json:"history_retention_number"`
}

// config is SABnzbd's configuration.
type config struct {
	Misc       misc       `json:"misc"`
	Categories []category `json:"categories"`
	Sorters    []sorter   `json:"sorters"`
}
