module github.com/mavioai/mavio/libs/rclone

go 1.27.1

require (
	github.com/mavioai/mavio/libs/core v0.0.0
	golang.org/x/sys v0.49.0
)

replace github.com/mavioai/mavio/libs/core => ../core
