package assistant

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/libs/core"
)

const (
	// recordTTL is how long the speech of an answer waits for its
	// conversation.
	recordTTL = 10 * time.Minute
	// maxParts is how many sentences of an answer are kept.
	maxParts = 64
)

var (
	// speechIDs are the IDs clients make for answers' speech.
	speechIDs = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)
	// conversationIDs are the IDs of conversations the folders are named
	// after.
	conversationIDs = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	answerKeys      = regexp.MustCompile(`^[0-9a-f]{32}$`)
	partNames       = regexp.MustCompile(`^([0-9]{1,2})\.(ogg|aac|mp3|flac|wav)$`)
	// partIndexes are the sentences in URLs, which name their encoding
	// in URLs made before speech was converted.
	partIndexes = regexp.MustCompile(`^([0-9]{1,2})(?:\.[a-z0-9]+)?$`)
)

// recorder keeps the speech of answers with their conversations
// (docs/assistant.md §8.3): a folder per conversation holding a folder per
// answer, named after its text, which holds a file per sentence and
// encoding: the synthesizer's, and those it was converted to for clients
// that do not play it. The speech of an answer waits in memory until its
// conversation is known.
type recorder struct {
	root *os.Root
	log  *slog.Logger

	mu   sync.Mutex
	open map[string]*recording
}

// recording is the speech of an answer, by its speech ID.
type recording struct {
	user core.ID
	// dir is the answer's folder once its conversation is known.
	dir   string
	parts map[int][]part
	at    time.Time
}

type part struct {
	ext   string
	audio []byte
}

// newRecorder returns a recorder keeping speech in dir; empty keeps none.
func newRecorder(dir string, log *slog.Logger) *recorder {
	r := &recorder{log: log, open: map[string]*recording{}}
	if dir == "" {
		return r
	}
	root, err := func() (*os.Root, error) {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, err
		}
		return os.OpenRoot(dir)
	}()
	if err != nil {
		log.Warn("assistant: no speech is kept", "dir", dir, "err", err)
		return r
	}
	r.root = root
	return r
}

// answerKey names an answer's folder after its text.
func answerKey(text string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(text)))
	return hex.EncodeToString(sum[:16])
}

