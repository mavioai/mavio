package downloadstation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

// api is a DSM web API, as Radarr's DiskStationApi.
type api string

// The APIs used.
const (
	synoInfo     api = "SYNO.API.Info"
	synoAuth     api = "SYNO.API.Auth"
	synoDSInfo   api = "SYNO.DownloadStation.Info"
	synoTask     api = "SYNO.DownloadStation.Task"
	synoTask2    api = "SYNO.DownloadStation2.Task"
	synoLocation api = "SYNO.DownloadStation2.Settings.Location"
	synoFileList api = "SYNO.FileStation.List"
	synoDSMInfo  api = "SYNO.DSM.Info"
)

// How long APIs' info and sessions are kept.
const (
	infoCacheTime = time.Hour
	sessionTime   = 6 * time.Hour
)

// The error codes of sessions.
const (
	errCodePermission  = 105
	errCodeTimeout     = 106
	errCodeInterrupted = 107
	errCodeNoSID       = 119
)

// apiInfo is where an API is and its versions, as SYNO.API.Info tells.
type apiInfo struct {
	Path       string `json:"path"`
	MinVersion int    `json:"minVersion"`
	MaxVersion int    `json:"maxVersion"`
}

// infoAPI is SYNO.API.Info itself, which is always at query.cgi.
var infoAPI = apiInfo{Path: "query.cgi", MinVersion: 1, MaxVersion: 1}

func needsAuth(a api) bool { return a != synoInfo && a != synoAuth }

// apiError is the DSM answering a call with a failure.
type apiError struct {
	Code int
	msg  string
}

func (e *apiError) Error() string { return e.msg }

// The messages of the DSM's error codes, as Radarr's DiskStationError.
var (
	commonMessages = map[int]string{
		100: "Unknown error",
		101: "Invalid parameter",
		102: "The requested API does not exist",
		103: "The requested method does not exist",
		104: "The requested version does not support the functionality",
		105: "The logged in session does not have permission",
		106: "Session timeout",
		107: "Session interrupted by duplicate login",
		119: "SID not found",
	}
	authMessages = map[int]string{
		400: "No such account or incorrect password",
		401: "Disabled account",
		402: "Denied permission",
		403: "2-step authentication code required",
		404: "Failed to authenticate 2-step authentication code",
		406: "Enforce to authenticate with 2-factor authentication code",
		407: "Blocked IP source",
		408: "Expired password cannot change",
		409: "Expired password",
		410: "Password must be changed",
	}
	taskMessages = map[int]string{
		400: "File upload failed",
		401: "Max number of tasks reached",
		402: "Destination denied",
		403: "Destination does not exist",
		404: "Invalid task id",
		405: "Invalid task action",
		406: "No default destination",
		407: "Set destination failed",
		408: "File does not exist",
	}
	fileStationMessages = map[int]string{
		160: "Permission denied. Give your user access to FileStation.",
		400: "Invalid parameter of file operation",
		401: "Unknown error of file operation",
		402: "System is too busy",
		403: "Invalid user does this file operation",
		404: "Invalid group does this file operation",
		405: "Invalid user and group does this file operation",
		406: "Can’t get user/group information from the account server",
		407: "Operation not permitted",
		408: "No such file or directory",
		409: "Non-supported file system",
		410: "Failed to connect internet-based file system (ex: CIFS)",
		411: "Read-only file system",
		412: "Filename too long in the non-encrypted file system",
		413: "Filename too long in the encrypted file system",
		414: "File already exists",
		415: "Disk quota exceeded",
		416: "No space left on device",
		417: "Input/output error",
		418: "Illegal name or path",
		419: "Illegal file name",
		420: "Illegal file name on FAT file system",
		421: "Device or resource busy",
		599: "No such task of the file operation",
	}
)

// errorMessage returns the message of an API's error code.
func errorMessage(a api, code int) string {
	var specific map[int]string
	switch a {
	case synoAuth:
		specific = authMessages
	case synoTask, synoTask2:
		specific = taskMessages
	case synoFileList:
		specific = fileStationMessages
	}
	if m, ok := specific[code]; ok {
		return m
	}
	if m, ok := commonMessages[code]; ok {
		return m
	}
	return fmt.Sprintf("%d - Unknown error", code)
}

func sessionError(code int) bool {
	return code == errCodePermission || code == errCodeTimeout || code == errCodeInterrupted || code == errCodeNoSID
}

// param is a query or form parameter; DSM wants form fields in order.
type param struct{ key, value string }

// upload is a file sent with a POST.
type upload struct {
	field, name string
	data        []byte
}

// call is a call of an API, as Radarr's DiskStationProxyBase builds them:
// a GET with its parameters in the query, or a multipart POST for uploads.
type call struct {
	api     api
	method  string
	version int
	params  []param
	file    *upload
	// op names the call in errors, as "get tasks".
	op string
}

// dsm calls the DSM web API: it discovers the APIs' paths and versions,
// kept for an hour, and logs in, keeping the session for 6 hours, as
// Radarr's DiskStationProxyBase.
type dsm struct {
	http *http.Client
	s    *settings
	now  func() time.Time
	log  *slog.Logger

	infoMu sync.Mutex
	infos  map[api]cachedInfo

	sidMu sync.Mutex
	sid   string
	sidAt time.Time
}

type cachedInfo struct {
	info apiInfo
	at   time.Time
}

