package assistant

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
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
	partNames       = regexp.MustCompile(`^([0-9]{1,2})\.(ogg|mp3)$`)
)

// recorder keeps the speech of answers with their conversations
// (docs/assistant.md §8.3): a folder per conversation holding a folder per
// answer, named after its text, which holds a file per sentence. The speech
// of an answer waits in memory until its conversation is known.
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
	parts map[int]part
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

// add keeps a sentence of the answer of a speech ID.
func (r *recorder) add(user core.ID, id string, index int, contentType string, audio []byte, now time.Time) {
	ext := map[string]string{"audio/ogg": "ogg", "audio/mpeg": "mp3"}[contentType]
	if r.root == nil || !speechIDs.MatchString(id) || index < 0 || index >= maxParts || ext == "" {
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
		rec.parts[index] = part{ext, audio}
		return
	}
	r.write(rec.dir, index, part{ext, audio})
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
	for i, p := range rec.parts {
		r.write(rec.dir, i, p)
	}
	rec.parts = nil
}

// recording returns the recording of an ID, made when missing; nil when
// it is another user's.
func (r *recorder) recording(user core.ID, id string, now time.Time) *recording {
	rec, ok := r.open[id]
	if !ok {
		rec = &recording{user: user, parts: map[int]part{}, at: now}
		r.open[id] = rec
	}
	if rec.user != user {
		return nil
	}
	return rec
}

func (r *recorder) write(dir string, index int, p part) {
	err := r.root.MkdirAll(dir, 0o750)
	if err == nil {
		err = r.root.WriteFile(path.Join(dir, fmt.Sprintf("%d.%s", index, p.ext)), p.audio, 0o640)
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

// parts returns the names of the files of an answer's speech, in order.
func (r *recorder) parts(conversation, answer string) []string {
	if r.root == nil || !conversationIDs.MatchString(conversation) {
		return nil
	}
	dir := path.Join(conversation, answerKey(answer))
	f, err := r.root.Open(dir)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()
	entries, err := f.ReadDir(-1)
	if err != nil {
		return nil
	}
	type named struct {
		index int
		name  string
	}
	var out []named
	for _, e := range entries {
		if m := partNames.FindStringSubmatch(e.Name()); m != nil {
			i, _ := strconv.Atoi(m[1])
			out = append(out, named{i, e.Name()})
		}
	}
	slices.SortFunc(out, func(a, b named) int { return cmp.Compare(a.index, b.index) })
	names := make([]string, len(out))
	for i, n := range out {
		names[i] = n.name
	}
	return names
}

// file opens a sentence of an answer's speech, with its content type.
func (r *recorder) file(conversation, answer, name string) (*os.File, string, error) {
	m := partNames.FindStringSubmatch(name)
	if r.root == nil || !conversationIDs.MatchString(conversation) || !answerKeys.MatchString(answer) || m == nil {
		return nil, "", fs.ErrNotExist
	}
	f, err := r.root.Open(path.Join(conversation, answer, name))
	if err != nil {
		return nil, "", err
	}
	return f, map[string]string{"ogg": "audio/ogg", "mp3": "audio/mpeg"}[m[2]], nil
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
func KeptSpeechURL(conversation, answer, part string) string {
	return "assistant/conversations/" + url.PathEscape(conversation) + "/speech/" + answer + "/" + part
}

// KeptSpeechHandler serves KeptSpeechPath to the conversation's user;
// resolve authenticates the request.
func (s *Service) KeptSpeechHandler(resolve func(r *http.Request) (auth.Principal, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := resolve(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		conv := r.PathValue("conversation")
		if _, _, _, err := s.own(auth.WithPrincipal(r.Context(), p), conv); err != nil {
			http.NotFound(w, r)
			return
		}
		f, ct, err := s.rec.file(conv, r.PathValue("answer"), r.PathValue("part"))
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
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Cache-Control", "private, no-cache")
		http.ServeContent(w, r, "", info.ModTime(), f)
	})
}
