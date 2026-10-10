// Package ssdp implements the Simple Service Discovery Protocol of UPnP
// 1.0 over IPv4: a server announcing a device and answering searches for
// it on chosen interfaces, and a client searching for devices.
package ssdp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/ipv4"
)

// Group is SSDP's IPv4 multicast group.
var Group = net.IPv4(239, 255, 255, 250)

// Port is SSDP's port.
const Port = 1900

// Advert is what a device announces: a notification type, and the unique
// service name of the device's or service's instance.
type Advert struct{ Type, USN string }

// Adverts returns the adverts of a root device of a type and its services:
// "upnp:rootdevice", its UDN ("uuid:…"), its type and each service type.
func Adverts(udn, deviceType string, serviceTypes ...string) []Advert {
	a := []Advert{{"upnp:rootdevice", udn + "::upnp:rootdevice"}, {udn, udn}, {deviceType, udn + "::" + deviceType}}
	for _, s := range serviceTypes {
		a = append(a, Advert{s, udn + "::" + s})
	}
	return a
}

// Interfaces returns the interfaces named, or when none are, every
// interface that is up, takes multicast and has an IPv4 address, loopback
// interfaces aside.
func Interfaces(names []string) ([]net.Interface, error) {
	all, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var out []net.Interface
	for _, ifi := range all {
		switch {
		case len(names) > 0:
			if !slices.Contains(names, ifi.Name) {
				continue
			}
		case ifi.Flags&net.FlagUp == 0, ifi.Flags&net.FlagMulticast == 0, ifi.Flags&net.FlagLoopback != 0:
			continue
		}
		if ipv4Of(&ifi) != nil {
			out = append(out, ifi)
		}
	}
	if len(names) > 0 && len(out) < len(names) {
		return out, fmt.Errorf("interfaces %q: %d of them are up with an IPv4 address", names, len(out))
	}
	return out, nil
}

// ipv4Of returns an interface's first IPv4 address, nil when it has none.
func ipv4Of(ifi *net.Interface) net.IP {
	addrs, err := ifi.Addrs()
	if err != nil {
		return nil
	}
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil {
			return n.IP.To4()
		}
	}
	return nil
}

// localIP returns the address of this host that reaches to.
func localIP(to *net.UDPAddr) net.IP {
	c, err := net.DialUDP("udp4", nil, to)
	if err != nil {
		return nil
	}
	defer c.Close()
	return c.LocalAddr().(*net.UDPAddr).IP
}

// Server announces a device and answers searches for it.
type Server struct {
	Adverts []Advert
	// Location returns the URL of the device's description as reached
	// through a local address.
	Location func(local net.IP) string
	// Product is the SERVER header: "OS/version UPnP/1.0 product/version".
	Product    string
	Interfaces []net.Interface
	// Interval is how often the device is announced; adverts are valid
	// for twice as long.
	Interval time.Duration
	// OnNotify, when set, receives the announcements of other devices.
	OnNotify func(m Message, from *net.UDPAddr)
	// Port is the port listened and announced on; 0 means Port.
	Port   int
	Logger *slog.Logger
}

func (s *Server) port() int {
	if s.Port == 0 {
		return Port
	}
	return s.Port
}

func (s *Server) group() *net.UDPAddr { return &net.UDPAddr{IP: Group, Port: s.port()} }

func (s *Server) logger() *slog.Logger {
	if s.Logger == nil {
		return slog.New(slog.DiscardHandler)
	}
	return s.Logger
}

