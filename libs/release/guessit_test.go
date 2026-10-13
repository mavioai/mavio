package release

import (
	"slices"
	"testing"
)

func TestParseStreamingService(t *testing.T) {
	tests := []struct{ title, want string }{
		{"The.Show.S01E01.1080p.AMZN.WEB-DL.DDP5.1.H.264-NTb", "Amazon Prime"},
		{"Movie.2020.1080p.NF.WEBRip.x264-GROUP", "Netflix"},
		{"The.Show.S02E03.720p.HULU.WEB-DL.AAC2.0.H.264-GROUP", "Hulu"},
		{"Show.S01E01.2160p.ATVP.WEB-DL.DDP5.1.Atmos.HDR.H.265-GROUP", "AppleTV"},
		{"Show.S01E01.1080p.DSNP.WEB-DL.DDP5.1.H.264", "Disney+"},
		{"Show.S01E01.iT.WEB-DL.x264", "iTunes"},
		{"Show.S01E01.IT.WEB-DL.x264", ""},
		{"The.Office.S01E01.720p.BluRay.x264-GROUP", ""},
		{"Netflix.Documentary.2020.1080p.BluRay.x264", ""},
		{"Movie.Titles.2020.1080p.NF.WEB.DD2.0.x264-SNEAkY", ""},
		{"Series.S01E08.German.DD51.Dubbed.DL.720p.AmazonHD.x264-TVS", "Amazon Prime"},
		{"Series.S08E01.1080p.WEB-DL.DD5.1.H264-NFHD", ""},
	}
	for _, tt := range tests {
		if got := ParseStreamingService(tt.title); got != tt.want {
			t.Errorf("ParseStreamingService(%q): got = %q, want = %q", tt.title, got, tt.want)
		}
	}
}

func TestParseVideoCodec(t *testing.T) {
	tests := []struct{ title, want string }{
		{"Movie.2020.1080p.BluRay.x264-GROUP", "H.264"},
		{"Movie.2020.1080p.WEB-DL.H.264-GROUP", "H.264"},
		{"Movie.2020.2160p.WEB-DL.H265-GROUP", "H.265"},
		{"Movie.2020.1080p.BluRay.HEVC10-GROUP", "H.265"},
		{"Movie.2020.DVDRip.XviD-GROUP", "Xvid"},
		{"Movie.2020.1080p.BluRay.AVC.REMUX", "H.264"},
		{"Movie.2020.720p.HDTV.MPEG2", "MPEG-2"},
		{"Movie.2020.1080p.BluRay", ""},
		{"Some.Movie.S02E15.divx", ""},
		{"Some.Movie.S02E15.xvid", "Xvid"},
	}
	for _, tt := range tests {
		if got := ParseVideoCodec(tt.title); got != tt.want {
			t.Errorf("ParseVideoCodec(%q): got = %q, want = %q", tt.title, got, tt.want)
		}
	}
}

func TestParseAudioCodecs(t *testing.T) {
	tests := []struct {
		title string
		want  []string
	}{
		{"Show.S01E01.1080p.AMZN.WEB-DL.DDP5.1.H.264-NTb", []string{"Dolby Digital Plus"}},
		{"Movie.2020.1080p.BluRay.DTS-HD.MA.5.1.x264", []string{"DTS-HD"}},
		{"Movie.2020.1080p.BluRay.DTS.MA.5.1.x264", []string{"DTS-HD"}},
		{"Movie.2020.1080p.BluRay.DTS.x264", []string{"DTS"}},
		{"Movie.2020.2160p.BluRay.TrueHD.7.1.Atmos.x265", []string{"Dolby TrueHD", "Dolby Atmos"}},
		{"Movie.2020.720p.WEB.AAC2.0.x264", []string{"AAC"}},
		{"Movie.2020.720p.HDTV.AC3.x264", []string{"Dolby Digital"}},
		{"Movie.2020.720p.HDTV.DD5.1.x264", []string{"Dolby Digital"}},
		{"Movie.2020.720p.WEB.EAC3.x264", []string{"Dolby Digital Plus"}},
		{"Movie.2020.1080p.BluRay.x264", nil},
		{"Movie.Title.2024.WEBRip.1080p.x264-EAC3", nil},
	}
	for _, tt := range tests {
		if got := ParseAudioCodecs(tt.title); !slices.Equal(got, tt.want) {
			t.Errorf("ParseAudioCodecs(%q): got = %q, want = %q", tt.title, got, tt.want)
		}
	}
}
