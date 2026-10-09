module github.com/mavioai/mavio/libs/library

go 1.27.0

require github.com/mavioai/mavio/libs/metadata v0.0.0

require github.com/mavioai/mavio/libs/core v0.0.0 // indirect

replace github.com/mavioai/mavio/libs/core => ../core

replace github.com/mavioai/mavio/libs/naming => ../naming

replace github.com/mavioai/mavio/libs/metadata => ../metadata
