package assistant

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/net/websocket"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/apps/server/internal/speech"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/media/vad"
	assistantv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/assistant/v1"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// Paths of the plain HTTP routes.
const (
	ListenPath = "/assistant/listen"
	SpeechPath = "/assistant/speech"
)

const (
	listenTTL = time.Minute
	// sampleRate and the limits of an utterance (docs/assistant.md §8.2).
	sampleRate   = 16000
	maxUtterance = 30 * time.Second
	endSilence   = time.Second
	speechProb   = 0.5
)

// listenToken is what a listening URL carries, signed.
type listenToken struct {
	User       string `json:"u"`
	Session    string `json:"s"`
	Recognizer string `json:"r"`
	Language   string `json:"l"`
	Expires    int64  `json:"e"`
	Nonce      string `json:"n"`
}

func (s *Service) sign(t listenToken) string {
	b, _ := json.Marshal(t)
	payload := base64.RawURLEncoding.EncodeToString(b)
	mac := hmac.New(sha256.New, s.cfg.Key)
	mac.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// verify checks a token, which serves once.
func (s *Service) verify(token string) (listenToken, error) {
	payload, sig, ok := strings.Cut(token, ".")
	if !ok || len(s.cfg.Key) == 0 {
		return listenToken{}, errors.New("malformed token")
	}
	mac := hmac.New(sha256.New, s.cfg.Key)
	mac.Write([]byte(payload))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(want)) {
		return listenToken{}, errors.New("bad signature")
	}
	b, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return listenToken{}, err
	}
	var t listenToken
	if err := json.Unmarshal(b, &t); err != nil {
		return listenToken{}, err
	}
	now := s.now()
	if now.Unix() > t.Expires {
		return listenToken{}, errors.New("expired token")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, used := s.listens[t.Nonce]; used {
		return listenToken{}, errors.New("used token")
	}
	s.listens[t.Nonce] = time.Unix(t.Expires, 0)
	return t, nil
}

// StartListening returns the WebSocket URL of the server's recognizer.
func (s *Service) StartListening(ctx context.Context, req *assistantv1.StartListeningRequest) (*assistantv1.StartListeningResponse, error) {
	p, set, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	if p.Session.ID.IsZero() {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("listening needs a signed-in device"))
	}
	id := req.GetRecognizer()
	if id == "" {
		id = set.Recognizer
	}
	r, err := speech.Pick(s.recognizers(ctx), id, req.GetLanguage(), nil)
	if errors.Is(err, speech.ErrNone) && req.GetRecognizer() == "" && id != "" {
		// The settings' recognizer does not take the language.
		r, err = speech.Pick(s.recognizers(ctx), "", req.GetLanguage(), nil)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	exp := s.now().Add(listenTTL)
	tok := s.sign(listenToken{
		User: p.User.ID.String(), Session: p.Session.ID.String(), Recognizer: r.ID, Language: req.GetLanguage(),
		Expires: exp.Unix(), Nonce: core.NewID().String(),
	})
	return assistantv1.StartListeningResponse_builder{
		Url: new(ListenPath + "?token=" + url.QueryEscape(tok)), Recognizer: &r.ID, ExpireTime: timestamppb.New(exp),
	}.Build(), nil
}

// heard is what the listening route sends back, as JSON text.
type heard struct {
	Text     string `json:"text,omitempty"`
	Language string `json:"language,omitempty"`
	// Final ends the utterance: the client stops sending.
	Final bool   `json:"final,omitempty"`
	Error string `json:"error,omitempty"`
}

// vadLoader loads the speech detector's model once.
type vadLoader struct {
	once  sync.Once
	model *vad.Model
	err   error
}

func (l *vadLoader) load() (*vad.Model, error) {
	l.once.Do(func() { l.model, l.err = vad.NewModel() })
	return l.model, l.err
}

// ListenHandler serves the listening route: the client sends 16 kHz mono
// 16-bit little-endian PCM in binary messages and "end" when the user
// lets go; the server ends the utterance itself after a second of silence
// or at 30 seconds, and answers with the text heard (§8.2).
func (s *Service) ListenHandler() http.Handler {
	ws := websocket.Server{
		// The token authenticates, so the origin need not be checked.
		Handshake: func(*websocket.Config, *http.Request) error { return nil },
		Handler: func(conn *websocket.Conn) {
			defer conn.Close()
			send := func(h heard) { _ = websocket.JSON.Send(conn, h) }
			ctx := conn.Request().Context()
			tok, err := s.verify(conn.Request().URL.Query().Get("token"))
			if err != nil {
				send(heard{Error: err.Error(), Final: true})
				return
			}
			text, lang, err := s.listen(ctx, tok, conn)
			if err != nil {
				s.log.WarnContext(ctx, "assistant: listen", "err", err)
				send(heard{Error: err.Error(), Final: true})
				return
			}
			send(heard{Text: text, Language: lang, Final: true})
		},
	}
	return ws
}

