package zone

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/miekg/dns"
)

const id = "abcdefghijklmnopqrstuvwxyz234567"

func handler(t *testing.T) *Handler {
	t.Helper()
	c := &Config{
		Zone: "s.mavio.fyi", Hostmaster: "hostmaster.mavio.dev", ACME: "acme.mavio.dev", PSL: "https://github.com/publicsuffix/list/pull/1",
		NameServers: []Host{
			{Name: "ns1.s.mavio.fyi", Addresses: []netip.Addr{netip.MustParseAddr("198.51.100.1"), netip.MustParseAddr("2001:db8::53")}},
			{Name: "ns2.s.mavio.fyi", Addresses: []netip.Addr{netip.MustParseAddr("198.51.100.2")}},
		},
		CAA:    []string{"letsencrypt.org", "pki.goog"},
		Relays: map[string][]netip.Addr{"eu": {netip.MustParseAddr("203.0.113.7"), netip.MustParseAddr("2001:db8::7")}},
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	return New(c)
}

func TestAnswer(t *testing.T) {
	h := handler(t)
	tests := []struct {
		name  string
		qtype uint16
		rcode int
		want  []string // the answers' data
	}{
		{"192-168-1-10." + id + ".s.mavio.fyi.", dns.TypeA, dns.RcodeSuccess, []string{"192.168.1.10"}},
		{"192-168-1-10." + strings.ToUpper(id) + ".S.MAVIO.fyi.", dns.TypeA, dns.RcodeSuccess, []string{"192.168.1.10"}},
		{"192-168-1-10." + id + ".s.mavio.fyi.", dns.TypeAAAA, dns.RcodeSuccess, nil},
		{"2001-db8--1." + id + ".s.mavio.fyi.", dns.TypeAAAA, dns.RcodeSuccess, []string{"2001:db8::1"}},
		{"2001-db8--1." + id + ".s.mavio.fyi.", dns.TypeA, dns.RcodeSuccess, nil},
		{"--ffff-c000-201." + id + ".s.mavio.fyi.", dns.TypeAAAA, dns.RcodeNameError, nil},
		{"192-168-1-300." + id + ".s.mavio.fyi.", dns.TypeA, dns.RcodeNameError, nil},
		{"192-168-1-10.short.s.mavio.fyi.", dns.TypeA, dns.RcodeNameError, nil},
		{"a.192-168-1-10." + id + ".s.mavio.fyi.", dns.TypeA, dns.RcodeNameError, nil},
		{id + ".s.mavio.fyi.", dns.TypeA, dns.RcodeSuccess, nil},
		{"relay-eu-t1ck3t." + id + ".s.mavio.fyi.", dns.TypeA, dns.RcodeSuccess, []string{"203.0.113.7"}},
		{"relay-eu-t1ck3t." + id + ".s.mavio.fyi.", dns.TypeAAAA, dns.RcodeSuccess, []string{"2001:db8::7"}},
		{"relay-us-t1ck3t." + id + ".s.mavio.fyi.", dns.TypeA, dns.RcodeNameError, nil},
		{"relay-eu." + id + ".s.mavio.fyi.", dns.TypeA, dns.RcodeNameError, nil},
		{"_acme-challenge." + id + ".s.mavio.fyi.", dns.TypeTXT, dns.RcodeSuccess, []string{id + ".acme.mavio.dev."}},
		{"s.mavio.fyi.", dns.TypeNS, dns.RcodeSuccess, []string{"ns1.s.mavio.fyi.", "ns2.s.mavio.fyi."}},
		{"s.mavio.fyi.", dns.TypeSOA, dns.RcodeSuccess, []string{"ns1.s.mavio.fyi. hostmaster.mavio.dev. 0 7200 3600 1209600 300"}},
		{"s.mavio.fyi.", dns.TypeCAA, dns.RcodeSuccess, []string{
			`0 issue "letsencrypt.org"`, `0 issuewild "letsencrypt.org"`, `0 issue "pki.goog"`, `0 issuewild "pki.goog"`,
		}},
		{"s.mavio.fyi.", dns.TypeA, dns.RcodeSuccess, nil},
		{"_psl.s.mavio.fyi.", dns.TypeTXT, dns.RcodeSuccess, []string{`"https://github.com/publicsuffix/list/pull/1"`}},
		{"ns1.s.mavio.fyi.", dns.TypeAAAA, dns.RcodeSuccess, []string{"2001:db8::53"}},
		{"www.s.mavio.fyi.", dns.TypeA, dns.RcodeNameError, nil},
		{"mavio.dev.", dns.TypeA, dns.RcodeRefused, nil},
	}
	for _, tt := range tests {
		t.Run(dns.TypeToString[tt.qtype]+" "+tt.name, func(t *testing.T) {
			req := new(dns.Msg)
			req.SetQuestion(tt.name, tt.qtype)
			m := h.Answer(req, "udp")
			var got []string
			for _, rr := range m.Answer {
				if rr.Header().Name != tt.name {
					t.Errorf("owner = %q, want = %q", rr.Header().Name, tt.name)
				}
				got = append(got, strings.TrimPrefix(rr.String(), rr.Header().String()))
			}
			if m.Rcode != tt.rcode || strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Errorf("got = %s %q, want = %s %q", dns.RcodeToString[m.Rcode], got, dns.RcodeToString[tt.rcode], tt.want)
			}
			if tt.rcode != dns.RcodeRefused && (!m.Authoritative || (len(got) == 0 && len(m.Ns) != 1)) {
				t.Errorf("authoritative = %v, authority = %v; want authoritative, with the SOA when empty", m.Authoritative, m.Ns)
			}
		})
	}
}

func TestGlueAndEDNS(t *testing.T) {
	h := handler(t)
	req := new(dns.Msg)
	req.SetQuestion("s.mavio.fyi.", dns.TypeNS)
	req.SetEdns0(4096, false)
	m := h.Answer(req, "udp")
	if len(m.Extra) != 4 || m.IsEdns0() == nil {
		t.Errorf("extra = %v, want three glue records and OPT", m.Extra)
	}
}

func TestValidate(t *testing.T) {
	for name, c := range map[string]Config{
		"no zone":             {ACME: "a", Hostmaster: "h", NameServers: []Host{{Name: "ns.x"}}},
		"no name servers":     {Zone: "x", ACME: "a", Hostmaster: "h"},
		"outside name server": {Zone: "x", ACME: "a", Hostmaster: "h", NameServers: []Host{{Name: "ns.y", Addresses: []netip.Addr{netip.MustParseAddr("192.0.2.1")}}}},
		"dashed region": {
			Zone: "x", ACME: "a", Hostmaster: "h", NameServers: []Host{{Name: "ns.x", Addresses: []netip.Addr{netip.MustParseAddr("192.0.2.1")}}},
			Relays: map[string][]netip.Addr{"us-east": nil},
		},
	} {
		if err := c.Validate(); err == nil {
			t.Errorf("%s: Validate() = nil, want an error", name)
		}
	}
}
