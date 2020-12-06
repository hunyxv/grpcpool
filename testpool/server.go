package main

import (
	"context"
	"fmt"

	"math/rand"
	"net"
	"time"
	
	"github.com/hunyxv/grpcpool/testpool/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

type HelloService struct{}

func (h *HelloService) SayHello(ctx context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {
	rander := rand.New(rand.NewSource(time.Now().UnixNano()))
	time.Sleep(time.Duration(time.Millisecond) * time.Duration(rander.Intn(100)+1))

	return &pb.HelloReply{Msg: fmt.Sprintf("Hello, %s!\n", req.Name)}, nil
}

func startServer() {
	var addr string = "127.0.0.1:8080"

	var keep = keepalive.EnforcementPolicy{
		MinTime:             5 * time.Second,
		PermitWithoutStream: true,
	}

	var kasp = keepalive.ServerParameters{
		MaxConnectionIdle:     15 * time.Second,
		MaxConnectionAge:      30 * time.Second,
		MaxConnectionAgeGrace: 5 * time.Second,
		Time:                  5 * time.Second,
		Timeout:               1 * time.Second,
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}

	srv := grpc.NewServer(
		grpc.KeepaliveEnforcementPolicy(keep),
		grpc.KeepaliveParams(kasp),
	)

	pb.RegisterHelloServiceServer(srv, new(HelloService))

	if err := srv.Serve(listener); err != nil {
		panic(err)
	}
}
