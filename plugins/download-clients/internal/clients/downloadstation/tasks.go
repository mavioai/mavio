package downloadstation

import (
	"bytes"
	"context"
	"crypto/sha1" //nolint:gosec // Radarr's download IDs hash the serial number with SHA-1.
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// taskStatus is the status of a task, as Radarr's
// DownloadStationTaskStatus: task API v1 names it in snake case, v2
// numbers it.
type taskStatus int

// The statuses.
const (
	statusUnknown taskStatus = iota
	statusWaiting
	statusDownloading
	statusPaused
	statusFinishing
	statusFinished
	statusHashChecking
	statusSeeding
	statusFilehostingWaiting
	statusExtracting
	statusError
	statusCaptchaNeeded
)

var statusNames = []string{
	"unknown", "waiting", "downloading", "paused", "finishing", "finished", "hash_checking", "seeding",
	"filehosting_waiting", "extracting", "error", "captcha_needed",
}

func (s taskStatus) String() string {
	if s < 0 || int(s) >= len(statusNames) {
		return strconv.Itoa(int(s))
	}
	return statusNames[s]
}

// UnmarshalJSON reads a status by name, ignoring underscores and case, or
// by number; anything else is unknown, as Radarr's
// UnderscoreStringEnumConverter.
func (s *taskStatus) UnmarshalJSON(b []byte) error {
	*s = statusUnknown
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return nil //nolint:nilerr // An unreadable status is unknown.
	}
	var str string
	switch v := v.(type) {
	case float64:
		str = strconv.FormatFloat(v, 'f', -1, 64)
	case string:
		str = v
	default:
		return nil
	}
	if n, err := strconv.Atoi(str); err == nil {
		if n >= 0 && n < len(statusNames) {
			*s = taskStatus(n)
		}
		return nil
	}
	key := strings.ReplaceAll(str, "_", "")
	for i, name := range statusNames {
		if strings.EqualFold(strings.ReplaceAll(name, "_", ""), key) {
			*s = taskStatus(i)
			return nil
		}
	}
	return nil
}

// MarshalJSON writes a status by its snake-case name.
func (s taskStatus) MarshalJSON() ([]byte, error) { return json.Marshal(s.String()) }

// stringMap is a map of strings that DSM may send as numbers, as Radarr
// reads the additional details and transfer of tasks.
type stringMap map[string]string

// UnmarshalJSON reads an object's values as strings: numbers and booleans
// as written, null as empty.
func (m *stringMap) UnmarshalJSON(b []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	out := make(stringMap, len(raw))
	for k, v := range raw {
		var s string
		switch {
		case json.Unmarshal(v, &s) == nil:
		case bytes.Equal(v, []byte("null")):
		default:
			s = string(v)
		}
		out[k] = s
	}
	*m = out
	return nil
}

// task is a task of Download Station, as Radarr's DownloadStationTask.
type task struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Size     int64  `json:"size"`
	Username string `json:"username"`
	// Type is one of BT, NZB, http, ftp, eMule and https.
	Type        string     `json:"type"`
	StatusExtra stringMap  `json:"status_extra,omitempty"`
	Status      taskStatus `json:"status"`
	Additional  struct {
		Detail   stringMap `json:"detail,omitempty"`
		Transfer stringMap `json:"transfer,omitempty"`
	} `json:"additional"`
}

func (t *task) String() string { return t.Title }

// taskAPI is a version of DownloadStation's task API, as Radarr's
// IDownloadStationTaskProxy.
type taskAPI interface {
	// info is the API's info.
	info(ctx context.Context) (*apiInfo, error)
	tasks(ctx context.Context) ([]task, error)
	remove(ctx context.Context, id string) error
	addURL(ctx context.Context, link, dir string) error
	addData(ctx context.Context, data []byte, name, dir string) error
}

// swallow returns no tasks for an API failure, as Radarr's task proxies
// do; failures to connect or sign in are returned.
func swallow(ctx context.Context, d *dsm, tasks []task, err error) ([]task, error) {
	var apiErr *apiError
	if errors.As(err, &apiErr) {
		d.log.ErrorContext(ctx, "get Download Station tasks", "err", err)
		return nil, nil
	}
	return tasks, err
}

