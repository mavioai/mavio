module github.com/mavioai/mavio/plugins/indexers

go 1.27.0

require (
	connectrpc.com/connect v1.21.0
	github.com/andybalholm/cascadia v1.3.4
	github.com/dlclark/regexp2 v1.12.0
	github.com/mavioai/mavio/libs/plugin v0.0.0-00010101000000-000000000000
	github.com/mavioai/mavio/libs/proto v0.0.0
	go.yaml.in/yaml/v3 v3.0.5
	golang.org/x/net v0.61.0
	golang.org/x/text v0.43.0
	google.golang.org/protobuf v1.36.12
)

require buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go v1.36.12-20260825204119-511051f7f437.2 // indirect

replace github.com/mavioai/mavio/libs/plugin => ../../libs/plugin

replace github.com/mavioai/mavio/libs/proto => ../../libs/proto