// Run announces the device on every interface and answers searches until
// ctx ends, then says goodbye.
func (s *Server) Run(ctx context.Context) error {
	conn, err := listen(ctx, s.port())
	if err != nil {
		return fmt.Errorf("listen on port %d: %w", s.port(), err)
	}
	defer conn.Close()
	p := ipv4.NewPacketConn(conn)
	var joined []net.Interface
	for _, ifi := range s.Interfaces {
		if err := p.JoinGroup(&ifi, &net.UDPAddr{IP: Group}); err != nil {
			s.logger().WarnContext(ctx, "joining the SSDP group failed", "interface", ifi.Name, "err", err)
			continue
		}
		joined = append(joined, ifi)
	}
	if len(joined) == 0 {
		return errors.New("joined the SSDP group on no interface")
	}
	sender, err := newSender()
	if err != nil {
		return err
	}
	defer sender.Close()

	var wg sync.WaitGroup
	defer wg.Wait()
	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()
	wg.Go(func() { s.serve(ctx, conn) })

	s.notify(ctx, sender, joined, "ssdp:alive")
	t := time.NewTicker(s.Interval)
	defer t.Stop()
	// A second announcement soon after the first makes up for a lost one.
	again := time.After(time.Second)
	for {
		select {
		case <-ctx.Done():
			s.notify(context.WithoutCancel(ctx), sender, joined, "ssdp:byebye")
			return nil
		case <-again:
			s.notify(ctx, sender, joined, "ssdp:alive")
		case <-t.C:
			s.notify(ctx, sender, joined, "ssdp:alive")
		}
	}
}

// serve reads datagrams until the connection closes.
func (s *Server) serve(ctx context.Context, conn net.PacketConn) {
	buf := make([]byte, 8192)
	var wg sync.WaitGroup
	defer wg.Wait()
	for {
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			return
		}
		from, ok := addr.(*net.UDPAddr)
		if !ok {
			continue
		}
		m, err := Parse(buf[:n])
		if err != nil {
			continue
		}
		switch m.Method {
		case "M-SEARCH":
			if strings.Trim(m.Header.Get("Man"), `"`) != "ssdp:discover" {
				continue
			}
			wg.Go(func() { s.answer(ctx, conn, m, from) })
		case "NOTIFY":
			if s.OnNotify != nil && !s.ours(m.Header.Get("Usn")) {
				s.OnNotify(m, from)
			}
		}
	}
}

func (s *Server) ours(usn string) bool {
	return slices.ContainsFunc(s.Adverts, func(a Advert) bool { return a.USN == usn })
}

// Answers returns the responses to a search target, local being the
// address the searcher reaches this host at.
func (s *Server) Answers(st string, local net.IP) []Message {
	var out []Message
	for _, a := range s.Adverts {
		target, ok := matches(st, a.Type)
		if !ok {
			continue
		}
		out = append(out, Message{Status: http.StatusOK, Header: http.Header{
			"Cache-Control": {s.maxAge()},
			"Date":          {time.Now().UTC().Format(http.TimeFormat)},
			"Ext":           {""},
			"Location":      {s.Location(local)},
			"Server":        {s.Product},
			"St":            {target},
			"Usn":           {strings.Replace(a.USN, "::"+a.Type, "::"+target, 1)},
		}})
	}
	return out
}

// answer responds to a search after a random delay of up to its MX
// seconds, at most five.
func (s *Server) answer(ctx context.Context, conn net.PacketConn, m Message, from *net.UDPAddr) {
	local := localIP(from)
	if local == nil {
		return
	}
	answers := s.Answers(m.Header.Get("St"), local)
	if len(answers) == 0 {
		return
	}
	mx, err := strconv.Atoi(m.Header.Get("Mx"))
	if err != nil || mx < 1 {
		mx = 1
	}
	delay := rand.N(time.Duration(min(mx, 5)) * time.Second / 2)
	select {
	case <-ctx.Done():
		return
	case <-time.After(delay):
	}
	for _, a := range answers {
		if _, err := conn.WriteTo(a.Bytes(), from); err != nil {
			s.logger().DebugContext(ctx, "answering a search failed", "to", from, "err", err)
			return
		}
	}
}

// matches reports whether a search target asks for an advertised type,
// and the type to answer with: a search for an earlier version of a type
// is answered with that version.
func matches(st, typ string) (string, bool) {
	switch {
	case st == "ssdp:all", st == typ:
		return typ, true
	case !strings.HasPrefix(st, "urn:"):
		return "", false
	}
	i, j := strings.LastIndexByte(st, ':'), strings.LastIndexByte(typ, ':')
	if i < 0 || j < 0 || st[:i] != typ[:j] {
		return "", false
	}
	want, err1 := strconv.Atoi(st[i+1:])
	have, err2 := strconv.Atoi(typ[j+1:])
	if err1 != nil || err2 != nil || want > have {
		return "", false
	}
	return st, true
}

