package gazellegames

import "github.com/mavioai/mavio/plugins/indexers/internal/indexer"

// capabilities returns GazelleGames' capabilities: its platforms, which
// searches name, and its categories.
func capabilities() *indexer.Capabilities {
	c := indexer.NewCapabilities()
	// Apple
	c.Categories.Add("Mac", indexer.ConsoleOther, "Mac")
	c.Categories.Add("iOS", indexer.PCMobileiOS, "iOS")
	c.Categories.Add("Apple Bandai Pippin", indexer.ConsoleOther, "Apple Bandai Pippin")
	c.Categories.Add("Apple II", indexer.ConsoleOther, "Apple II")
	// Google
	c.Categories.Add("Android", indexer.PCMobileAndroid, "Android")
	// Microsoft
	c.Categories.Add("DOS", indexer.PCGames, "DOS")
	c.Categories.Add("Windows", indexer.PCGames, "Windows")
	c.Categories.Add("Xbox", indexer.ConsoleXBox, "Xbox")
	c.Categories.Add("Xbox 360", indexer.ConsoleXBox360, "Xbox 360")
	// Nintendo
	c.Categories.Add("Game Boy", indexer.ConsoleOther, "Game Boy")
	c.Categories.Add("Game Boy Advance", indexer.ConsoleOther, "Game Boy Advance")
	c.Categories.Add("Game Boy Color", indexer.ConsoleOther, "Game Boy Color")
	c.Categories.Add("NES", indexer.ConsoleOther, "NES")
	c.Categories.Add("Nintendo 64", indexer.ConsoleOther, "Nintendo 64")
	c.Categories.Add("Nintendo 3DS", indexer.ConsoleOther, "Nintendo 3DS")
	c.Categories.Add("New Nintendo 3DS", indexer.ConsoleOther, "New Nintendo 3DS")
	c.Categories.Add("Nintendo DS", indexer.ConsoleNDS, "Nintendo DS")
	c.Categories.Add("Nintendo GameCube", indexer.ConsoleOther, "Nintendo GameCube")
	c.Categories.Add("Pokemon Mini", indexer.ConsoleOther, "Pokemon Mini")
	c.Categories.Add("SNES", indexer.ConsoleOther, "SNES")
	c.Categories.Add("Switch", indexer.ConsoleOther, "Switch")
	c.Categories.Add("Virtual Boy", indexer.ConsoleOther, "Virtual Boy")
	c.Categories.Add("Wii", indexer.ConsoleWii, "Wii")
	c.Categories.Add("Wii U", indexer.ConsoleWiiU, "Wii U")
	// Sony
	c.Categories.Add("PlayStation 1", indexer.ConsoleOther, "PlayStation 1")
	c.Categories.Add("PlayStation 2", indexer.ConsoleOther, "PlayStation 2")
	c.Categories.Add("PlayStation 3", indexer.ConsolePS3, "PlayStation 3")
	c.Categories.Add("PlayStation 4", indexer.ConsolePS4, "PlayStation 4")
	c.Categories.Add("PlayStation Portable", indexer.ConsolePSP, "PlayStation Portable")
	c.Categories.Add("PlayStation Vita", indexer.ConsolePSVita, "PlayStation Vita")
	// Sega
	c.Categories.Add("Dreamcast", indexer.ConsoleOther, "Dreamcast")
	c.Categories.Add("Game Gear", indexer.ConsoleOther, "Game Gear")
	c.Categories.Add("Master System", indexer.ConsoleOther, "Master System")
	c.Categories.Add("Mega Drive", indexer.ConsoleOther, "Mega Drive")
	c.Categories.Add("Pico", indexer.ConsoleOther, "Pico")
	c.Categories.Add("Saturn", indexer.ConsoleOther, "Saturn")
	c.Categories.Add("SG-1000", indexer.ConsoleOther, "SG-1000")
	// Atari
	c.Categories.Add("Atari 2600", indexer.ConsoleOther, "Atari 2600")
	c.Categories.Add("Atari 5200", indexer.ConsoleOther, "Atari 5200")
	c.Categories.Add("Atari 7800", indexer.ConsoleOther, "Atari 7800")
	c.Categories.Add("Atari Jaguar", indexer.ConsoleOther, "Atari Jaguar")
	c.Categories.Add("Atari Lynx", indexer.ConsoleOther, "Atari Lynx")
	c.Categories.Add("Atari ST", indexer.ConsoleOther, "Atari ST")
	// Amstrad
	c.Categories.Add("Amstrad CPC", indexer.ConsoleOther, "Amstrad CPC")
	// Bandai
	c.Categories.Add("Bandai WonderSwan", indexer.ConsoleOther, "Bandai WonderSwan")
	c.Categories.Add("Bandai WonderSwan Color", indexer.ConsoleOther, "Bandai WonderSwan Color")
	// Commodore
	c.Categories.Add("Commodore 64", indexer.ConsoleOther, "Commodore 64")
	c.Categories.Add("Commodore 128", indexer.ConsoleOther, "Commodore 128")
	c.Categories.Add("Commodore Amiga", indexer.ConsoleOther, "Commodore Amiga")
	c.Categories.Add("Amiga CD32", indexer.ConsoleOther, "Amiga CD32")
	c.Categories.Add("Commodore Plus-4", indexer.ConsoleOther, "Commodore Plus-4")
	c.Categories.Add("Commodore VIC-20", indexer.ConsoleOther, "Commodore VIC-20")
	// NEC
	c.Categories.Add("NEC PC-98", indexer.ConsoleOther, "NEC PC-98")
	c.Categories.Add("NEC PC-FX", indexer.ConsoleOther, "NEC PC-FX")
	c.Categories.Add("NEC SuperGrafx", indexer.ConsoleOther, "NEC SuperGrafx")
	c.Categories.Add("NEC TurboGrafx-16", indexer.ConsoleOther, "NEC TurboGrafx-16")
	// Sinclair
	c.Categories.Add("ZX Spectrum", indexer.ConsoleOther, "ZX Spectrum")
	// Spectravideo
	c.Categories.Add("MSX", indexer.ConsoleOther, "MSX")
	c.Categories.Add("MSX 2", indexer.ConsoleOther, "MSX 2")
	// Tiger
	c.Categories.Add("Game.com", indexer.ConsoleOther, "Game.com")
	c.Categories.Add("Gizmondo", indexer.ConsoleOther, "Gizmondo")
	// VTech
	c.Categories.Add("V.Smile", indexer.ConsoleOther, "V.Smile")
	c.Categories.Add("CreatiVision", indexer.ConsoleOther, "CreatiVision")
	// Tabletop Games
	c.Categories.Add("Board Game", indexer.ConsoleOther, "Board Game")
	c.Categories.Add("Card Game", indexer.ConsoleOther, "Card Game")
	c.Categories.Add("Miniature Wargames", indexer.ConsoleOther, "Miniature Wargames")
	c.Categories.Add("Pen and Paper RPG", indexer.ConsoleOther, "Pen and Paper RPG")
	// Other
	c.Categories.Add("3DO", indexer.ConsoleOther, "3DO")
	c.Categories.Add("Casio Loopy", indexer.ConsoleOther, "Casio Loopy")
	c.Categories.Add("Casio PV-1000", indexer.ConsoleOther, "Casio PV-1000")
	c.Categories.Add("Colecovision", indexer.ConsoleOther, "Colecovision")
	c.Categories.Add("Emerson Arcadia 2001", indexer.ConsoleOther, "Emerson Arcadia 2001")
	c.Categories.Add("Entex Adventure Vision", indexer.ConsoleOther, "Entex Adventure Vision")
	c.Categories.Add("Epoch Super Casette Vision", indexer.ConsoleOther, "Epoch Super Casette Vision") //nolint:misspell // the site's name
	c.Categories.Add("Fairchild Channel F", indexer.ConsoleOther, "Fairchild Channel F")
	c.Categories.Add("Funtech Super Acan", indexer.ConsoleOther, "Funtech Super Acan")
	c.Categories.Add("GamePark GP32", indexer.ConsoleOther, "GamePark GP32")
	c.Categories.Add("General Computer Vectrex", indexer.ConsoleOther, "General Computer Vectrex")
	c.Categories.Add("Interactive DVD", indexer.ConsoleOther, "Interactive DVD")
	c.Categories.Add("Linux", indexer.ConsoleOther, "Linux")
	c.Categories.Add("Hartung Game Master", indexer.ConsoleOther, "Hartung Game Master")
	c.Categories.Add("Magnavox-Phillips Odyssey", indexer.ConsoleOther, "Magnavox-Phillips Odyssey")
	c.Categories.Add("Mattel Intellivision", indexer.ConsoleOther, "Mattel Intellivision")
	c.Categories.Add("Memotech MTX", indexer.ConsoleOther, "Memotech MTX")
	c.Categories.Add("Miles Gordon Sam Coupe", indexer.ConsoleOther, "Miles Gordon Sam Coupe")
	c.Categories.Add("Nokia N-Gage", indexer.ConsoleOther, "Nokia N-Gage")
	c.Categories.Add("Oculus Quest", indexer.ConsoleOther, "Oculus Quest")
	c.Categories.Add("Ouya", indexer.ConsoleOther, "Ouya")
	c.Categories.Add("Philips Videopac+", indexer.ConsoleOther, "Philips Videopac+")
	c.Categories.Add("Philips CD-i", indexer.ConsoleOther, "Philips CD-i")
	c.Categories.Add("Phone/PDA", indexer.ConsoleOther, "Phone/PDA")
	c.Categories.Add("RCA Studio II", indexer.ConsoleOther, "RCA Studio II")
	c.Categories.Add("Sharp X1", indexer.ConsoleOther, "Sharp X1")
	c.Categories.Add("Sharp X68000", indexer.ConsoleOther, "Sharp X68000")
	c.Categories.Add("SNK Neo Geo", indexer.ConsoleOther, "SNK Neo Geo")
	c.Categories.Add("SNK Neo Geo Pocket", indexer.ConsoleOther, "SNK Neo Geo Pocket")
	c.Categories.Add("Taito Type X", indexer.ConsoleOther, "Taito Type X")
	c.Categories.Add("Tandy Color Computer", indexer.ConsoleOther, "Tandy Color Computer")
	c.Categories.Add("Tangerine Oric", indexer.ConsoleOther, "Tangerine Oric")
	c.Categories.Add("Thomson MO5", indexer.ConsoleOther, "Thomson MO5")
	c.Categories.Add("Watara Supervision", indexer.ConsoleOther, "Watara Supervision")
	c.Categories.Add("Retro - Other", indexer.ConsoleOther, "Retro - Other")
	// Categories, which are not platforms
	c.Categories.Add("1", indexer.PCGames, "Games")
	c.Categories.Add("2", indexer.PC0day, "Applications")
	c.Categories.Add("3", indexer.BooksEBook, "E-Books")
	c.Categories.Add("4", indexer.AudioOther, "OST")
	return c
}
