module github.com/mavioai/mavio/plugins/scraper-musicbrainz

go 1.27.0

require (
	connectrpc.com/connect v1.21.0
	github.com/mavioai/mavio/libs/plugin v0.0.0
	github.com/mavioai/mavio/libs/proto v0.0.0
	google.golang.org/protobuf v1.36.12
)

require buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go v1.36.12-20260825204119-511051f7f437.2 // indirect

replace github.com/mavioai/mavio/libs/plugin => ../../libs/plugin

replace github.com/mavioai/mavio/libs/proto => ../../libs/proto
