module main

go 1.15

replace github.com/hunyxv/grpcpool => ../

replace pb => ./pb

require (
	github.com/golang/protobuf v1.4.3 // indirect
	github.com/hunyxv/grpcpool v0.0.0-00010101000000-000000000000 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	golang.org/x/net v0.0.0-20201202161906-c7110b5ffcbb // indirect
	golang.org/x/sys v0.0.0-20201202213521-69691e467435 // indirect
	golang.org/x/text v0.3.4 // indirect
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e
	google.golang.org/genproto v0.0.0-20201203001206-6486ece9c497 // indirect
	google.golang.org/grpc v1.34.0
	google.golang.org/protobuf v1.25.0 // indirect
	pb v0.0.0-00010101000000-000000000000
)
