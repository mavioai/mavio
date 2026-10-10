package indexer

// The categories are those of Prowlarr's NewznabStandardCategory; they are
// read only.

// The Newznab standard categories.
var (
	ZedOther          = &Category{ID: 0, Name: "Other"}
	ZedOtherMisc      = &Category{ID: 10, Name: "Other/Misc"}
	ZedOtherHashed    = &Category{ID: 20, Name: "Other/Hashed"}
	Console           = &Category{ID: 1000, Name: "Console"}
	ConsoleNDS        = &Category{ID: 1010, Name: "Console/NDS"}
	ConsolePSP        = &Category{ID: 1020, Name: "Console/PSP"}
	ConsoleWii        = &Category{ID: 1030, Name: "Console/Wii"}
	ConsoleXBox       = &Category{ID: 1040, Name: "Console/XBox"}
	ConsoleXBox360    = &Category{ID: 1050, Name: "Console/XBox 360"}
	ConsoleWiiware    = &Category{ID: 1060, Name: "Console/Wiiware"}
	ConsoleXBox360DLC = &Category{ID: 1070, Name: "Console/XBox 360 DLC"}
	ConsolePS3        = &Category{ID: 1080, Name: "Console/PS3"}
	ConsoleOther      = &Category{ID: 1090, Name: "Console/Other"}
	Console3DS        = &Category{ID: 1110, Name: "Console/3DS"}
	ConsolePSVita     = &Category{ID: 1120, Name: "Console/PS Vita"}
	ConsoleWiiU       = &Category{ID: 1130, Name: "Console/WiiU"}
	ConsoleXBoxOne    = &Category{ID: 1140, Name: "Console/XBox One"}
	ConsolePS4        = &Category{ID: 1180, Name: "Console/PS4"}
	Movies            = &Category{ID: 2000, Name: "Movies"}
	MoviesForeign     = &Category{ID: 2010, Name: "Movies/Foreign"}
	MoviesOther       = &Category{ID: 2020, Name: "Movies/Other"}
	MoviesSD          = &Category{ID: 2030, Name: "Movies/SD"}
	MoviesHD          = &Category{ID: 2040, Name: "Movies/HD"}
	MoviesUHD         = &Category{ID: 2045, Name: "Movies/UHD"}
	MoviesBluRay      = &Category{ID: 2050, Name: "Movies/BluRay"}
	Movies3D          = &Category{ID: 2060, Name: "Movies/3D"}
	MoviesDVD         = &Category{ID: 2070, Name: "Movies/DVD"}
	MoviesWEBDL       = &Category{ID: 2080, Name: "Movies/WEB-DL"}
	Moviesx265        = &Category{ID: 2090, Name: "Movies/x265"}
	Audio             = &Category{ID: 3000, Name: "Audio"}
	AudioMP3          = &Category{ID: 3010, Name: "Audio/MP3"}
	AudioVideo        = &Category{ID: 3020, Name: "Audio/Video"}
	AudioAudiobook    = &Category{ID: 3030, Name: "Audio/Audiobook"}
	AudioLossless     = &Category{ID: 3040, Name: "Audio/Lossless"}
	AudioOther        = &Category{ID: 3050, Name: "Audio/Other"}
	AudioForeign      = &Category{ID: 3060, Name: "Audio/Foreign"}
	PC                = &Category{ID: 4000, Name: "PC"}
	PC0day            = &Category{ID: 4010, Name: "PC/0day"}
	PCISO             = &Category{ID: 4020, Name: "PC/ISO"}
	PCMac             = &Category{ID: 4030, Name: "PC/Mac"}
	PCMobileOther     = &Category{ID: 4040, Name: "PC/Mobile-Other"}
	PCGames           = &Category{ID: 4050, Name: "PC/Games"}
	PCMobileiOS       = &Category{ID: 4060, Name: "PC/Mobile-iOS"}
	PCMobileAndroid   = &Category{ID: 4070, Name: "PC/Mobile-Android"}
	TV                = &Category{ID: 5000, Name: "TV"}
	TVWEBDL           = &Category{ID: 5010, Name: "TV/WEB-DL"}
	TVForeign         = &Category{ID: 5020, Name: "TV/Foreign"}
	TVSD              = &Category{ID: 5030, Name: "TV/SD"}
	TVHD              = &Category{ID: 5040, Name: "TV/HD"}
	TVUHD             = &Category{ID: 5045, Name: "TV/UHD"}
	TVOther           = &Category{ID: 5050, Name: "TV/Other"}
	TVSport           = &Category{ID: 5060, Name: "TV/Sport"}
	TVAnime           = &Category{ID: 5070, Name: "TV/Anime"}
	TVDocumentary     = &Category{ID: 5080, Name: "TV/Documentary"}
	TVx265            = &Category{ID: 5090, Name: "TV/x265"}
	XXX               = &Category{ID: 6000, Name: "XXX"}
	XXXDVD            = &Category{ID: 6010, Name: "XXX/DVD"}
	XXXWMV            = &Category{ID: 6020, Name: "XXX/WMV"}
	XXXXviD           = &Category{ID: 6030, Name: "XXX/XviD"}
	XXXx264           = &Category{ID: 6040, Name: "XXX/x264"}
	XXXUHD            = &Category{ID: 6045, Name: "XXX/UHD"}
	XXXPack           = &Category{ID: 6050, Name: "XXX/Pack"}
	XXXImageSet       = &Category{ID: 6060, Name: "XXX/ImageSet"}
	XXXOther          = &Category{ID: 6070, Name: "XXX/Other"}
	XXXSD             = &Category{ID: 6080, Name: "XXX/SD"}
	XXXWEBDL          = &Category{ID: 6090, Name: "XXX/WEB-DL"}
	Books             = &Category{ID: 7000, Name: "Books"}
	BooksMags         = &Category{ID: 7010, Name: "Books/Mags"}
	BooksEBook        = &Category{ID: 7020, Name: "Books/EBook"}
	BooksComics       = &Category{ID: 7030, Name: "Books/Comics"}
	BooksTechnical    = &Category{ID: 7040, Name: "Books/Technical"}
	BooksOther        = &Category{ID: 7050, Name: "Books/Other"}
	BooksForeign      = &Category{ID: 7060, Name: "Books/Foreign"}
	Other             = &Category{ID: 8000, Name: "Other"}
	OtherMisc         = &Category{ID: 8010, Name: "Other/Misc"}
	OtherHashed       = &Category{ID: 8020, Name: "Other/Hashed"}
)

