module github.com/mavioai/mavio/libs/media

go 1.27.0

require (
	github.com/mavioai/mavio/libs/core v0.0.0
	github.com/mavioai/mavio/libs/subtitle v0.0.0
	github.com/mavioai/mavio/tools/fixtures v0.0.0
	golang.org/x/sys v0.48.0
)

require (
	github.com/wlynxg/chardet v1.0.5 // indirect
	golang.org/x/text v0.42.0 // indirect
)

replace github.com/mavioai/mavio/libs/core => ../core

replace github.com/mavioai/mavio/tools/fixtures => ../../tools/fixtures

replace github.com/mavioai/mavio/libs/subtitle => ../subtitle
