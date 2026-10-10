package speedapp

import (
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
)

// Definitions are the SpeedApp sites.
var Definitions = []indexer.Definition{
	definition(indexer.Definition{
		ID: "retroflix", Name: "RetroFlix",
		Description: "RetroFlix (RFX) is a Private Torrent Tracker for Classic Movies / TV / General Releases",
	}, retroFlix),
	definition(indexer.Definition{
		ID: "speedapp", Name: "SpeedApp.io", Language: "ro-RO",
		Description: "SpeedApp is a ROMANIAN Private Torrent Tracker for MOVIES / TV / GENERAL",
	}, speedApp),
}

var retroFlix = site{
	links:           []string{"https://retroflix.club/"},
	legacy:          []string{"https://retroflix.net/"},
	rateLimit:       2100 * time.Millisecond,
	minimumSeedTime: 432000, // 120 hours
	categories: func(c *indexer.Categories) {
		c.Add("401", indexer.Movies, "Movies")
		c.Add("402", indexer.TV, "TV Series")
		c.Add("406", indexer.AudioVideo, "Music Videos")
		c.Add("407", indexer.TVSport, "Sports")
		c.Add("409", indexer.Books, "Books")
		c.Add("408", indexer.Audio, "HQ Audio")
	},
}

var speedApp = site{
	links:           []string{"https://speedapp.io/"},
	legacy:          []string{"https://speedapp.io"},
	minimumSeedTime: 172800, // 48 hours
	categories: func(c *indexer.Categories) {
		c.Add("38", indexer.Movies, "Movie Packs")
		c.Add("10", indexer.MoviesSD, "Movies: SD")
		c.Add("35", indexer.MoviesSD, "Movies: SD Ro")
		c.Add("8", indexer.MoviesHD, "Movies: HD")
		c.Add("29", indexer.MoviesHD, "Movies: HD Ro")
		c.Add("7", indexer.MoviesDVD, "Movies: DVD")
		c.Add("2", indexer.MoviesDVD, "Movies: DVD Ro")
		c.Add("17", indexer.MoviesBluRay, "Movies: BluRay")
		c.Add("24", indexer.MoviesBluRay, "Movies: BluRay Ro")
		c.Add("59", indexer.Movies, "Movies: Ro")
		c.Add("57", indexer.MoviesUHD, "Movies: 4K (2160p) Ro")
		c.Add("61", indexer.MoviesUHD, "Movies: 4K (2160p)")
		c.Add("41", indexer.TV, "TV Packs")
		c.Add("66", indexer.TV, "TV Packs Ro")
		c.Add("45", indexer.TVSD, "TV Episodes")
		c.Add("46", indexer.TVSD, "TV Episodes Ro")
		c.Add("43", indexer.TVHD, "TV Episodes HD")
		c.Add("44", indexer.TVHD, "TV Episodes HD Ro")
		c.Add("60", indexer.TV, "TV Ro")
		c.Add("11", indexer.PCGames, "Games: PC-ISO")
		c.Add("52", indexer.Console, "Games: Console")
		c.Add("1", indexer.PC0day, "Applications")
		c.Add("14", indexer.PC, "Applications: Linux")
		c.Add("37", indexer.PCMac, "Applications: Mac")
		c.Add("19", indexer.PCMobileOther, "Applications: Mobile")
		c.Add("62", indexer.TV, "TV Cartoons")
		c.Add("3", indexer.TVAnime, "TV Anime / Hentai")
		c.Add("6", indexer.BooksEBook, "E-books")
		c.Add("5", indexer.Audio, "Music")
		c.Add("64", indexer.AudioVideo, "Music Video")
		c.Add("18", indexer.Other, "Images")
		c.Add("22", indexer.TVSport, "TV Sports")
		c.Add("58", indexer.TVSport, "TV Sports Ro")
		c.Add("9", indexer.TVDocumentary, "TV Documentary")
		c.Add("63", indexer.TVDocumentary, "TV Documentary Ro")
		c.Add("65", indexer.Other, "Tutorial")
		c.Add("67", indexer.OtherMisc, "Miscellaneous")
		c.Add("15", indexer.XXX, "XXX Movies")
		c.Add("47", indexer.XXX, "XXX DVD")
		c.Add("48", indexer.XXX, "XXX HD")
		c.Add("49", indexer.XXXImageSet, "XXX Images")
		c.Add("50", indexer.XXX, "XXX Packs")
		c.Add("51", indexer.XXX, "XXX SD")
	},
}
