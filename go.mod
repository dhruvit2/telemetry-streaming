module telemetry-streaming

go 1.26.2

require (
	github.com/dhruvit2/messagebroker v0.0.2
	go.uber.org/zap v1.24.0
	google.golang.org/grpc v1.80.0
)

//replace github.com/dhruvit2/messagebroker => ../messagebroker

require (
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.49.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	golang.org/x/text v0.33.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260120221211-b8f7ae30c516 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
