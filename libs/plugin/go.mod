module github.com/mavioai/mavio/libs/plugin

go 1.27.0

require (
	connectrpc.com/connect v1.21.0
	github.com/mavioai/mavio/libs/proto v0.0.0
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3
	github.com/tetratelabs/wazero v1.12.0
	google.golang.org/protobuf v1.36.12
)

require (
	golang.org/x/sys v0.44.0 // indirect
	golang.org/x/text v0.41.0 // indirect
)

replace github.com/mavioai/mavio/libs/proto => ../proto
