module github.com/mavioai/mavio/apps/server

go 1.27.0

require github.com/mavioai/mavio/libs/proto v0.0.0

require (
	connectrpc.com/connect v1.21.0 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)

replace github.com/mavioai/mavio/libs/proto => ../../libs/proto
