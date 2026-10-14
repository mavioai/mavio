package relay

import (
	"testing"
	"time"
)

// The vector apps/website's tests share.
var (
	vectorSecret = []byte("0123456789abcdef0123456789abcdef")
	vectorID     = "abcdefghijklmnopqrstuvwxyz234567"
	vectorExp    = time.Unix(1_800_000_000, 0)
	vectorTicket = "nne5eaf3rf7i7gajowc76q6yypqckda5"
)

func TestTicket(t *testing.T) {
	if got := Ticket(vectorSecret, vectorID, "eu", vectorExp); got != vectorTicket {
		t.Errorf("Ticket = %q, want = %q", got, vectorTicket)
	}
	tests := []struct {
		name   string
		ticket string
		id     string
		region string
		now    time.Time
		want   bool
	}{
		{"valid", vectorTicket, vectorID, "eu", vectorExp.Add(-time.Minute), true},
		{"at expiry", vectorTicket, vectorID, "eu", vectorExp, true},
		{"expired", vectorTicket, vectorID, "eu", vectorExp.Add(time.Second), false},
		{"other region", vectorTicket, vectorID, "us", vectorExp.Add(-time.Minute), false},
		{"other server", vectorTicket, "bbcdefghijklmnopqrstuvwxyz234567", "eu", vectorExp.Add(-time.Minute), false},
		{"truncated", vectorTicket[:30], vectorID, "eu", vectorExp.Add(-time.Minute), false},
		{"not base32", "!" + vectorTicket[1:], vectorID, "eu", vectorExp.Add(-time.Minute), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CheckTicket(vectorSecret, tt.ticket, tt.id, tt.region, tt.now); got != tt.want {
				t.Errorf("CheckTicket = %v, want = %v", got, tt.want)
			}
		})
	}
}

func TestParseName(t *testing.T) {
	tests := []struct {
		name               string
		region, ticket, id string
		ok                 bool
	}{
		{"relay-eu-t1ck3t." + vectorID + ".mavio.site", "eu", "t1ck3t", vectorID, true},
		{"Relay-EU-T1CK3T." + vectorID + ".mavio.site.", "eu", "t1ck3t", vectorID, true},
		{"relay-eu." + vectorID + ".mavio.site", "", "", "", false},
		{"relay-eu-t1ck3t." + vectorID + ".example.com", "", "", "", false},
		{"192-0-2-1." + vectorID + ".mavio.site", "", "", "", false},
		{"relay-eu-t1ck3t.x." + vectorID + ".mavio.site", "", "", "", false},
		{"relay--t1ck3t." + vectorID + ".mavio.site", "", "", "", false},
	}
	for _, tt := range tests {
		region, ticket, id, ok := ParseName(tt.name, "mavio.site")
		if ok != tt.ok || ok && (region != tt.region || ticket != tt.ticket || id != tt.id) {
			t.Errorf("ParseName(%q) = %q, %q, %q, %v, want = %q, %q, %q, %v",
				tt.name, region, ticket, id, ok, tt.region, tt.ticket, tt.id, tt.ok)
		}
	}
}
