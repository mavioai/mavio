module github.com/mavioai/mavio/apps/desktop

go 1.27.1

require (
	connectrpc.com/connect v1.21.0
	github.com/ebitengine/purego v0.11.1
	github.com/egoist/mygo v0.5.5
	github.com/mavioai/mavio/libs/client-rust/capi v0.0.0-00010101000000-000000000000
	github.com/mavioai/mavio/libs/design v0.0.0
	github.com/mavioai/mavio/libs/proto v0.0.0
	golang.org/x/text v0.43.0
	google.golang.org/protobuf v1.36.12
	rsc.io/qr v0.2.0
)

require (
	buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go v1.36.12-20260825204119-511051f7f437.2 // indirect
	github.com/go-text/typesetting v0.3.5 // indirect
	golang.org/x/image v0.46.0 // indirect
)

replace github.com/egoist/mygo => github.com/mavioai/mygo v0.0.0-20261019122329-505d4a2f2619

replace github.com/mavioai/mavio/libs/design => ../../libs/design

replace github.com/mavioai/mavio/libs/client-rust/capi => ../../libs/client-rust/capi

replace github.com/mavioai/mavio/libs/proto => ../../libs/proto
