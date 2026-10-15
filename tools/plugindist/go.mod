module github.com/mavioai/mavio/tools/plugindist

go 1.27.0

require (
	github.com/mavioai/mavio/libs/plugin v0.0.0
	github.com/mavioai/mavio/libs/proto v0.0.0
	golang.org/x/sync v0.24.0
)

require (
	buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go v1.36.12-20260825204119-511051f7f437.2 // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)

replace github.com/mavioai/mavio/libs/plugin => ../../libs/plugin

replace github.com/mavioai/mavio/libs/proto => ../../libs/proto
