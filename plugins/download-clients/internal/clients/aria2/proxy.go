package aria2

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

// status is a download of aria2.tellStatus and its kin, as Radarr's
// Aria2Status; aria2 gives numbers as strings.
type status struct {
	Name            string
	InfoHash        string
	CompletedLength int64
	DownloadSpeed   int64
	Files           []string
	GID             string
	Status          string
	TotalLength     int64
	UploadLength    int64
	ErrorMessage    string
}

func parseStatus(v *value) status {
	s := status{}
	str := func(name string) string {
		if m := v.member(name); m != nil {
			return m.str()
		}
		return ""
	}
	num := func(name string) int64 {
		n, _ := strconv.ParseInt(str(name), 10, 64)
		return n
	}
	if bt := v.member("bittorrent"); bt != nil {
		if info := bt.member("info"); info != nil {
			if n := info.member("name"); n != nil {
				s.Name = n.str()
			}
		}
		if s.Name == "" {
			if n := bt.member("name"); n != nil {
				s.Name = n.str()
			}
		}
	}
	if files := v.member("files"); files != nil {
		for _, f := range files.values() {
			if p := f.member("path"); p != nil {
				s.Files = append(s.Files, p.str())
			}
		}
	}
	s.InfoHash, s.GID, s.Status, s.ErrorMessage = str("infoHash"), str("gid"), str("status"), str("errorMessage")
	s.CompletedLength, s.DownloadSpeed = num("completedLength"), num("downloadSpeed")
	s.TotalLength, s.UploadLength = num("totalLength"), num("uploadLength")
	return s
}

// maxListed is how many waiting and stopped downloads are listed.
const maxListed = 10 * 1024

// proxy calls aria2's XML-RPC as Radarr's Aria2Proxy.
type proxy struct {
	http     *http.Client
	settings *settings
}

func (p *proxy) token() string { return "token:" + p.settings.SecretToken }

// call calls a method with the secret token and returns its result.
func (p *proxy) call(ctx context.Context, method string, params ...any) (*value, error) {
	body, err := encodeCall(method, append([]any{p.token()}, params...)...)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.settings.BaseURL(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "text/xml")
	resp, err := client.Send(p.http, req)
	if err != nil {
		return nil, err
	}
	if err := resp.Err(); err != nil {
		// A fault answered with a failure status is the fault.
		var fault *faultError
		if _, ferr := decodeResponse(resp.Body); errors.As(ferr, &fault) {
			return nil, fault
		}
		return nil, err
	}
	return decodeResponse(resp.Body)
}

func (p *proxy) version(ctx context.Context) (string, error) {
	v, err := p.call(ctx, "aria2.getVersion")
	if err != nil {
		return "", err
	}
	if m := v.member("version"); m != nil {
		return m.str(), nil
	}
	return "", nil
}

func (p *proxy) status(ctx context.Context, gid string) (status, error) {
	v, err := p.call(ctx, "aria2.tellStatus", gid)
	if err != nil {
		return status{}, err
	}
	return parseStatus(v), nil
}

func (p *proxy) list(ctx context.Context, method string, params ...any) ([]status, error) {
	v, err := p.call(ctx, method, params...)
	if err != nil {
		return nil, err
	}
	var out []status
	for _, s := range v.values() {
		out = append(out, parseStatus(&s))
	}
	return out, nil
}

// downloads lists the active, waiting and stopped downloads.
func (p *proxy) downloads(ctx context.Context) ([]status, error) {
	active, err := p.list(ctx, "aria2.tellActive")
	if err != nil {
		return nil, err
	}
	waiting, err := p.list(ctx, "aria2.tellWaiting", 0, maxListed)
	if err != nil {
		return nil, err
	}
	stopped, err := p.list(ctx, "aria2.tellStopped", 0, maxListed)
	if err != nil {
		return nil, err
	}
	return append(append(active, waiting...), stopped...), nil
}

// globals returns the global options.
func (p *proxy) globals(ctx context.Context) (map[string]string, error) {
	v, err := p.call(ctx, "aria2.getGlobalOption")
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	if v.Struct != nil {
		for i := range v.Struct.Members {
			out[v.Struct.Members[i].Name] = v.Struct.Members[i].Value.str()
		}
	}
	return out, nil
}

func options(dir string) map[string]string {
	o := map[string]string{}
	if dir != "" {
		o["dir"] = dir
	}
	return o
}

// addMagnet adds a magnet link and returns its GID.
func (p *proxy) addMagnet(ctx context.Context, link, dir string) (string, error) {
	v, err := p.call(ctx, "aria2.addUri", []string{link}, options(dir))
	if err != nil {
		return "", err
	}
	return v.str(), nil
}

// addTorrent adds a torrent file and returns its GID; the URIs, none,
// are to be sent for the options to be.
func (p *proxy) addTorrent(ctx context.Context, data []byte, dir string) (string, error) {
	v, err := p.call(ctx, "aria2.addTorrent", data, []string{}, options(dir))
	if err != nil {
		return "", err
	}
	return v.str(), nil
}

// remove removes an active or waiting download.
func (p *proxy) remove(ctx context.Context, gid string) (bool, error) {
	v, err := p.call(ctx, "aria2.forceRemove", gid)
	if err != nil {
		return false, err
	}
	return v.str() == gid, nil
}

// removeResult removes a stopped download.
func (p *proxy) removeResult(ctx context.Context, gid string) (bool, error) {
	v, err := p.call(ctx, "aria2.removeDownloadResult", gid)
	if err != nil {
		return false, err
	}
	return v.str() == "OK", nil
}
