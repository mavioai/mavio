package sites

import (
	"testing"

	"github.com/mavioai/mavio/plugins/indexers/internal/ported"
)

func TestObsoleteCases(t *testing.T) {
	const rarbg = "Rarbg shut down on 2023-05-31; Prowlarr marks it obsolete"
	ported.Run(t, "rarbg_tests/rarbg.json", ported.Cases{Skip: map[string]string{
		"should_parse_recent_feed_from_Rarbg":    rarbg,
		"should_parse_error_20_as_empty_results": rarbg,
		"should_warn_on_unknown_error":           rarbg,
	}})
}
