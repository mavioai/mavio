package zone

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strings"

	"github.com/miekg/dns"
)

// Config describes the zone.
type Config struct {
	// Zone is the domain served, such as mavio.site.
	Zone string `json:"zone"`
	// NameServers are the zone's name servers, under the zone.
	NameServers []Host `json:"nameServers"`
	// Hostmaster is the SOA's mailbox, such as hostmaster.mavio.dev.
	Hostmaster string `json:"hostmaster"`
	// Serial is the SOA's serial, changed with the static records.
	Serial uint32 `json:"serial"`
	// ACME is the domain the challenges' CNAMEs point under, such as
	// acme.mavio.dev.
	ACME string `json:"acme"`
	// PSL is the value of the _psl TXT record, the Public Suffix List's
	// pull request adding the zone.
	PSL string `json:"psl"`
	// CAA are the CAs allowed to issue for the zone, such as
	// letsencrypt.org.
	CAA []string `json:"caa"`
	// Relays are the relays' addresses by region (§5).
	Relays map[string][]netip.Addr `json:"relays"`
	// TTL is the TTL of computed and static records, RelayTTL that of
	// relay names; zero is an hour and five minutes.
	TTL      uint32 `json:"ttl"`
	RelayTTL uint32 `json:"relayTTL"`
}

// Host is a name with its addresses.
type Host struct {
	Name      string       `json:"name"`
	Addresses []netip.Addr `json:"addresses"`
}

