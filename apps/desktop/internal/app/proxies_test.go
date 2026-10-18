package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
)

func TestProxies(t *testing.T) {
	h := newHarness(t, true, 1280, 900)
	want := func(texts ...string) {
		t.Helper()
		for _, w := range texts {
			if !h.tt.HasText(w) {
				t.Errorf("no %q; texts = %q", w, h.tt.Texts())
			}
		}
	}
	proxies := func() []*systemv1.Proxy {
		h.core.mu.Lock()
		defer h.core.mu.Unlock()
		return h.core.proxies
	}
	ps := &h.app.settingsState.admin.proxies

	// The system proxy, the server's own and the server's requests.
	h.app.router.Push("/settings/proxies")
	h.settle()
	want("System proxy", "http://10.0.0.1:3128 · Reached directly: localhost", "Office", "socks5://proxy.example.com:1080 · anna",
		"Add Proxy", "The Server's Requests", "Plugin Catalogs", "Images", "No proxy")

	// Testing the system proxy.
	h.click("Test")
	want("Reached in 142 ms · Exit address 203.0.113.7 · Japan")

	// Adding one.
	h.click("Add Proxy")
	ps.draft.name, ps.draft.host, ps.draft.port = "Lab", "192.168.1.2", 8888
	h.settle()
	h.save(shots, "settings-proxies")
	h.click("Save")
	want("Lab", "http://192.168.1.2:8888")
	if n := len(proxies()); n != 3 {
		t.Errorf("len(proxies) = %d, want = 3", n)
	}

	// Editing Office, then deleting it.
	h.click("Office")
	if ps.editing == nil || *ps.editing != "p1" || ps.draft.kind != "socks5" || ps.draft.port != 1080 {
		t.Fatalf("editing = %v, draft = %+v, want Office's", ps.editing, ps.draft)
	}
	h.click("Delete")
	want("Delete Office?")
	h.tt.Key(0, ui.KeyEnter)
	h.settle()
	if h.tt.HasText("Office") {
		t.Errorf("Office was not deleted")
	}
}
