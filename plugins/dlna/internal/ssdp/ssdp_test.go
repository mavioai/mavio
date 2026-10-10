package ssdp

import (
	"context"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestMessageRoundTrip(t *testing.T) {
	m := Message{Method: "NOTIFY", Header: http.Header{
		"Host": {"239.255.255.250:1900"}, "Nt": {"upnp:rootdevice"}, "Nts": {"ssdp:alive"},
		"Ext": {""}, "X-Other": {"1"},
	}}
	b := string(m.Bytes())
	want := "NOTIFY * HTTP/1.1\r\nHOST: 239.255.255.250:1900\r\nEXT:\r\nNT: upnp:rootdevice\r\nNTS: ssdp:alive\r\nX-OTHER: 1\r\n\r\n"
	if b != want {
		t.Errorf("Bytes() = %q, want = %q", b, want)
	}
	got, err := Parse([]byte(b))
	if err != nil {
		t.Fatal(err)
	}
	if got.Method != "NOTIFY" || got.Header.Get("Nts") != "ssdp:alive" || got.Header.Get("x-other") != "1" {
		t.Errorf("Parse() = %+v", got)
	}
}

func TestParse(t *testing.T) {
	for _, tt := range []struct {
		name, in string
		method   string
		status   int
		st       string
		wantErr  bool
	}{
		{"search", "M-SEARCH * HTTP/1.1\r\nST: ssdp:all\r\nMAN: \"ssdp:discover\"\r\n\r\n", "M-SEARCH", 0, "ssdp:all", false},
		{"bare line feeds", "HTTP/1.1 200 OK\nst:upnp:rootdevice\n", "", 200, "upnp:rootdevice", false},
		{"bad start", "GET / HTTP/1.1\r\n\r\n", "", 0, "", true},
		{"bad header", "HTTP/1.1 200 OK\r\nnonsense\r\n\r\n", "", 0, "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m, err := Parse([]byte(tt.in))
			if (err != nil) != tt.wantErr {
				t.Fatalf("Parse() error = %v, want error = %v", err, tt.wantErr)
			}
			if err == nil && (m.Method != tt.method || m.Status != tt.status || m.Header.Get("St") != tt.st) {
				t.Errorf("Parse() = %+v, want = %s %d %s", m, tt.method, tt.status, tt.st)
			}
		})
	}
}

const (
	udn       = "uuid:5b8a3b2e-0000-4000-8000-000000000001"
	mediaSrv  = "urn:schemas-upnp-org:device:MediaServer:1"
	directory = "urn:schemas-upnp-org:service:ContentDirectory:2"
)

func TestAnswers(t *testing.T) {
	s := &Server{
		Adverts:  Adverts(udn, mediaSrv, directory),
		Location: func(ip net.IP) string { return "http://" + ip.String() + "/desc.xml" },
		Interval: 30 * time.Minute,
	}
	for _, tt := range []struct {
		st   string
		want []string // the USNs answered with
	}{
		{"ssdp:all", []string{udn + "::upnp:rootdevice", udn, udn + "::" + mediaSrv, udn + "::" + directory}},
		{"upnp:rootdevice", []string{udn + "::upnp:rootdevice"}},
		{udn, []string{udn}},
		{mediaSrv, []string{udn + "::" + mediaSrv}},
		// An earlier version is answered with that version.
		{"urn:schemas-upnp-org:service:ContentDirectory:1", []string{udn + "::urn:schemas-upnp-org:service:ContentDirectory:1"}},
		{"urn:schemas-upnp-org:service:ContentDirectory:3", nil},
		{"urn:schemas-upnp-org:service:AVTransport:1", nil},
	} {
		t.Run(tt.st, func(t *testing.T) {
			var got []string
			for _, m := range s.Answers(tt.st, net.IPv4(192, 168, 1, 2)) {
				got = append(got, m.Header.Get("Usn"))
				if l := m.Header.Get("Location"); l != "http://192.168.1.2/desc.xml" {
					t.Errorf("location = %q", l)
				}
				if c := m.Header.Get("Cache-Control"); c != "max-age=3600" {
					t.Errorf("cache control = %q", c)
				}
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("USNs = %v, want = %v", got, tt.want)
			}
		})
	}
}

// loopback returns the loopback interface, skipping the test when there is
// none.
func loopback(t *testing.T) []net.Interface {
	t.Helper()
	all, err := net.Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	for _, ifi := range all {
		if ifi.Flags&net.FlagLoopback != 0 && ifi.Flags&net.FlagUp != 0 {
			ifaces, err := Interfaces([]string{ifi.Name})
			if err != nil {
				t.Skipf("loopback: %v", err)
			}
			return ifaces
		}
	}
	t.Skip("no loopback interface")
	return nil
}

// TestRunAndSearch announces devices on a spare port and finds them, over
// multicast on the loopback interface.
func TestRunAndSearch(t *testing.T) {
	ifaces := loopback(t)
	probe, err := net.ListenPacket("udp4", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	port := probe.LocalAddr().(*net.UDPAddr).Port
	probe.Close()

	var mu sync.Mutex
	var notified []Message
	other := &Server{
		Adverts:    Adverts("uuid:other", mediaSrv),
		Location:   func(ip net.IP) string { return "http://" + ip.String() + "/other.xml" },
		Interfaces: ifaces, Interval: time.Hour, Port: port,
	}
	s := &Server{
		Adverts:    Adverts(udn, mediaSrv, directory),
		Location:   func(ip net.IP) string { return "http://" + ip.String() + "/desc.xml" },
		Product:    "test/1 UPnP/1.0 mavio/1",
		Interfaces: ifaces, Interval: time.Hour, Port: port,
		OnNotify: func(m Message, _ *net.UDPAddr) {
			mu.Lock()
			defer mu.Unlock()
			notified = append(notified, m)
		},
	}
	ctx := t.Context()
	errs := make(chan error, 2)
	go func() { errs <- s.Run(ctx) }()
	ctx2, cancel := context.WithCancel(ctx)
	go func() { errs <- other.Run(ctx2) }()

	var found []Message
	deadline := time.Now().Add(20 * time.Second)
	for len(found) == 0 && time.Now().Before(deadline) {
		err := Search(ctx, ifaces, directory, port, 2*time.Second, func(m Message, _ *net.UDPAddr) {
			if m.Header.Get("Usn") == udn+"::"+directory {
				found = append(found, m)
			}
		})
		if err != nil {
			t.Skipf("Search() = %v; multicast is unavailable", err)
		}
	}
	if len(found) == 0 {
		t.Skip("found nothing; multicast does not loop back here")
	}
	if l := found[0].Header.Get("Location"); !strings.HasPrefix(l, "http://") || !strings.HasSuffix(l, "/desc.xml") {
		t.Errorf("location = %q", l)
	}

	// The other device says goodbye when it stops; this one hears it and
	// not its own announcements.
	cancel()
	if err := <-errs; err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		bye := slices.ContainsFunc(notified, func(m Message) bool { return m.Header.Get("Nts") == "ssdp:byebye" })
		mu.Unlock()
		if bye {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	for _, m := range notified {
		if strings.HasPrefix(m.Header.Get("Usn"), udn) {
			t.Errorf("heard its own announcement %v", m.Header)
		}
	}
	if !slices.ContainsFunc(notified, func(m Message) bool { return m.Header.Get("Nts") == "ssdp:byebye" }) {
		t.Errorf("notified = %d messages, want = a goodbye among them", len(notified))
	}
}
