module github.com/mavioai/mavio/libs/media

go 1.27.0

require (
	github.com/mavioai/mavio/libs/core v0.0.0
	github.com/mavioai/mavio/tools/fixtures v0.0.0
)

replace github.com/mavioai/mavio/libs/core => ../core

replace github.com/mavioai/mavio/tools/fixtures => ../../tools/fixtures