// listen reads an utterance and recognizes it.
func (s *Service) listen(ctx context.Context, tok listenToken, conn *websocket.Conn) (string, string, error) {
	uid, err := core.ParseID(tok.User)
	if err != nil {
		return "", "", err
	}
	u, err := s.cfg.Users.Get(ctx, uid)
	if err != nil {
		return "", "", err
	}
	r, err := speech.Pick(s.recognizers(ctx), tok.Recognizer, "", nil)
	if err != nil {
		return "", "", err
	}
	model, err := s.vad.load()
	if err != nil {
		return "", "", err
	}
	det := model.NewDetector()
	var (
		pcm, rest []byte
		window    = make([]float32, 0, 512)
		spoke     bool
		silence   time.Duration
		perFrame  = time.Duration(512) * time.Second / sampleRate
	)
	limit := sampleRate * 2 * int(maxUtterance/time.Second)
	_ = conn.SetReadDeadline(time.Now().Add(maxUtterance + 10*time.Second))
read:
	for len(pcm) < limit {
		var msg []byte
		if err := websocket.Message.Receive(conn, &msg); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return "", "", err
		}
		// PCM comes in whole samples, so the three bytes of "end" cannot be
		// audio.
		if string(msg) == "end" {
			break
		}
		pcm = append(pcm, msg...)
		rest = append(rest, msg...)
		n := len(rest) &^ 1
		for i := 0; i < n; i += 2 {
			window = append(window, float32(int16(binary.LittleEndian.Uint16(rest[i:])))/32768)
			if len(window) < cap(window) {
				continue
			}
			p, err := det.Process(window)
			window = window[:0]
			if err != nil {
				return "", "", err
			}
			switch {
			case p >= speechProb:
				spoke, silence = true, 0
			case spoke:
				if silence += perFrame; silence >= endSilence {
					break read
				}
			}
		}
		rest = rest[n:]
	}
	if !spoke || len(pcm) < 2 {
		return "", tok.Language, nil
	}
	pcm = pcm[:len(pcm)-len(pcm)%2]
	req := pluginv1.RecognizeSpeechRequest_builder{
		Audio:    pluginv1.SpeechAudio_builder{Pcm: pcm, Limit: durationpb.New(maxUtterance)}.Build(),
		Language: &tok.Language,
	}.Build()
	if set := s.cfg.Settings(); set.Hints && r.Info.GetHints() {
		req.SetHints(s.hints(auth.WithPrincipal(ctx, auth.Principal{User: u}), auth.Principal{User: u}))
	}
	resp, err := r.Recognize(ctx, req)
	if err != nil {
		return "", "", fmt.Errorf("recognize: %w", err)
	}
	lang := resp.GetLanguage()
	if lang == "" {
		lang = tok.Language
	}
	return strings.TrimSpace(resp.GetText()), lang, nil
}

// speechRequest is the body of the speech route.
type speechRequest struct {
	Text     string `json:"text"`
	Language string `json:"language"`
	// Voice is "<plugin>/<synthesizer>/<voice>"; empty picks the
	// settings' voice for the language, else the first.
	Voice string `json:"voice"`
	// Format is "mp3" or "opus" (Opus in Ogg, the default).
	Format string `json:"format"`
}

// SpeechHandler serves POST /assistant/speech, which speaks a text with a
// server voice (§8.3); resolve authenticates the request.
func (s *Service) SpeechHandler(resolve func(r *http.Request) (auth.Principal, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := resolve(r); err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		set := s.cfg.Settings()
		if !set.Enabled {
			http.Error(w, "the assistant is off", http.StatusPreconditionFailed)
			return
		}
		var req speechRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&req); err != nil || strings.TrimSpace(req.Text) == "" || len(req.Text) > 4000 {
			http.Error(w, "text of 1–4000 characters is required", http.StatusBadRequest)
			return
		}
		voices := s.voices(r.Context())
		v, ok := pickVoice(voices, req.Voice, req.Language, set.Voices)
		if !ok {
			http.Error(w, "no voice speaks "+req.Language, http.StatusPreconditionFailed)
			return
		}
		format := pluginv1.SpeechFormat_SPEECH_FORMAT_OGG_OPUS
		if req.Format == "mp3" {
			format = pluginv1.SpeechFormat_SPEECH_FORMAT_MP3
		}
		resp, err := v.Synthesize(r.Context(), req.Text, req.Language, format)
		if err != nil {
			s.log.WarnContext(r.Context(), "assistant: synthesize", "err", err)
			http.Error(w, "synthesis failed", http.StatusBadGateway)
			return
		}
		ct := resp.GetContentType()
		if ct == "" {
			ct = map[bool]string{true: "audio/mpeg", false: "audio/ogg"}[req.Format == "mp3"]
		}
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(resp.GetAudio())
	})
}

// pickVoice picks the voice asked for, else the default of the language,
// else the first that speaks it.
func pickVoice(voices []speech.Voice, id, lang string, defaults map[string]string) (speech.Voice, bool) {
	if id == "" {
		id = defaults[lang]
		if id == "" {
			id = defaults[speech.Base(lang)]
		}
	}
	for _, v := range voices {
		if id != "" && v.ID == id {
			return v, true
		}
	}
	for _, v := range voices {
		if speech.Speaks(v.Info.GetLanguages(), lang) {
			return v, true
		}
	}
	return speech.Voice{}, false
}
