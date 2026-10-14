package auth

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/libs/core"
)

// MediaKeyPurpose derives the key of media URLs from the server's
// identity key.
const MediaKeyPurpose = "mavio media urls v1"

// MediaPrefix starts the path of a signed media URL: signed/{token}/{path}.
const MediaPrefix = "signed/"

// Lifetimes of media URLs.
const (
	// ImageURLPeriod is how long an image's URL stays the same, so that
	// caches keep their copies; it is accepted for a period more.
	ImageURLPeriod = 12 * time.Hour
	// sessionCheckTTL is how long a session found signed in is not looked
	// up again; an ended session's URLs stop working within it.
	sessionCheckTTL = 30 * time.Second
)

const (
	mediaTokenVersion = 1
	// version, session ID, expiry in Unix seconds (0 for none), the
	// number of path segments signed, and the truncated HMAC-SHA256.
	mediaTokenLen = 1 + 16 + 8 + 1 + mediaMACLen
	mediaMACLen   = 16
)

var errBadMediaToken = errors.New("invalid media URL")

// MediaURLs signs and checks the URLs of media: direct play, HLS
// playlists and segments, subtitles, attachments and images. A URL is
// signed for one session and a resource, the first segments of its path,
// so that the paths below it, such as an HLS playlist's segments, share
// it; players and casting receivers need no headers, and a URL stops
// working when its session ends or it expires.
type MediaURLs struct {
	store core.Store
	key   []byte
	now   func() time.Time

	mu      sync.Mutex
	checked map[core.ID]time.Time
}

// NewMediaURLs returns media URLs authenticated with key, whose sessions
// are looked up in store.
func NewMediaURLs(store core.Store, key []byte) *MediaURLs {
	return &MediaURLs{store: store, key: bytes.Clone(key), now: time.Now, checked: map[core.ID]time.Time{}}
}

// Sign returns the URL path, relative to the server's base URL, of p for
// session, signing its first scope segments. A zero expires never
// expires, for resources that end with their session, such as playbacks.
func (m *MediaURLs) Sign(session core.ID, p string, scope int, expires time.Time) string {
	p = strings.TrimPrefix(p, "/")
	segments := strings.Split(p, "/")
	if scope < 1 || scope > len(segments) || scope > 255 {
		scope = len(segments)
	}
	var exp uint64
	if !expires.IsZero() {
		exp = uint64(expires.Unix())
	}
	b := make([]byte, 0, mediaTokenLen)
	b = append(b, mediaTokenVersion)
	b = append(b, session[:]...)
	b = binary.BigEndian.AppendUint64(b, exp)
	b = append(b, byte(scope))
	b = append(b, m.mac(b, strings.Join(segments[:scope], "/"))...)
	return MediaPrefix + base64.RawURLEncoding.EncodeToString(b) + "/" + p
}

// SignImage signs the path of an image, its first scope segments, until
// the end of the period after the current one.
func (m *MediaURLs) SignImage(session core.ID, p string, scope int) string {
	return m.Sign(session, p, scope, m.now().Truncate(ImageURLPeriod).Add(2*ImageURLPeriod))
}

func (m *MediaURLs) mac(header []byte, scope string) []byte {
	h := hmac.New(sha256.New, m.key)
	h.Write(header)
	h.Write([]byte(scope))
	return h.Sum(nil)[:mediaMACLen]
}

// verify returns the session of a signed path, token/path without the
// prefix, and the path it stands for.
func (m *MediaURLs) verify(signed string) (core.ID, string, error) {
	token, p, ok := strings.Cut(signed, "/")
	if !ok {
		return core.ID{}, "", errBadMediaToken
	}
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(b) != mediaTokenLen || b[0] != mediaTokenVersion {
		return core.ID{}, "", errBadMediaToken
	}
	header, sum := b[:mediaTokenLen-mediaMACLen], b[mediaTokenLen-mediaMACLen:]
	scope := int(header[len(header)-1])
	segments := strings.Split(p, "/")
	if scope < 1 || scope > len(segments) || !hmac.Equal(sum, m.mac(header, strings.Join(segments[:scope], "/"))) {
		return core.ID{}, "", errBadMediaToken
	}
	if exp := binary.BigEndian.Uint64(header[17:25]); exp != 0 && !m.now().Before(time.Unix(int64(exp), 0)) {
		return core.ID{}, "", errBadMediaToken
	}
	return core.ID(header[1:17]), p, nil
}