func (s *Server) maxAge() string { return "max-age=" + strconv.Itoa(int(2*s.Interval/time.Second)) }

// notify announces every advert on every interface.
func (s *Server) notify(ctx context.Context, sender *ipv4.PacketConn, ifaces []net.Interface, nts string) {
	for _, ifi := range ifaces {
		local := ipv4Of(&ifi)
		if local == nil {
			continue
		}
		if err := sender.SetMulticastInterface(&ifi); err != nil {
			s.logger().DebugContext(ctx, "announcing failed", "interface", ifi.Name, "err", err)
			continue
		}
		for _, a := range s.Adverts {
			h := http.Header{
				"Host": {s.group().String()}, "Nt": {a.Type}, "Nts": {nts}, "Usn": {a.USN},
			}
			if nts == "ssdp:alive" {
				h["Cache-Control"] = []string{s.maxAge()}
				h["Location"] = []string{s.Location(local)}
				h["Server"] = []string{s.Product}
			}
			msg := Message{Method: "NOTIFY", Header: h}
			if _, err := sender.WriteTo(msg.Bytes(), nil, s.group()); err != nil {
				s.logger().DebugContext(ctx, "announcing failed", "interface", ifi.Name, "err", err)
				break
			}
		}
	}
}

// newSender returns a socket sending to multicast groups.
func newSender() (*ipv4.PacketConn, error) {
	conn, err := net.ListenPacket("udp4", "0.0.0.0:0")
	if err != nil {
		return nil, err
	}
	p := ipv4.NewPacketConn(conn)
	// UPnP 1.0 suggests a TTL of 4; looping back lets other programs on
	// this host find the device.
	if err := p.SetMulticastTTL(4); err != nil {
		conn.Close()
		return nil, err
	}
	if err := p.SetMulticastLoopback(true); err != nil {
		conn.Close()
		return nil, err
	}
	return p, nil
}

// Search multicasts a search for a target on every interface and calls
// found with each response until wait passes or ctx ends; port 0 means
// Port.
func Search(ctx context.Context, ifaces []net.Interface, st string, port int, wait time.Duration, found func(m Message, from *net.UDPAddr)) error {
	if port == 0 {
		port = Port
	}
	sender, err := newSender()
	if err != nil {
		return err
	}
	defer sender.Close()
	group := &net.UDPAddr{IP: Group, Port: port}
	mx := max(1, int(wait/time.Second)-1)
	msg := Message{Method: "M-SEARCH", Header: http.Header{
		"Host": {group.String()}, "Man": {`"ssdp:discover"`}, "Mx": {strconv.Itoa(mx)}, "St": {st},
	}}
	sent := 0
	for _, ifi := range ifaces {
		if err := sender.SetMulticastInterface(&ifi); err != nil {
			continue
		}
		if _, err := sender.WriteTo(msg.Bytes(), nil, group); err == nil {
			sent++
		}
	}
	if sent == 0 {
		return errors.New("searched on no interface")
	}
	deadline := time.Now().Add(wait)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err := sender.SetReadDeadline(deadline); err != nil {
		return err
	}
	stop := context.AfterFunc(ctx, func() { _ = sender.SetReadDeadline(time.Now()) })
	defer stop()
	buf := make([]byte, 8192)
	for {
		n, _, addr, err := sender.ReadFrom(buf)
		if err != nil {
			if ne, ok := errors.AsType[net.Error](err); ok && ne.Timeout() {
				return ctx.Err()
			}
			return err
		}
		from, ok := addr.(*net.UDPAddr)
		if !ok {
			continue
		}
		if m, err := Parse(buf[:n]); err == nil && m.Status == http.StatusOK {
			found(m, from)
		}
	}
}
