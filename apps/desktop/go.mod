module github.com/mavioai/mavio/apps/desktop

go 1.27.1

require (
	github.com/egoist/mygo v0.5.5
	github.com/mavioai/mavio/libs/client-rust/capi v0.0.0-00010101000000-000000000000
	github.com/mavioai/mavio/libs/design v0.0.0
)

require (
	github.com/ebitengine/purego v0.11.1 // indirect
	github.com/go-text/typesetting v0.3.5 // indirect
	golang.org/x/image v0.46.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)

replace github.com/egoist/mygo => github.com/mavioai/mygo v0.0.0-20261017101849-79674cdfa118

replace github.com/mavioai/mavio/libs/design => ../../libs/design

replace github.com/mavioai/mavio/libs/client-rust/capi => ../../libs/client-rust/capi

replace github.com/mavioai/mavio/libs/proto => ../../libs/proto