// info returns an API's info, asking the DSM when not known; nil when the
// DSM does not have the API.
func (d *dsm) info(ctx context.Context, a api) (*apiInfo, error) {
	if a == synoInfo {
		info := infoAPI
		return &info, nil
	}
	d.infoMu.Lock()
	defer d.infoMu.Unlock()
	if c, ok := d.infos[a]; ok && d.now().Sub(c.at) < infoCacheTime {
		return &c.info, nil
	}
	var data map[string]apiInfo
	err := d.do(ctx, &infoAPI, call{
		api: synoInfo, method: "query", version: 1, op: "get api info",
		params: []param{{"query", string(synoAuth) + "," + string(a)}},
	}, &data)
	if err != nil {
		return nil, err
	}
	if d.infos == nil {
		d.infos = map[api]cachedInfo{}
	}
	for name, info := range data {
		info.Path = strings.TrimLeft(info.Path, `/\`)
		d.infos[api(name)] = cachedInfo{info: info, at: d.now()}
	}
	if c, ok := d.infos[a]; ok {
		return &c.info, nil
	}
	return nil, nil
}

// require returns an API's info, failing when the DSM does not have it.
func (d *dsm) require(ctx context.Context, a api) (*apiInfo, error) {
	info, err := d.info(ctx, a)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, fmt.Errorf("info of %s not found on %s:%d", a, d.s.Host, d.s.Port)
	}
	return info, nil
}

// call calls an API and decodes its data into out, when not nil.
func (d *dsm) call(ctx context.Context, c call, out any) error {
	info, err := d.require(ctx, c.api)
	if err != nil {
		return err
	}
	return d.do(ctx, info, c, out)
}

// session returns the session ID, logging in when there is none.
func (d *dsm) session(ctx context.Context) (string, error) {
	d.sidMu.Lock()
	defer d.sidMu.Unlock()
	if d.sid != "" && d.now().Sub(d.sidAt) < sessionTime {
		return d.sid, nil
	}
	info, err := d.require(ctx, synoAuth)
	if err != nil {
		return "", err
	}
	version := 2
	if info.MaxVersion >= 7 {
		version = 6
	}
	var data struct {
		SID string `json:"sid"`
	}
	err = d.do(ctx, info, call{
		api: synoAuth, method: "login", version: version, op: "login",
		params: []param{
			{"account", d.s.Username}, {"passwd", d.s.Password}, {"format", "sid"}, {"session", "DownloadStation"},
		},
	}, &data)
	if err != nil {
		return "", err
	}
	d.sid, d.sidAt = data.SID, d.now()
	return d.sid, nil
}

func (d *dsm) clearSession() {
	d.sidMu.Lock()
	defer d.sidMu.Unlock()
	d.sid = ""
}

func (d *dsm) do(ctx context.Context, info *apiInfo, c call, out any) error {
	if c.version < info.MinVersion || c.version > info.MaxVersion {
		return fmt.Errorf("%s version %d is not supported, the Diskstation supports %d to %d", c.api, c.version, info.MinVersion, info.MaxVersion)
	}
	var sid string
	if needsAuth(c.api) {
		s, err := d.session(ctx)
		if err != nil {
			return err
		}
		sid = s
	}
	req, err := d.request(ctx, info, c, sid)
	if err != nil {
		return err
	}
	resp, err := client.Send(d.http, req)
	if err != nil {
		return fmt.Errorf("unable to connect to Diskstation, please check your settings: %w", err)
	}
	if err := resp.Err(); err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return &client.HTTPError{StatusCode: resp.StatusCode}
	}
	var r struct {
		Success bool `json:"success"`
		Error   *struct {
			Code int `json:"code"`
		} `json:"error"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(resp.Body, &r); err != nil {
		return fmt.Errorf("failed to %s: %w", c.op, err)
	}
	if !r.Success {
		code := 0
		if r.Error != nil {
			code = r.Error.Code
		}
		msg := fmt.Sprintf("failed to %s. Reason: %s", c.op, errorMessage(c.api, code))
		d.log.ErrorContext(ctx, msg)
		switch {
		case c.api == synoAuth:
			return &client.AuthError{Err: errors.New(msg)}
		case sessionError(code):
			d.clearSession()
			if code == errCodePermission {
				return &client.AuthError{Err: errors.New(msg)}
			}
		}
		return &apiError{Code: code, msg: msg}
	}
	if out != nil && len(r.Data) > 0 && string(r.Data) != "null" {
		if err := json.Unmarshal(r.Data, out); err != nil {
			return fmt.Errorf("failed to %s: %w", c.op, err)
		}
	}
	return nil
}

func (d *dsm) request(ctx context.Context, info *apiInfo, c call, sid string) (*http.Request, error) {
	u := d.s.BaseURL() + "/webapi/" + info.Path
	query := url.Values{}
	base := []param{{"api", string(c.api)}, {"version", strconv.Itoa(c.version)}, {"method", c.method}}
	if c.file == nil {
		if sid != "" {
			query.Set("_sid", sid)
		}
		for _, p := range append(base, c.params...) {
			query.Add(p.key, p.value)
		}
		return http.NewRequestWithContext(ctx, http.MethodGet, u+"?"+query.Encode(), nil)
	}
	var fields []param
	if sid != "" {
		if c.api == synoTask2 {
			query.Set("_sid", sid)
		} else {
			fields = append(fields, param{"_sid", sid})
		}
	}
	fields = append(append(fields, base...), c.params...)
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for _, p := range fields {
		if err := w.WriteField(p.key, p.value); err != nil {
			return nil, err
		}
	}
	part, err := w.CreateFormFile(c.file.field, c.file.name)
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(c.file.data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req, nil
}
