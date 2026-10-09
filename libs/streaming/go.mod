module github.com/mavioai/mavio/libs/streaming

go 1.27.0

require (
	github.com/mavioai/mavio/libs/core v0.0.0
	github.com/mavioai/mavio/libs/media v0.0.0
)

require golang.org/x/sys v0.48.0 // indirect

replace github.com/mavioai/mavio/libs/core => ../core

replace github.com/mavioai/mavio/libs/media => ../media

replace github.com/mavioai/mavio/tools/fixtures => ../../tools/fixtures
