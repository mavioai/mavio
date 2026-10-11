module github.com/mavioai/mavio/libs/acquire

go 1.27.0

require (
	github.com/dlclark/regexp2 v1.12.0
	github.com/mavioai/mavio/libs/release v0.0.0
)

require golang.org/x/text v0.43.0 // indirect

replace github.com/mavioai/mavio/libs/release => ../release
