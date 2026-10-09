module github.com/mavioai/mavio/libs/library

go 1.27.0

require (
	github.com/mavioai/mavio/libs/core v0.0.0
	github.com/mavioai/mavio/libs/metadata v0.0.0
	github.com/mavioai/mavio/libs/naming v0.0.0
	github.com/mavioai/mavio/libs/subtitle v0.0.0-20261009173406-cd88046ac3d1
	golang.org/x/sync v0.23.0
	golang.org/x/sys v0.49.0
	golang.org/x/text v0.42.0
)

require (
	github.com/dlclark/regexp2 v1.12.0 // indirect
	github.com/wlynxg/chardet v1.0.5 // indirect
)

replace github.com/mavioai/mavio/libs/core => ../core

replace github.com/mavioai/mavio/libs/naming => ../naming

replace github.com/mavioai/mavio/libs/metadata => ../metadata
