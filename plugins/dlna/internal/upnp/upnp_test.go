package upnp

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const cds = "urn:schemas-upnp-org:service:ContentDirectory:1"

func TestActionRoundTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a, err := ReadAction(r)
		if err != nil {
			t.Errorf("ReadAction() = %v", err)
			WriteFault(w, ErrInvalidAction)
			return
		}
		if a.Args["ObjectID"] == "missing" {
			WriteFault(w, ErrNoSuchObject)
			return
		}
		WriteResponse(w, a, []Arg{{"Result", a.Args["Filter"]}, {"Echo", a.Args["ObjectID"]}, {"Action", a.ServiceType + "#" + a.Name}})
	}))
	defer srv.Close()

	didl := `<DIDL-Lite xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/"><item id="1">a &amp; b</item></DIDL-Lite>`
	out, err := Call(t.Context(), srv.Client(), srv.URL, cds, "Browse", []Arg{{"ObjectID", "0 & <1>"}, {"Filter", didl}})
	if err != nil {
		t.Fatal(err)
	}
	if out["Echo"] != "0 & <1>" || out["Result"] != didl || out["Action"] != cds+"#Browse" {
		t.Errorf("Call() = %q", out)
	}
	_, err = Call(t.Context(), srv.Client(), srv.URL, cds, "Browse", []Arg{{"ObjectID", "missing"}})
	if f, ok := errors.AsType[*Fault](err); !ok || f.Code != 701 {
		t.Errorf("Call(missing) = %v, want = fault 701", err)
	}
}

func TestReadActionLenient(t *testing.T) {
	// Devices send other namespace prefixes and an unquoted SOAPACTION.
	body := `<?xml version="1.0"?><SOAP-ENV:Envelope xmlns:SOAP-ENV="http://schemas.xmlsoap.org/soap/envelope/"><SOAP-ENV:Body>` +
		`<m:GetSystemUpdateID xmlns:m="` + cds + `"/></SOAP-ENV:Body></SOAP-ENV:Envelope>`
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.Header.Set("SOAPACTION", cds+"#GetSystemUpdateID")
	a, err := ReadAction(r)
	if err != nil || a.Name != "GetSystemUpdateID" || a.ServiceType != cds || len(a.Args) != 0 {
		t.Errorf("ReadAction() = %+v, %v", a, err)
	}
	r = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.Header.Set("SOAPACTION", `"`+cds+`#Browse"`)
	if _, err := ReadAction(r); err == nil {
		t.Error("ReadAction(mismatched action) = nil error")
	}
}

