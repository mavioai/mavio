module github.com/mavioai/mavio/libs/media

go 1.27.0

require (
	github.com/mavioai/mavio/libs/core v0.0.0
	github.com/mavioai/mavio/libs/imaging v0.0.0
	github.com/mavioai/mavio/libs/subtitle v0.0.0
	github.com/mavioai/mavio/tools/fixtures v0.0.0
	golang.org/x/sys v0.48.0
)

require (
	github.com/bbrks/go-blurhash v1.2.0 // indirect
	github.com/gen2brain/vpx v0.2.1 // indirect
	github.com/wlynxg/chardet v1.0.5 // indirect
	go.n16f.net/thumbhash v1.1.0 // indirect
	golang.org/x/image v0.46.0 // indirect
	golang.org/x/text v0.42.0 // indirect
)

replace github.com/mavioai/mavio/libs/core => ../core

replace github.com/mavioai/mavio/tools/fixtures => ../../tools/fixtures

replace github.com/mavioai/mavio/libs/subtitle => ../subtitle

replace github.com/mavioai/mavio/libs/imaging => ../imaging