// Load reads a JSON configuration file.
func Load(file string) (*Config, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("read the configuration: %w", err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return &c, c.Validate()
}

// Validate checks the configuration and fills in defaults.
func (c *Config) Validate() error {
	c.Zone = canonical(c.Zone)
	c.ACME, c.Hostmaster = canonical(c.ACME), canonical(c.Hostmaster)
	switch {
	case c.Zone == "." || c.ACME == "." || c.Hostmaster == ".":
		return errors.New("zone, acme and hostmaster are required")
	case len(c.NameServers) == 0:
		return errors.New("at least one name server is required")
	}
	for i, ns := range c.NameServers {
		c.NameServers[i].Name = canonical(ns.Name)
		if !dns.IsSubDomain(c.Zone, c.NameServers[i].Name) || len(ns.Addresses) == 0 {
			return fmt.Errorf("name server %q must be under %s with addresses", ns.Name, c.Zone)
		}
	}
	for region := range c.Relays {
		if region == "" || strings.ContainsAny(region, "-.") {
			return fmt.Errorf("relay region %q must be a label without dashes", region)
		}
	}
	if c.TTL == 0 {
		c.TTL = 3600
	}
	if c.RelayTTL == 0 {
		c.RelayTTL = 300
	}
	return nil
}

func canonical(name string) string { return dns.CanonicalName(name) }

// Handler answers queries for the zone.
type Handler struct{ c *Config }

// New returns a handler of a validated configuration.
func New(c *Config) *Handler { return &Handler{c: c} }

// ServeDNS answers one query.
func (h *Handler) ServeDNS(w dns.ResponseWriter, req *dns.Msg) {
	_ = w.WriteMsg(h.Answer(req, w.RemoteAddr().Network()))
}

// Answer returns the response to req, received over network ("udp" or
// "tcp"), truncated to fit a UDP response.
func (h *Handler) Answer(req *dns.Msg, network string) *dns.Msg {
	m := new(dns.Msg)
	m.SetReply(req)
	if req.Opcode != dns.OpcodeQuery || len(req.Question) != 1 {
		m.SetRcode(req, dns.RcodeNotImplemented)
		return m
	}
	q := req.Question[0]
	name := canonical(q.Name)
	if q.Qclass != dns.ClassINET || !dns.IsSubDomain(h.c.Zone, name) {
		m.SetRcode(req, dns.RcodeRefused)
		return m
	}
	m.Authoritative = true
	answer, exists := h.lookup(q.Name, name, q.Qtype)
	m.Answer = answer
	switch {
	case !exists:
		m.Rcode = dns.RcodeNameError
		m.Ns = []dns.RR{h.soa()}
	case len(answer) == 0:
		m.Ns = []dns.RR{h.soa()}
	case q.Qtype == dns.TypeNS && name == h.c.Zone:
		m.Extra = h.glue()
	}
	size := dns.MinMsgSize
	if opt := req.IsEdns0(); opt != nil {
		size = max(int(opt.UDPSize()), dns.MinMsgSize)
		m.SetEdns0(1232, false)
	}
	if network == "udp" {
		m.Truncate(min(size, 1232))
	}
	return m
}

// lookup returns the records of a name and type, and whether the name
// exists; owner is the name as asked.
func (h *Handler) lookup(owner, name string, qtype uint16) ([]dns.RR, bool) {
	c := h.c
	hdr := func(t uint16, ttl uint32) dns.RR_Header {
		return dns.RR_Header{Name: owner, Rrtype: t, Class: dns.ClassINET, Ttl: ttl}
	}
	if qtype == dns.TypeANY {
		// RFC 8482: a minimal answer.
		return []dns.RR{&dns.HINFO{Hdr: hdr(dns.TypeHINFO, c.TTL), Cpu: "RFC8482"}}, true
	}
	if name == c.Zone {
		var rrs []dns.RR
		switch qtype {
		case dns.TypeSOA:
			rrs = append(rrs, h.soa())
		case dns.TypeNS:
			for _, ns := range c.NameServers {
				rrs = append(rrs, &dns.NS{Hdr: hdr(dns.TypeNS, c.TTL), Ns: ns.Name})
			}
		case dns.TypeCAA:
			for _, ca := range c.CAA {
				rrs = append(rrs, &dns.CAA{Hdr: hdr(dns.TypeCAA, c.TTL), Tag: "issue", Value: ca})
				rrs = append(rrs, &dns.CAA{Hdr: hdr(dns.TypeCAA, c.TTL), Tag: "issuewild", Value: ca})
			}
		}
		return rrs, true
	}
	if name == "_psl."+c.Zone && c.PSL != "" {
		if qtype == dns.TypeTXT {
			return []dns.RR{&dns.TXT{Hdr: hdr(dns.TypeTXT, c.TTL), Txt: []string{c.PSL}}}, true
		}
		return nil, true
	}
	for _, ns := range c.NameServers {
		if name == ns.Name {
			return addresses(ns.Addresses, qtype, hdr, c.TTL), true
		}
	}
	labels := dns.SplitDomainName(strings.TrimSuffix(name, "."+c.Zone))
	if len(labels) == 0 || len(labels) > 2 || !isID(labels[len(labels)-1]) {
		return nil, false
	}
	id := labels[len(labels)-1]
	if len(labels) == 1 {
		return nil, true // a server's own name has no records
	}
	label := labels[0]
	if label == "_acme-challenge" {
		return []dns.RR{&dns.CNAME{Hdr: hdr(dns.TypeCNAME, c.TTL), Target: id + "." + c.ACME}}, true
	}
	if rest, ok := strings.CutPrefix(label, "relay-"); ok {
		region, ticket, ok := strings.Cut(rest, "-")
		addrs, known := c.Relays[region]
		if !ok || !known || ticket == "" {
			return nil, false
		}
		return addresses(addrs, qtype, hdr, c.RelayTTL), true
	}
	addr, ok := parseAddress(label)
	if !ok {
		return nil, false
	}
	return addresses([]netip.Addr{addr}, qtype, hdr, c.TTL), true
}

func addresses(addrs []netip.Addr, qtype uint16, hdr func(uint16, uint32) dns.RR_Header, ttl uint32) []dns.RR {
	var rrs []dns.RR
	for _, a := range addrs {
		switch {
		case a.Is4() && qtype == dns.TypeA:
			rrs = append(rrs, &dns.A{Hdr: hdr(dns.TypeA, ttl), A: a.AsSlice()})
		case a.Is6() && qtype == dns.TypeAAAA:
			rrs = append(rrs, &dns.AAAA{Hdr: hdr(dns.TypeAAAA, ttl), AAAA: a.AsSlice()})
		}
	}
	return rrs
}

func (h *Handler) soa() dns.RR {
	c := h.c
	return &dns.SOA{
		Hdr: dns.RR_Header{Name: c.Zone, Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: c.TTL},
		Ns:  c.NameServers[0].Name, Mbox: c.Hostmaster, Serial: c.Serial,
		Refresh: 7200, Retry: 3600, Expire: 1209600, Minttl: 300,
	}
}

// glue returns the name servers' addresses.
func (h *Handler) glue() []dns.RR {
	var rrs []dns.RR
	for _, ns := range h.c.NameServers {
		hdr := func(t uint16, ttl uint32) dns.RR_Header {
			return dns.RR_Header{Name: ns.Name, Rrtype: t, Class: dns.ClassINET, Ttl: ttl}
		}
		rrs = append(rrs, addresses(ns.Addresses, dns.TypeA, hdr, h.c.TTL)...)
		rrs = append(rrs, addresses(ns.Addresses, dns.TypeAAAA, hdr, h.c.TTL)...)
	}
	return rrs
}

// isID reports whether a label is a server's ID: 32 characters of
// lowercase base32 (docs/accounts.md §5.1).
func isID(label string) bool {
	if len(label) != 32 {
		return false
	}
	for _, r := range label {
		if (r < 'a' || r > 'z') && (r < '2' || r > '7') {
			return false
		}
	}
	return true
}

// parseAddress reads an address label: an IPv4 address with dashes for
// dots (192-168-1-10), or an IPv6 address with dashes for colons
// (2001-db8--1).
func parseAddress(label string) (netip.Addr, bool) {
	if strings.Count(label, "-") == 3 {
		if a, err := netip.ParseAddr(strings.ReplaceAll(label, "-", ".")); err == nil && a.Is4() {
			return a, true
		}
	}
	a, err := netip.ParseAddr(strings.ReplaceAll(label, "-", ":"))
	if err != nil || !a.Is6() || a.Is4In6() || a.Zone() != "" {
		return netip.Addr{}, false
	}
	return a, true
}
