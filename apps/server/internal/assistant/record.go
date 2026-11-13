package assistant

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/mavioai/mavio/apps/server/internal/auth"
)

var (
	// conversationIDs are the IDs of conversations the folders are named
	// after.
	conversationIDs = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	answerKeys      = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

// recorder keeps the speech of answers with their conversations
// (docs/assistant.md §8.8): a folder per conversation holding a file per
// answer and encoding, named after the answer's text: Opus, as the relay
// encodes it, and those it was converted to for clients that do not play
// it.
type recorder struct {
	root   *os.Root
	ffmpeg string
	log    *slog.Logger
}

// newRecorder returns a recorder keeping speech in dir, encoding it with
// ffmpeg; empty dir keeps none, and without ffmpeg speech is kept as WAV.
func newRecorder(dir, ffmpeg string, log *slog.Logger) *recorder {
	r := &recorder{ffmpeg: ffmpeg, log: log}
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

// close closes the folder speech is kept in.
func (r *recorder) close() error {
	if r.root == nil {
		return nil
	}
	return r.root.Close()
}

// answerKey names an answer's files after its text.
func answerKey(text string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(text)))
	return hex.EncodeToString(sum[:16])
}

// wav wraps 16-bit mono PCM at a rate in a WAV header.
func wav(pcm []byte, rate int) []byte {
	out := make([]byte, 44, 44+len(pcm))
	copy(out, "RIFF")
	binary.LittleEndian.PutUint32(out[4:], uint32(36+len(pcm)))
	copy(out[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(out[16:], 16)
	binary.LittleEndian.PutUint16(out[20:], 1)
	binary.LittleEndian.PutUint16(out[22:], 1)
	binary.LittleEndian.PutUint32(out[24:], uint32(rate))
	binary.LittleEndian.PutUint32(out[28:], uint32(rate*2))
	binary.LittleEndian.PutUint16(out[32:], 2)
	binary.LittleEndian.PutUint16(out[34:], 16)
	copy(out[36:], "data")
	binary.LittleEndian.PutUint32(out[40:], uint32(len(pcm)))
	return append(out, pcm...)
}

// keepPCM keeps the speech of an answer, 16-bit mono PCM at a rate,
// encoded as Opus, else as WAV without ffmpeg; it replaces the speech of
// the same answer.
func (r *recorder) keepPCM(ctx context.Context, conversation, answer string, pcm []byte, rate int) error {
	if r.root == nil || !conversationIDs.MatchString(conversation) || strings.TrimSpace(answer) == "" || len(pcm) == 0 {
		return nil
	}
	audio, f := wav(pcm, rate), audioFormats[len(audioFormats)-1]
	if opus, err := convert(ctx, r.ffmpeg, audio, audioFormats[0]); err == nil {
		audio, f = opus, audioFormats[0]
	} else if !errors.Is(err, errNoFFmpeg) {
		return err
	}
	key := answerKey(answer)
	for _, k := range r.encodings(conversation, key) {
		if err := r.root.Remove(path.Join(conversation, key+"."+k.ext)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return r.write(conversation, key, f, audio)
}

// write keeps a file of an answer's speech, whole or not at all: it is
// written aside and renamed.
func (r *recorder) write(conversation, key string, f audioFormat, audio []byte) error {
	name := path.Join(conversation, key+"."+f.ext)
	err := r.root.MkdirAll(conversation, 0o750)
	if err == nil {
		err = r.root.WriteFile(name+".part", audio, 0o640)
	}
	if err == nil {
		err = r.root.Rename(name+".part", name)
	}
	return err
}

// has tells whether the speech of an answer is kept.
func (r *recorder) has(conversation, answer string) bool {
	return len(r.encodings(conversation, answerKey(answer))) > 0
}

// encodings returns the encodings an answer's speech is kept in, by its
// key, the most balanced first.
func (r *recorder) encodings(conversation, key string) []audioFormat {
	if r.root == nil || !conversationIDs.MatchString(conversation) || !answerKeys.MatchString(key) {
		return nil
	}
	var out []audioFormat
	for _, f := range audioFormats {
		if _, err := r.root.Stat(path.Join(conversation, key+"."+f.ext)); err == nil {
			out = append(out, f)
		}
	}
	return out
}

// file opens an answer's speech in an encoding.
func (r *recorder) file(conversation, key string, f audioFormat) (*os.File, error) {
	if r.root == nil || !conversationIDs.MatchString(conversation) || !answerKeys.MatchString(key) {
		return nil, fs.ErrNotExist
	}
	return r.root.Open(path.Join(conversation, key+"."+f.ext))
}

// read reads an answer's speech in an encoding.
func (r *recorder) read(conversation, key string, f audioFormat) ([]byte, error) {
	file, err := r.file(conversation, key, f)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	return io.ReadAll(file)
}

// keep keeps an answer's speech converted to an encoding.
func (r *recorder) keep(conversation, key string, f audioFormat, audio []byte) {
	if r.root == nil || !conversationIDs.MatchString(conversation) || !answerKeys.MatchString(key) {
		return
	}
	if err := r.write(conversation, key, f, audio); err != nil {
		r.log.Warn("assistant: keep speech", "err", err)
	}
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

// KeptSpeechPath is the route of answers' kept speech.
const KeptSpeechPath = "/assistant/conversations/{conversation}/speech/{answer}"

// KeptSpeechURL is the URL of an answer's kept speech, relative to the
// server's base URL.
func KeptSpeechURL(conversation, answer string) string {
	return "assistant/conversations/" + url.PathEscape(conversation) + "/speech/" + answerKey(answer)
}

// KeptSpeechHandler serves KeptSpeechPath to the conversation's user, in
// an encoding the query's formats name (§8.8): one it is kept in, else
// the most balanced it is converted to and kept in from then on; resolve
// authenticates the request.
func (s *Service) KeptSpeechHandler(resolve func(r *http.Request) (auth.Principal, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := resolve(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		conv, key := r.PathValue("conversation"), r.PathValue("answer")
		if _, _, _, err := s.own(auth.WithPrincipal(r.Context(), p), conv); err != nil {
			http.NotFound(w, r)
			return
		}
		kept := s.rec.encodings(conv, key)
		if len(kept) == 0 {
			http.NotFound(w, r)
			return
		}
		formats := formatList(r.URL.Query().Get("formats"))
		if i := slices.IndexFunc(kept, func(f audioFormat) bool { return plays(formats, f) }); i >= 0 {
			f, err := s.rec.file(conv, key, kept[i])
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
		audio, err := s.rec.read(conv, key, kept[0])
		if err == nil {
			audio, err = convert(r.Context(), s.cfg.FFmpeg, audio, to)
		}
		if err != nil {
			s.log.WarnContext(r.Context(), "assistant: convert kept speech", "err", err)
			http.Error(w, "the speech could not be converted", http.StatusBadGateway)
			return
		}
		s.rec.keep(conv, key, to, audio)
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