// parentCategories are the standard categories with subcategories.
var parentCategories = []*Category{
	Console,
	Movies,
	Audio,
	PC,
	TV,
	XXX,
	Books,
	Other,
	ZedOther,
}

// standardCategories are all the standard categories.
var standardCategories = []*Category{
	Console,
	ConsoleNDS,
	ConsolePSP,
	ConsoleWii,
	ConsoleXBox,
	ConsoleXBox360,
	ConsoleWiiware,
	ConsoleXBox360DLC,
	ConsolePS3,
	ConsoleOther,
	Console3DS,
	ConsolePSVita,
	ConsoleWiiU,
	ConsoleXBoxOne,
	ConsolePS4,
	Movies,
	MoviesForeign,
	MoviesOther,
	MoviesSD,
	MoviesHD,
	MoviesUHD,
	MoviesBluRay,
	Movies3D,
	MoviesDVD,
	MoviesWEBDL,
	Moviesx265,
	Audio,
	AudioMP3,
	AudioVideo,
	AudioAudiobook,
	AudioLossless,
	AudioOther,
	AudioForeign,
	PC,
	PC0day,
	PCISO,
	PCMac,
	PCMobileOther,
	PCGames,
	PCMobileiOS,
	PCMobileAndroid,
	TV,
	TVWEBDL,
	TVForeign,
	TVSD,
	TVHD,
	TVUHD,
	TVOther,
	TVSport,
	TVAnime,
	TVDocumentary,
	TVx265,
	XXX,
	XXXDVD,
	XXXWMV,
	XXXXviD,
	XXXx264,
	XXXUHD,
	XXXPack,
	XXXImageSet,
	XXXOther,
	XXXSD,
	XXXWEBDL,
	Books,
	BooksMags,
	BooksEBook,
	BooksComics,
	BooksTechnical,
	BooksOther,
	BooksForeign,
	Other,
	OtherMisc,
	OtherHashed,
	ZedOther,
	ZedOtherHashed,
	ZedOtherMisc,
}

func init() {
	ZedOther.Sub = []*Category{ZedOtherMisc, ZedOtherHashed}
	Console.Sub = []*Category{ConsoleNDS, ConsolePSP, ConsoleWii, ConsoleXBox, ConsoleXBox360, ConsoleWiiware, ConsoleXBox360DLC, ConsolePS3, ConsoleOther, Console3DS, ConsolePSVita, ConsoleWiiU, ConsoleXBoxOne, ConsolePS4}
	Movies.Sub = []*Category{MoviesForeign, MoviesOther, MoviesSD, MoviesHD, MoviesUHD, MoviesBluRay, Movies3D, MoviesDVD, MoviesWEBDL, Moviesx265}
	Audio.Sub = []*Category{AudioMP3, AudioVideo, AudioAudiobook, AudioLossless, AudioOther, AudioForeign}
	PC.Sub = []*Category{PC0day, PCISO, PCMac, PCMobileOther, PCGames, PCMobileiOS, PCMobileAndroid}
	TV.Sub = []*Category{TVWEBDL, TVForeign, TVSD, TVHD, TVUHD, TVOther, TVSport, TVAnime, TVDocumentary, TVx265}
	XXX.Sub = []*Category{XXXDVD, XXXWMV, XXXXviD, XXXx264, XXXUHD, XXXPack, XXXImageSet, XXXOther, XXXSD, XXXWEBDL}
	Books.Sub = []*Category{BooksMags, BooksEBook, BooksComics, BooksTechnical, BooksOther, BooksForeign}
	Other.Sub = []*Category{OtherMisc, OtherHashed}
}
