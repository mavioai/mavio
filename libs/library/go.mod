module github.com/mavioai/mavio/libs/library

go 1.27.0

require (
	github.com/mavioai/mavio/libs/core v0.0.0
	github.com/mavioai/mavio/libs/metadata v0.0.0
	github.com/mavioai/mavio/libs/naming v0.0.0
	golang.org/x/sync v0.23.0
)

require (
	github.com/dlclark/regexp2 v1.12.0 // indirect
	golang.org/x/text v0.42.0 // indirect
)

replace github.com/mavioai/mavio/libs/core => ../core

replace github.com/mavioai/mavio/libs/naming => ../naming

replace github.com/mavioai/mavio/libs/metadata => ../metadata
