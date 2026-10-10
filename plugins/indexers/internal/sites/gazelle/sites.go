package gazelle

import (
	"strconv"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
)

// Definitions are the Gazelle sites.
var Definitions = []indexer.Definition{
	definition(indexer.Definition{
		ID: "alpharatio", Name: "AlphaRatio",
		Description: "AlphaRatio(AR) is a Private Torrent Tracker for 0DAY / GENERAL",
	}, alphaRatio, freeleechOnlyField("Search freeleech releases only"), excludeSceneField),
	definition(indexer.Definition{
		ID: "brokenstones", Name: "BrokenStones",
		Description: "Broken Stones is a Private site for MacOS and iOS APPS / GAMES",
	}, brokenStones),
	definition(indexer.Definition{
		ID: "desigaane", Name: "DesiGaane",
		Description: "DesiGaane is a Private Torrent Tracker for DESI MUSIC",
	}, desiGaane),
	definition(indexer.Definition{
		ID: "dicmusic", Name: "DICMusic", Language: "zh-CN",
		Description: "DICMusic is a CHINESE Private Torrent Tracker for MUSIC",
	}, dicMusic),
	definition(indexer.Definition{
		ID: "greatposterwall", Name: "GreatPosterWall",
		Description: "GreatPosterWall (GPW) is a CHINESE Private site for MOVIES",
	}, greatPosterWall, freeleechOnlyField("Search freeleech torrents only")),
	definition(indexer.Definition{
		ID: "secretcinema", Name: "Secret Cinema",
		Description: "Secret Cinema is a Private ratioless site for rare MOVIES.",
	}, secretCinema),
}

var alphaRatio = site{
	links: []string{"https://alpharatio.cc/"},
	info: func() indexer.Info {
		i := baseInfo
		i.SupportsPagination = true
		i.RateLimit = 3 * time.Second
		return i
	}(),
	imdbInTags: true,
	caps: func() *indexer.Capabilities {
		c := indexer.NewCapabilities()
		c.LimitsDefault, c.LimitsMax = 50, 50
		c.Params[indexer.SearchTV] = []indexer.Param{indexer.ParamQ, indexer.ParamSeason, indexer.ParamEp}
		c.Params[indexer.SearchMovie] = []indexer.Param{indexer.ParamQ}
		c.Categories.Add("1", indexer.TVSD, "TvSD")
		c.Categories.Add("2", indexer.TVHD, "TvHD")
		c.Categories.Add("3", indexer.TVUHD, "TvUHD")
		c.Categories.Add("4", indexer.TVSD, "TvDVDRip")
		c.Categories.Add("5", indexer.TVSD, "TvPackSD")
		c.Categories.Add("6", indexer.TVHD, "TvPackHD")
		c.Categories.Add("7", indexer.TVUHD, "TvPackUHD")
		c.Categories.Add("8", indexer.MoviesSD, "MovieSD")
		c.Categories.Add("9", indexer.MoviesHD, "MovieHD")
		c.Categories.Add("10", indexer.MoviesUHD, "MovieUHD")
		c.Categories.Add("11", indexer.MoviesSD, "MoviePackSD")
		c.Categories.Add("12", indexer.MoviesHD, "MoviePackHD")
		c.Categories.Add("13", indexer.MoviesUHD, "MoviePackUHD")
		c.Categories.Add("14", indexer.XXX, "MovieXXX")
		c.Categories.Add("15", indexer.MoviesBluRay, "Bluray")
		c.Categories.Add("16", indexer.TVAnime, "AnimeSD")
		c.Categories.Add("17", indexer.TVAnime, "AnimeHD")
		c.Categories.Add("18", indexer.PCGames, "GamesPC")
		c.Categories.Add("19", indexer.ConsoleXBox, "GamesxBox")
		c.Categories.Add("20", indexer.ConsolePS4, "GamesPS")
		c.Categories.Add("21", indexer.ConsoleWii, "GamesNin")
		c.Categories.Add("22", indexer.PC0day, "AppsWindows")
		c.Categories.Add("23", indexer.PCMac, "AppsMAC")
		c.Categories.Add("24", indexer.PC0day, "AppsLinux")
		c.Categories.Add("25", indexer.PCMobileOther, "AppsMobile")
		c.Categories.Add("26", indexer.XXX, "0dayXXX")
		c.Categories.Add("27", indexer.Books, "eBook")
		c.Categories.Add("28", indexer.AudioAudiobook, "AudioBook")
		c.Categories.Add("29", indexer.AudioOther, "Music")
		c.Categories.Add("30", indexer.Other, "Misc")
		return c
	},
	params: func(s *Site, p *indexer.Params, q *indexer.Query) {
		if s.settings.FreeleechOnly {
			p.Set("freetorrent", "1")
		}
		if s.settings.ExcludeScene {
			p.Set("scene", "0")
		}
		if q.Limit > 0 && q.Offset > 0 {
			p.Set("page", strconv.Itoa(q.Offset/q.Limit+1))
		}
	},
}

var brokenStones = site{
	links: []string{"https://brokenstones.is/"},
	info:  baseInfo,
	caps: func() *indexer.Capabilities {
		c := indexer.NewCapabilities()
		c.Categories.Add("1", indexer.PCMac, "MacOS Apps")
		c.Categories.Add("2", indexer.PCMac, "MacOS Games")
		c.Categories.Add("3", indexer.PCMobileiOS, "iOS Apps")
		c.Categories.Add("4", indexer.PCMobileiOS, "iOS Games")
		c.Categories.Add("5", indexer.Other, "Graphics")
		c.Categories.Add("6", indexer.Audio, "Audio")
		c.Categories.Add("7", indexer.Other, "Tutorials")
		c.Categories.Add("8", indexer.Other, "Other")
		return c
	},
}

var desiGaane = site{
	links: []string{"https://desigaane.rocks/"},
	info:  baseInfo,
	caps: func() *indexer.Capabilities {
		c := indexer.NewCapabilities()
		c.Categories.Add("1", indexer.Audio, "Music")
		return c
	},
}

var dicMusic = site{
	links:  []string{"https://dicmusic.com/"},
	legacy: []string{"https://dicmusic.club/"},
	cookie: true,
	info:   baseInfo,
	caps: func() *indexer.Capabilities {
		c := indexer.NewCapabilities()
		c.Params[indexer.SearchMusic] = []indexer.Param{indexer.ParamQ, indexer.ParamAlbum, indexer.ParamArtist, indexer.ParamLabel, indexer.ParamYear}
		c.Categories.Add("1", indexer.Audio, "Music")
		c.Categories.Add("2", indexer.PC, "Applications")
		return c
	},
}