func TestDescription(t *testing.T) {
	desc := `<?xml version="1.0"?><root xmlns="urn:schemas-upnp-org:device-1-0"><specVersion><major>1</major><minor>0</minor></specVersion>
<device><deviceType>urn:schemas-upnp-org:device:MediaRenderer:1</deviceType><friendlyName>TV</friendlyName><manufacturer>ACME</manufacturer>
<modelName>X1</modelName><UDN>uuid:tv</UDN><serviceList>
<service><serviceType>urn:schemas-upnp-org:service:AVTransport:1</serviceType><serviceId>urn:upnp-org:serviceId:AVTransport</serviceId>
<SCPDURL>/avt.xml</SCPDURL><controlURL>control/avt</controlURL><eventSubURL>/event/avt</eventSubURL></service>
</serviceList></device></root>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, desc) }))
	defer srv.Close()
	d, err := FetchDescription(t.Context(), srv.Client(), srv.URL+"/dev/desc.xml")
	if err != nil {
		t.Fatal(err)
	}
	dev, ok := d.Device.Find("urn:schemas-upnp-org:device:MediaRenderer:1")
	if !ok || dev.FriendlyName != "TV" || dev.UDN != "uuid:tv" {
		t.Fatalf("Find() = %+v, %v", dev, ok)
	}
	// A later version of a service serves the earlier ones.
	s, ok := dev.Service("urn:schemas-upnp-org:service:AVTransport:2")
	if !ok || s.ControlURL != srv.URL+"/dev/control/avt" || s.SCPDURL != srv.URL+"/avt.xml" {
		t.Errorf("Service() = %+v, %v", s, ok)
	}
}

func TestPublisher(t *testing.T) {
	type event struct{ sid, seq, body string }
	var mu sync.Mutex
	var events []event
	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		events = append(events, event{r.Header.Get("Sid"), r.Header.Get("Seq"), string(body)})
		mu.Unlock()
	}))
	defer sink.Close()

	p := &Publisher{MaxTimeout: time.Hour}
	h := p.Handler("cds", func() []Arg { return []Arg{{"SystemUpdateID", "1"}} })
	do := func(method string, headers map[string]string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/events/cds", nil)
		r.RemoteAddr = "127.0.0.1:5000"
		for k, v := range headers {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		h(w, r)
		return w
	}

	// Callbacks elsewhere than the subscriber are refused.
	if w := do("SUBSCRIBE", map[string]string{"Nt": "upnp:event", "Callback": "<http://192.0.2.1/x>"}); w.Code != http.StatusPreconditionFailed {
		t.Errorf("foreign callback: %d", w.Code)
	}
	w := do("SUBSCRIBE", map[string]string{"Nt": "upnp:event", "Callback": "<" + sink.URL + "/cb>", "Timeout": "Second-60"})
	sid := w.Header().Get("Sid")
	if w.Code != http.StatusOK || !strings.HasPrefix(sid, "uuid:") || w.Header().Get("Timeout") != "Second-60" {
		t.Fatalf("SUBSCRIBE: %d %v", w.Code, w.Header())
	}
	wait := func(n int) []event {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			mu.Lock()
			got := append([]event(nil), events...)
			mu.Unlock()
			if len(got) >= n {
				return got
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("got %d events, want %d", len(events), n)
		return nil
	}
	got := wait(1)
	if got[0].sid != sid || got[0].seq != "0" || !strings.Contains(got[0].body, "<SystemUpdateID>1</SystemUpdateID>") {
		t.Errorf("initial event = %+v", got[0])
	}
	p.Notify(t.Context(), "cds", []Arg{{"SystemUpdateID", "2"}})
	p.Notify(t.Context(), "other", []Arg{{"X", "1"}})
	got = wait(2)
	if len(got) != 2 || got[1].seq != "1" || !strings.Contains(got[1].body, "<SystemUpdateID>2</SystemUpdateID>") {
		t.Errorf("events = %+v", got)
	}

	if w := do("SUBSCRIBE", map[string]string{"Sid": sid, "Timeout": "Second-infinite"}); w.Code != http.StatusOK || w.Header().Get("Timeout") != "Second-3600" {
		t.Errorf("renewal: %d %v", w.Code, w.Header())
	}
	if w := do("SUBSCRIBE", map[string]string{"Sid": "uuid:unknown"}); w.Code != http.StatusPreconditionFailed {
		t.Errorf("renewal of an unknown subscription: %d", w.Code)
	}
	if w := do("UNSUBSCRIBE", map[string]string{"Sid": sid}); w.Code != http.StatusOK {
		t.Errorf("UNSUBSCRIBE: %d", w.Code)
	}
	if n := p.Subscribers("cds"); n != 0 {
		t.Errorf("subscribers = %d, want = 0", n)
	}
}

func TestPublisherExpires(t *testing.T) {
	now := time.Unix(1000, 0)
	p := &Publisher{now: func() time.Time { return now }}
	r := httptest.NewRequest("SUBSCRIBE", "/", nil)
	r.RemoteAddr = "127.0.0.1:1"
	r.Header.Set("Nt", "upnp:event")
	r.Header.Set("Callback", "<http://127.0.0.1:9/>")
	r.Header.Set("Timeout", "Second-10")
	p.Handler("cds", func() []Arg { return nil })(httptest.NewRecorder(), r)
	if n := p.Subscribers("cds"); n != 1 {
		t.Fatalf("subscribers = %d, want = 1", n)
	}
	now = now.Add(11 * time.Second)
	if n := p.Subscribers("cds"); n != 0 {
		t.Errorf("subscribers after the timeout = %d, want = 0", n)
	}
}