// taskV1 is SYNO.DownloadStation.Task, as Radarr's
// DownloadStationTaskProxyV1.
type taskV1 struct{ d *dsm }

func (t taskV1) info(ctx context.Context) (*apiInfo, error) { return t.d.info(ctx, synoTask) }

func (t taskV1) tasks(ctx context.Context) ([]task, error) {
	var data struct {
		Tasks []task `json:"tasks"`
	}
	err := t.d.call(ctx, call{
		api: synoTask, method: "list", version: 1, op: "get tasks",
		params: []param{{"additional", "detail,transfer"}},
	}, &data)
	return swallow(ctx, t.d, data.Tasks, err)
}

func (t taskV1) remove(ctx context.Context, id string) error {
	return t.d.call(ctx, call{
		api: synoTask, method: "delete", version: 1, op: "remove item " + id,
		params: []param{{"id", id}, {"force_complete", "false"}},
	}, nil)
}

func (t taskV1) addURL(ctx context.Context, link, dir string) error {
	params := []param{{"uri", link}}
	if strings.TrimSpace(dir) != "" {
		params = append(params, param{"destination", dir})
	}
	return t.d.call(ctx, call{api: synoTask, method: "create", version: 3, op: "add task from url " + link, params: params}, nil)
}

func (t taskV1) addData(ctx context.Context, data []byte, name, dir string) error {
	var params []param
	if strings.TrimSpace(dir) != "" {
		params = append(params, param{"destination", dir})
	}
	return t.d.call(ctx, call{
		api: synoTask, method: "create", version: 2, op: "add task from data " + name, params: params,
		file: &upload{field: "file", name: name, data: data},
	}, nil)
}

// taskV2 is SYNO.DownloadStation2.Task, as Radarr's
// DownloadStationTaskProxyV2.
type taskV2 struct{ d *dsm }

func (t taskV2) info(ctx context.Context) (*apiInfo, error) { return t.d.info(ctx, synoTask2) }

// tasks lists the tasks with their details, then with their transfer, as
// v2 cannot give both at once.
func (t taskV2) tasks(ctx context.Context) ([]task, error) {
	list := func(additional, op string) ([]task, int, error) {
		var data struct {
			Task  []task `json:"task"`
			Total int    `json:"total"`
		}
		err := t.d.call(ctx, call{
			api: synoTask2, method: "list", version: 1, op: op,
			params: []param{{"additional", additional}},
		}, &data)
		return data.Task, data.Total, err
	}
	tasks, total, err := list("detail", "get tasks with additional detail")
	if err != nil || total == 0 {
		return swallow(ctx, t.d, nil, err)
	}
	transfers, _, err := list("transfer", "get tasks with additional transfer")
	if err != nil {
		return swallow(ctx, t.d, nil, err)
	}
	for i := range tasks {
		for _, tr := range transfers {
			if tr.ID == tasks[i].ID {
				tasks[i].Additional.Transfer = tr.Additional.Transfer
				break
			}
		}
	}
	return tasks, nil
}

func (t taskV2) remove(ctx context.Context, id string) error {
	return t.d.call(ctx, call{
		api: synoTask2, method: "delete", version: 2, op: "remove item " + id,
		params: []param{{"id", id}, {"force_complete", "false"}},
	}, nil)
}

// defaultDestination is Download Station's default folder, which v2 needs
// for tasks without one, as Radarr's DownloadStation2SettingsLocationProxy.
func (t taskV2) defaultDestination(ctx context.Context) (string, error) {
	info, err := t.d.require(ctx, synoLocation)
	if err != nil {
		return "", err
	}
	var data struct {
		DefaultDestination string `json:"default_destination"`
	}
	err = t.d.do(ctx, info, call{api: synoLocation, method: "get", version: info.MinVersion, op: "get default destination folder"}, &data)
	return data.DefaultDestination, err
}

