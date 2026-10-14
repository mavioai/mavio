package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func TestMediaURLs(t *testing.T) {
	ctx := t.Context()
	s, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	newSession := func(name string, disabled bool) core.ID {
		u := core.User{Name: name, PasswordHash: HashPassword("pw"), Disabled: disabled}
		if err := s.Users().Create(ctx, &u); err != nil {
			t.Fatal(err)
		}
		sess := core.AuthSession{UserID: u.ID, DeviceKey: make([]byte, 32), DeviceID: "d"}
		if err := s.AuthSessions().Create(ctx, &sess); err != nil {
			t.Fatal(err)
		}
		return sess.ID
	}
	alice, bob := newSession("alice", false), newSession("bob", true)

	t0 := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	now := t0
	m := NewMediaURLs(s, make([]byte, 32))
	m.now = func() time.Time { return now }
	other := NewMediaURLs(s, []byte("another key"))

	// Requests reach next with the path the URL stands for.
	var served string
	h := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.Context().Value(mediaGrantKey{}).(core.ID); !ok {
			t.Errorf("%s served without a grant", r.URL.Path)
		}
		served = r.URL.Path
	}))
	get := func(u string, plugin bool) (int, string) {
		served = ""
		r := httptest.NewRequestWithContext(ctx, http.MethodGet, "/"+u, nil)
		if plugin {
			r = r.WithContext(context.WithValue(r.Context(), pluginKey{}, Plugin{ID: "dlna"}))
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code, served
	}

	playback := m.Sign(alice, "media/pb/master.m3u8", 2, time.Time{})
	if !strings.HasPrefix(playback, MediaPrefix) || !strings.HasSuffix(playback, "/media/pb/master.m3u8") {
		t.Errorf("Sign = %q", playback)
	}
	base := strings.TrimSuffix(playback, "master.m3u8")
	image := m.SignImage(alice, "images/img", 2)
	tests := []struct {
		name, url string
		plugin    bool
		want      int
		wantPath  string
	}{
		{"playlist", playback, false, http.StatusOK, "/media/pb/master.m3u8"},
		{"segment beside it", base + "3.mp4", false, http.StatusOK, "/media/pb/3.mp4"},
		{"another playback", strings.Replace(playback, "/pb/", "/pc/", 1), false, http.StatusForbidden, ""},
		{"image", image, false, http.StatusOK, "/images/img"},
		{"another key", other.Sign(alice, "images/img", 2, time.Time{}), false, http.StatusForbidden, ""},
		{"ended session", m.Sign(core.NewID(), "images/img", 2, time.Time{}), false, http.StatusForbidden, ""},
		{"disabled user", m.Sign(bob, "images/img", 2, time.Time{}), false, http.StatusForbidden, ""},
		{"plugin device", m.Sign(core.NewID(), "media/pb/stream.mkv", 2, time.Time{}), true, http.StatusOK, "/media/pb/stream.mkv"},
		{"not a token", MediaPrefix + "nope/images/img", false, http.StatusForbidden, ""},
		{"expired", m.Sign(alice, "images/img", 2, t0), false, http.StatusForbidden, ""},
	}
	for _, tt := range tests {
		if code, p := get(tt.url, tt.plugin); code != tt.want || p != tt.wantPath {
			t.Errorf("%s: got = %d %q, want = %d %q", tt.name, code, p, tt.want, tt.wantPath)
		}
	}

	// Image URLs stay the same within a period and expire after the next.
	now = t0.Add(ImageURLPeriod / 2)
	if again := m.SignImage(alice, "images/img", 2); again != image {
		t.Errorf("image URL changed within its period: %q, then %q", image, again)
	}
	now = t0.Add(2*ImageURLPeriod - time.Second)
	if code, _ := get(image, false); code != http.StatusOK {
		t.Errorf("image URL in the next period = %d, want = 200", code)
	}
	now = t0.Add(2 * ImageURLPeriod)
	if code, _ := get(image, false); code != http.StatusForbidden {
		t.Errorf("image URL after the next period = %d, want = 403", code)
	}

	// Ending a session stops its URLs once its check expires.
	if err := s.AuthSessions().Delete(ctx, alice); err != nil {
		t.Fatal(err)
	}
	if code, _ := get(playback, false); code != http.StatusOK {
		t.Errorf("playback just after its session ended = %d, want = 200 until rechecked", code)
	}
	now = now.Add(sessionCheckTTL)
	if code, _ := get(playback, false); code != http.StatusForbidden {
		t.Errorf("playback of an ended session = %d, want = 403", code)
	}

	// The signer of a request signs for its session only.
	if got := MediaURL(WithMediaURLs(ctx, m), "images/img", 2, true); got != "" {
		t.Errorf("MediaURL without a session = %q, want = empty", got)
	}
	signed := WithMediaURLs(WithPrincipal(ctx, Principal{Session: core.AuthSession{ID: alice}}), m)
	if got := MediaURL(signed, "media/pb/stream.mkv", 2, false); got != m.Sign(alice, "media/pb/stream.mkv", 2, time.Time{}) {
		t.Errorf("MediaURL = %q", got)
	}
}

func TestRequireMedia(t *testing.T) {
	ctx := t.Context()
	s, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	u := core.User{Name: "alice", PasswordHash: HashPassword("pw")}
	if err := s.Users().Create(ctx, &u); err != nil {
		t.Fatal(err)
	}
	sess := core.AuthSession{UserID: u.ID, DeviceKey: make([]byte, 32), DeviceID: "d", LastSeenAt: time.Now()}
	if err := s.AuthSessions().Create(ctx, &sess); err != nil {
		t.Fatal(err)
	}
	tokens := NewAccessTokens(make([]byte, 32))
	token, _ := tokens.Issue(sess.ID)
	h := RequireMedia(NewInterceptor(s, tokens), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	tests := []struct {
		name string
		ctx  context.Context
		auth string
		want int
	}{
		{"no credentials", ctx, "", http.StatusUnauthorized},
		{"bad token", ctx, "Bearer nope", http.StatusUnauthorized},
		{"access token", ctx, "Bearer " + token, http.StatusOK},
		{"signed URL", context.WithValue(ctx, mediaGrantKey{}, sess.ID), "", http.StatusOK},
		{"plugin", context.WithValue(ctx, pluginKey{}, Plugin{ID: "dlna"}), "", http.StatusOK},
	}
	for _, tt := range tests {
		r := httptest.NewRequestWithContext(tt.ctx, http.MethodGet, "/images/img", nil)
		if tt.auth != "" {
			r.Header.Set("Authorization", tt.auth)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tt.want {
			t.Errorf("%s: got = %d, want = %d", tt.name, w.Code, tt.want)
		}
	}
}
