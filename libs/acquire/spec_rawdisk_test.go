package acquire

import "testing"

// TestRawDiskSpec ports Radarr's RawDiskSpecificationFixture.
func TestRawDiskSpec(t *testing.T) {
	tests := []struct {
		name, title, container string
		want                   bool
	}{
		{"should_return_true_if_no_container_specified", "Movie.title.1998", "", true},
		{"should_return_true_if_mkv", "Movie.title.1998", "MKV", true},
		{"should_return_false_if_vob", "Movie.title.1998", "VOB", false},
		{"should_return_false_if_iso", "Movie.title.1998", "ISO", false},
		{"should_return_false_if_m2ts", "Movie.title.1998", "M2TS", false},
		{"should_compare_case_insensitive", "Movie.title.1998", "vob", false},
	}
	for _, title := range []string{
		"Series Title S02 Disc 1 1080i Blu-ray DTS-HD MA 2.0 AVC-TrollHD",
		"Series Title S03 Disc 1 1080p Blu-ray LPCM 2.0 AVC-TrollHD",
		"SERIES TITLE S02 1080P FULL BLURAY AVC DTS-HD MA 5 1",
		"Series.Title.S06.2016.DISC.3.BluRay.1080p.AVC.Atmos.TrueHD7.1-MTeam",
		"Series Title S05 Disc 1 BluRay 1080p AVC Atmos TrueHD 7 1-MTeam",
		// Radarr lists this title twice.
		"Series Title S05 Disc 1 BluRay 1080p AVC Atmos TrueHD 7 1-MTeam",
		"Someone.the.Entertainer.Presents.S01.NTSC.3xDVD9.MPEG-2.DD2.0",
		"Series.Title.S00.The.Christmas.Special.2011.PAL.DVD5.DD2.0",
		"Series.of.Desire.2000.S1_D01.NTSC.DVD5",
	} {
		tests = append(tests, struct {
			name, title, container string
			want                   bool
		}{"should_return_false_if_matches_disc_format(" + title + ")", title, "", false})
	}
	for _, title := range []string{
		"Series Title EP50 USLT NTSC DVDRemux DD2.0",
		"Series.Title.S01.NTSC.DVDRip.DD2.0.x264-PLAiD",
	} {
		tests = append(tests, struct {
			name, title, container string
			want                   bool
		}{"should_return_true_if_dvdrip(" + title + ")", title, "", true})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _ := newTestEnv()
			c := spCandidate()
			c.Release.Title = tt.title
			c.Release.Protocol = ProtocolTorrent
			c.Release.Container = tt.container
			spWant(t, RawDisk{}, e, c, nil, tt.want)
		})
	}
}
