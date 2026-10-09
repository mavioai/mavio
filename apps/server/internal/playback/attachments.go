package playback

import (
	"bytes"
	"cmp"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/libs/core"
)

// fontTypes are the MIME types of font attachments by codec, for files
// that name none.
var fontTypes = map[string]string{"ttf": "font/ttf", "otf": "font/otf", "woff": "font/woff", "woff2": "font/woff2"}

// Attachments lists the files attached to the source, such as the fonts
// its ASS subtitles use.
func (p *Playback) Attachments() []core.MediaStream {
	var out []core.MediaStream
	for _, st := range p.Source().Streams {
		if st.Kind == core.StreamAttachment {
			if st.MimeType == "" {
				st.MimeType = cmp.Or(fontTypes[strings.ToLower(st.Codec)], "application/octet-stream")
			}
			out = append(out, st)
		}
	}
	return out
}

// AttachmentURL returns the path of an attachment relative to the
// server's base URL.
func (p *Playback) AttachmentURL(index int) string {
	return "media/" + p.ID + "/attachments/" + strconv.Itoa(index)
}

// attachmentDir holds a playback's extracted attachments.
func (m *Manager) attachmentDir(p *Playback) string {
	return filepath.Join(*m.dir.Load(), p.ID+"-attachments")
}

// serveAttachment extracts an attachment with ffmpeg once and serves it.
func (m *Manager) serveAttachment(w http.ResponseWriter, r *http.Request, p *Playback) {
	index, err := strconv.Atoi(r.PathValue("index"))
	var att *core.MediaStream
	for _, st := range p.Attachments() {
		if st.Index == index {
			att = &st
		}
	}
	if err != nil || att == nil || m.cfg.FFmpegPath == "" {
		http.NotFound(w, r)
		return
	}
	p.attachMu.Lock()
	defer p.attachMu.Unlock()
	dir := m.attachmentDir(p)
	p.attachments = dir
	file := filepath.Join(dir, strconv.Itoa(index))
	if _, err := os.Stat(file); err != nil {
		if err := m.extractAttachment(r, p, index, dir, file); err != nil {
			m.log.ErrorContext(r.Context(), "extract attachment", "playback", p.ID, "index", index, "err", err)
			http.Error(w, "attachment unavailable", http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Type", att.MimeType)
	if att.Title != "" {
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", filepath.Base(att.Title)))
	}
	http.ServeFile(w, r, file)
}

func (m *Manager) extractAttachment(r *http.Request, p *Playback, index int, dir, file string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := file + ".tmp"
	var stderr bytes.Buffer
	// ffmpeg dumps the attachment while opening the input; the null output
	// only satisfies its need for one.
	cmd := exec.CommandContext(r.Context(), m.cfg.FFmpegPath, "-v", "error", "-nostdin", "-y",
		"-dump_attachment:"+strconv.Itoa(index), tmp, "-i", p.Source().Path, "-t", "0", "-f", "null", "-")
	cmd.Stderr = &limitedBuffer{b: &stderr, max: 16 << 10}
	_ = cmd.Run() // ffmpeg may report the null output's lack of streams
	if info, err := os.Stat(tmp); err != nil || info.Size() == 0 {
		_ = os.Remove(tmp)
		return fmt.Errorf("ffmpeg dumped nothing: %s", strings.TrimSpace(stderr.String()))
	}
	return os.Rename(tmp, file)
}