// destination returns the destination parameter of a task: its folder,
// else the default one quoted, as Radarr sends them.
func (t taskV2) destination(ctx context.Context, dir string, quote bool) ([]param, error) {
	if strings.TrimSpace(dir) != "" {
		if quote {
			dir = `"` + dir + `"`
		}
		return []param{{"destination", dir}}, nil
	}
	def, err := t.defaultDestination(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(def) == "" {
		t.d.log.ErrorContext(ctx, "unable to get the default destination folder from Download Station")
		return nil, nil
	}
	return []param{{"destination", `"` + def + `"`}}, nil
}

func (t taskV2) addURL(ctx context.Context, link, dir string) error {
	dest, err := t.destination(ctx, dir, false)
	if err != nil {
		return err
	}
	params := append([]param{{"type", "url"}, {"url", link}, {"create_list", "false"}}, dest...)
	return t.d.call(ctx, call{api: synoTask2, method: "create", version: 2, op: "add task from url " + link, params: params}, nil)
}

func (t taskV2) addData(ctx context.Context, data []byte, name, dir string) error {
	dest, err := t.destination(ctx, dir, true)
	if err != nil {
		return err
	}
	params := append([]param{{"type", `"file"`}, {"file", `["fileData"]`}, {"create_list", "false"}}, dest...)
	return t.d.call(ctx, call{
		api: synoTask2, method: "create", version: 2, op: "add task from data " + name, params: params,
		file: &upload{field: "fileData", name: name, data: data},
	}, nil)
}

// fileInfo is a file or folder of FileStation.
type fileInfo struct {
	IsDir      bool           `json:"isdir"`
	Name       string         `json:"name"`
	Path       string         `json:"path"`
	Additional map[string]any `json:"additional"`
}

// fileInfo returns the info of a file or folder, as Radarr's
// FileStationProxy.GetInfoFileOrDirectory.
func (d *dsm) fileInfo(ctx context.Context, path string) (*fileInfo, error) {
	paths, err := json.Marshal([]string{path})
	if err != nil {
		return nil, err
	}
	var data struct {
		Files []fileInfo `json:"files"`
	}
	err = d.call(ctx, call{
		api: synoFileList, method: "getinfo", version: 2, op: "get info of " + path,
		params: []param{{"path", string(paths)}, {"additional", `["real_path"]`}},
	}, &data)
	if err != nil {
		return nil, err
	}
	if len(data.Files) == 0 {
		return nil, fmt.Errorf("no info of %s", path)
	}
	return &data.Files[0], nil
}

// defaultDir returns Download Station's default destination, as Radarr's
// GetDefaultDir.
func (d *dsm) defaultDir(ctx context.Context) (string, error) {
	var data map[string]any
	if err := d.call(ctx, call{api: synoDSInfo, method: "getConfig", version: 1, op: "get config"}, &data); err != nil {
		return "", err
	}
	dir, _ := data["default_destination"].(string)
	return dir, nil
}

// serialNumber returns the Diskstation's serial number, as Radarr's
// DSMInfoProxy.
func (d *dsm) serialNumber(ctx context.Context) (string, error) {
	info, err := d.require(ctx, synoDSMInfo)
	if err != nil {
		return "", err
	}
	var data struct {
		Serial string `json:"serial"`
	}
	err = d.do(ctx, info, call{api: synoDSMInfo, method: "getinfo", version: info.MinVersion, op: "get serial number"}, &data)
	return data.Serial, err
}

// hashSerial hashes a serial number as Radarr's HashConverter, in
// upper-case hex.
func hashSerial(serial string) string {
	sum := sha1.Sum([]byte(serial)) //nolint:gosec // Not for security; Radarr's IDs.
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

// remap returns the full path of a path in a shared folder, from the
// shared folder's physical path, as Radarr's OsPath arithmetic.
func remap(path, share, physical string) string {
	rel := strings.Trim(strings.TrimPrefix(path, share), "/")
	if rel == "" {
		return physical
	}
	return joinPath(physical, rel)
}

func joinPath(dir, name string) string {
	return strings.TrimRight(dir, "/") + "/" + name
}