// signedIn reports whether session is still signed in by an enabled user.
func (m *MediaURLs) signedIn(ctx context.Context, session core.ID) (bool, error) {
	now := m.now()
	m.mu.Lock()
	until, ok := m.checked[session]
	m.mu.Unlock()
	if ok && now.Before(until) {
		return true, nil
	}
	sess, err := m.store.AuthSessions().Get(ctx, session)
	if errors.Is(err, core.ErrNotFound) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("look up session: %w", err)
	}
	user, err := m.store.Users().Get(ctx, sess.UserID)
	if errors.Is(err, core.ErrNotFound) || err == nil && user.Disabled {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("look up user: %w", err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, t := range m.checked {
		if !now.Before(t) {
			delete(m.checked, id)
		}
	}
	m.checked[session] = now.Add(sessionCheckTTL)
	return true, nil
}

type mediaGrantKey struct{}

// Handler serves signed media URLs, GET /signed/{token}/{path...}, as next
// serves /{path}, once the token is checked. Plugins' host API requests
// are not checked for the session, which may be a plugin's device.
func (m *MediaURLs) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		signed, ok := strings.CutPrefix(r.URL.Path, "/"+MediaPrefix)
		if !ok {
			http.NotFound(w, r)
			return
		}
		session, p, err := m.verify(signed)
		if err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		if _, plugin := r.Context().Value(pluginKey{}).(Plugin); !plugin {
			ok, err := m.signedIn(r.Context(), session)
			if err != nil {
				slog.ErrorContext(r.Context(), "check media URL", "err", err)
				http.Error(w, "session unavailable", http.StatusInternalServerError)
				return
			} else if !ok {
				http.Error(w, "session ended", http.StatusForbidden)
				return
			}
		}
		r2 := r.Clone(context.WithValue(r.Context(), mediaGrantKey{}, session))
		r2.URL.Path, r2.URL.RawPath = "/"+p, ""
		next.ServeHTTP(w, r2)
	})
}

// RequireMedia serves next for requests through a signed media URL, from
// plugins' host API, or with a bearer token resolved by authn; it answers
// others unauthorized.
func RequireMedia(authn *Interceptor, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, granted := r.Context().Value(mediaGrantKey{}).(core.ID)
		_, plugin := r.Context().Value(pluginKey{}).(Plugin)
		if granted || plugin {
			next.ServeHTTP(w, r)
			return
		}
		token, ok := bearer(r.Header)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if _, err := authn.Resolve(r.Context(), token); connect.CodeOf(err) == connect.CodeUnauthenticated {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		} else if err != nil {
			slog.ErrorContext(r.Context(), "authenticate media request", "err", err)
			http.Error(w, "authentication unavailable", http.StatusInternalServerError)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type mediaSignerKey struct{}

// WithMediaURLs returns ctx signing media URLs with m for its principal.
func WithMediaURLs(ctx context.Context, m *MediaURLs) context.Context {
	return context.WithValue(ctx, mediaSignerKey{}, m)
}

// MediaURL returns the path p signed for the session of ctx's principal,
// its first scope segments, as MediaURLs.Sign; image tells whether it
// expires as an image's. It is empty for principals without a session,
// such as API keys, which fetch media with their credentials.
func MediaURL(ctx context.Context, p string, scope int, image bool) string {
	m, ok := ctx.Value(mediaSignerKey{}).(*MediaURLs)
	pr, _ := FromContext(ctx)
	if !ok || pr.Session.ID.IsZero() {
		return ""
	}
	if image {
		return m.SignImage(pr.Session.ID, p, scope)
	}
	return m.Sign(pr.Session.ID, p, scope, time.Time{})
}
