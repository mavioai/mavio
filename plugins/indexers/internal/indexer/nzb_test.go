package indexer

import (
	"errors"
	"strings"
	"testing"
)

// The NZBs of Radarr's and Sonarr's NzbValidationServiceFixture
// (src/NzbDrone.Core.Test/Files/Nzbs), shortened to one file.
const (
	nzbHeader  = "\xef\xbb\xbf<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"
	nzbDoctype = "<!DOCTYPE nzb\n  PUBLIC '-//newzBin//DTD NZB 1.1//EN'\n  'http://www.newzbin.com/DTD/nzb/nzb-1.1.dtd'>\n"
	nzbFile    = `  <file date="1504571104" poster="autom@gical.tv (M@GiC)" subject="[432278]-[FULL]-[#a.b.teevee@EFNet]-[ Series.Title.S04E06.720p.HDTV.x264-KILLERS ]-[01/46] - &quot;Series.Title.s04e06.720p.hdtv.x264-killers.nfo&quot; yEnc (1/1)">
    <groups>
      <group>alt.binaries.teevee</group>
    </groups>
    <segments>
      <segment bytes="12053" number="1">ZQ9h749E781168561i4J0Q6-01m6Q3185@2894t-767038L.Pg7769</segment>
    </segments>
  </file>
`
)

func TestValidateNZB(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		wantErr string
	}{
		{name: "should_throw_on_invalid_nzb", data: nzbHeader + "<fail>\n" + nzbFile + "</fail>\n", wantErr: "expected 'nzb' found 'fail'"},
		{name: "should_throw_when_no_files", data: nzbHeader + nzbDoctype + "<nzb xmlns=\"http://www.newzbin.com/DTD/2003/nzb\">\n</nzb>\n", wantErr: "no files"},
		{name: "should_throw_on_newznab_error", data: nzbHeader + "<error code=\"201\" description=\"Incorrect parameter\" />\n", wantErr: "201 - Incorrect parameter"},
		{name: "should_validate_nzb", data: nzbHeader + nzbDoctype + "<nzb xmlns=\"http://www.newzbin.com/DTD/2003/nzb\">\n" + nzbFile + "</nzb>\n"},
		{name: "not_xml", data: "Request limit reached", wantErr: "no root element"},
		{name: "malformed", data: "<nzb><file></nzb>", wantErr: "unable to parse"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNZB([]byte(tt.data))
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateNZB() = %v, want = nil", err)
				}
				return
			}
			if !errors.Is(err, ErrInvalidNZB) || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("ValidateNZB() = %v, want = an invalid NZB error containing %q", err, tt.wantErr)
			}
		})
	}
}