// add keeps a sentence of the answer of a speech ID in an encoding.
func (r *recorder) add(user core.ID, id string, index int, f audioFormat, audio []byte, now time.Time) {
	if r.root == nil || !speechIDs.MatchString(id) || index < 0 || index >= maxParts {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expire(now)
	rec := r.recording(user, id, now)
	if rec == nil {
		return
	}
	if rec.dir == "" {
		rec.parts[index] = append(rec.parts[index], part{f.ext, audio})
		return
	}
	r.write(rec.dir, index, part{f.ext, audio})
}

// bind gives the speech of an ID its answer in a conversation, replacing
// the speech of the same answer, and keeps what of it came.
func (r *recorder) bind(user core.ID, id, conversation, answer string, now time.Time) {
	if r.root == nil || !speechIDs.MatchString(id) || !conversationIDs.MatchString(conversation) || strings.TrimSpace(answer) == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	rec := r.recording(user, id, now)
	if rec == nil || rec.dir != "" {
		return
	}
	rec.dir = path.Join(conversation, answerKey(answer))
	if err := r.root.RemoveAll(rec.dir); err != nil {
		r.log.Warn("assistant: replace speech", "err", err)
	}
	for i, ps := range rec.parts {
		for _, p := range ps {
			r.write(rec.dir, i, p)
		}
	}
	rec.parts = nil
}

// recording returns the recording of an ID, made when missing; nil when
// it is another user's.
func (r *recorder) recording(user core.ID, id string, now time.Time) *recording {
	rec, ok := r.open[id]
	if !ok {
		rec = &recording{user: user, parts: map[int][]part{}, at: now}
		r.open[id] = rec
	}
	if rec.user != user {
		return nil
	}
	return rec
}

// write keeps a file of a sentence, whole or not at all: it is written
// aside and renamed.
func (r *recorder) write(dir string, index int, p part) {
	name := path.Join(dir, fmt.Sprintf("%d.%s", index, p.ext))
	err := r.root.MkdirAll(dir, 0o750)
	if err == nil {
		err = r.root.WriteFile(name+".part", p.audio, 0o640)
	}
	if err == nil {
		err = r.root.Rename(name+".part", name)
	}
	if err != nil {
		r.log.Warn("assistant: keep speech", "err", err)
	}
}

// expire forgets the recordings past recordTTL.
func (r *recorder) expire(now time.Time) {
	for id, rec := range r.open {
		if now.Sub(rec.at) > recordTTL {
			delete(r.open, id)
		}
	}
}

// parts returns the sentences of an answer's speech, in order.
func (r *recorder) parts(conversation, answer string) []int {
	var out []int
	for name := range r.files(conversation, answerKey(answer)) {
		if m := partNames.FindStringSubmatch(name); m != nil {
			i, _ := strconv.Atoi(m[1])
			out = append(out, i)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// encodings returns the encodings a sentence of an answer's speech is
// kept in, the most balanced first.
func (r *recorder) encodings(conversation, answer string, index int) []audioFormat {
	files := r.files(conversation, answer)
	var out []audioFormat
	for _, f := range audioFormats {
		if files[fmt.Sprintf("%d.%s", index, f.ext)] {
			out = append(out, f)
		}
	}
	return out
}

// files returns the names of the files of an answer's speech, by its key.
func (r *recorder) files(conversation, answer string) map[string]bool {
	if r.root == nil || !conversationIDs.MatchString(conversation) || !answerKeys.MatchString(answer) {
		return nil
	}
	f, err := r.root.Open(path.Join(conversation, answer))
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()
	entries, err := f.ReadDir(-1)
	if err != nil {
		return nil
	}
	out := map[string]bool{}
	for _, e := range entries {
		if partNames.MatchString(e.Name()) {
			out[e.Name()] = true
		}
	}
	return out
}

// file opens a sentence of an answer's speech in an encoding.
func (r *recorder) file(conversation, answer string, index int, f audioFormat) (*os.File, error) {
	if r.root == nil || !conversationIDs.MatchString(conversation) || !answerKeys.MatchString(answer) {
		return nil, fs.ErrNotExist
	}
	return r.root.Open(path.Join(conversation, answer, fmt.Sprintf("%d.%s", index, f.ext)))
}

// read reads a sentence of an answer's speech in an encoding.
func (r *recorder) read(conversation, answer string, index int, f audioFormat) ([]byte, error) {
	file, err := r.file(conversation, answer, index, f)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	return io.ReadAll(file)
}

// keep keeps a sentence of an answer's speech converted to an encoding.
func (r *recorder) keep(conversation, answer string, index int, f audioFormat, audio []byte) {
	if r.root == nil || !conversationIDs.MatchString(conversation) || !answerKeys.MatchString(answer) {
		return
	}
	r.write(path.Join(conversation, answer), index, part{f.ext, audio})
}

// remove deletes the speech of a conversation.
func (r *recorder) remove(conversation string) {
	if r.root == nil || !conversationIDs.MatchString(conversation) {
		return
	}
	if err := r.root.RemoveAll(conversation); err != nil {
		r.log.Warn("assistant: delete speech", "err", err)
	}
}

// forget forgets the recordings past recordTTL.
func (r *recorder) forget(now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expire(now)
}

// prune deletes the speech of conversations that are gone, but for that
// of the last hour, whose conversation may be newer than those listed.
func (r *recorder) prune(live map[string]bool, now time.Time) {
	if r.root == nil {
		return
	}
	f, err := r.root.Open(".")
	if err != nil {
		return
	}
	entries, err := f.ReadDir(-1)
	_ = f.Close()
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		r.log.Warn("assistant: list speech", "err", err)
	}
	for _, e := range entries {
		if !e.IsDir() || live[e.Name()] || !conversationIDs.MatchString(e.Name()) {
			continue
		}
		if info, err := e.Info(); err == nil && now.Sub(info.ModTime()) < time.Hour {
			continue
		}
		r.remove(e.Name())
	}
}

// KeptSpeechPath is the route of the sentences of answers' kept speech.
const KeptSpeechPath = "/assistant/conversations/{conversation}/speech/{answer}/{part}"

// KeptSpeechURL is the URL of a sentence of an answer's kept speech,
// relative to the server's base URL.
func KeptSpeechURL(conversation, answer string, index int) string {
	return "assistant/conversations/" + url.PathEscape(conversation) + "/speech/" + answer + "/" + strconv.Itoa(index)
}

// KeptSpeechHandler serves KeptSpeechPath to the conversation's user, in
// an encoding the query's formats name (§8.3): one it is kept in, else
// the most balanced it is converted to and kept in from then on; resolve
// authenticates the request.
func (s *Service) KeptSpeechHandler(resolve func(r *http.Request) (auth.Principal, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := resolve(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		conv, answer := r.PathValue("conversation"), r.PathValue("answer")
		if _, _, _, err := s.own(auth.WithPrincipal(r.Context(), p), conv); err != nil {
			http.NotFound(w, r)
			return
		}
		m := partIndexes.FindStringSubmatch(r.PathValue("part"))
		if m == nil {
			http.NotFound(w, r)
			return
		}
		index, _ := strconv.Atoi(m[1])
		kept := s.rec.encodings(conv, answer, index)
		if len(kept) == 0 {
			http.NotFound(w, r)
			return
		}
		formats := formatList(r.URL.Query().Get("formats"))
		if i := slices.IndexFunc(kept, func(f audioFormat) bool { return plays(formats, f) }); i >= 0 {
			f, err := s.rec.file(conv, answer, index, kept[i])
			if err != nil {
				http.NotFound(w, r)
				return
			}
			defer func() { _ = f.Close() }()
			info, err := f.Stat()
			if err != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", kept[i].contentType)
			w.Header().Set("Cache-Control", "private, no-cache")
			http.ServeContent(w, r, "", info.ModTime(), f)
			return
		}
		to := speechTarget(kept[0], formats)
		audio, err := s.rec.read(conv, answer, index, kept[0])
		if err == nil {
			audio, err = convert(r.Context(), s.cfg.FFmpeg, audio, to)
		}
		if err != nil {
			s.log.WarnContext(r.Context(), "assistant: convert kept speech", "err", err)
			http.Error(w, "the speech could not be converted", http.StatusBadGateway)
			return
		}
		s.rec.keep(conv, answer, index, to, audio)
		w.Header().Set("Content-Type", to.contentType)
		w.Header().Set("Cache-Control", "private, no-cache")
		_, _ = w.Write(audio)
	})
}

// formatList parses the formats a client plays, separated by commas.
func formatList(s string) []string {
	var out []string
	for f := range strings.SplitSeq(s, ",") {
		if f = strings.ToLower(strings.TrimSpace(f)); f != "" {
			out = append(out, f)
		}
	}
	return out
}
