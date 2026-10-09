module github.com/mavioai/mavio/plugins/scraper-openlibrary

go 1.27.0

require (
	github.com/mavioai/mavio/libs/plugin v0.0.0
	github.com/mavioai/mavio/libs/proto v0.0.0
	google.golang.org/protobuf v1.36.12
)

require connectrpc.com/connect v1.21.0 // indirect

replace github.com/mavioai/mavio/libs/plugin => ../../libs/plugin

replace github.com/mavioai/mavio/libs/proto => ../../libs/proto
