package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/hunyxv/grpcpool"

	"github.com/hunyxv/grpcpool/testpool/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

func builder() (*grpc.ClientConn, error) {
	var addr string = "127.0.0.1:8080"

	var kacp = keepalive.ClientParameters{
		Time:                10 * time.Second,
		Timeout:             time.Second,
		PermitWithoutStream: true,
	}

	return grpc.Dial(addr,
		grpc.WithTimeout(time.Second*2),
		grpc.WithBlock(),
		grpc.WithInsecure(),
		grpc.WithKeepaliveParams(kacp),
	)
}

func sayHello(conn grpcpool.LogicConn) {
	svrClient := pb.NewHelloServiceClient(conn.Conn())
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	resp, err := svrClient.SayHello(ctx, &pb.HelloRequest{Name: "Lixu"})
	if err == grpc.ErrClientConnClosing {
		log.Fatal("call server's SayHello err", err)
	}
	if err != nil {
		log.Fatal("call server's unary err", err)
	}
	fmt.Println(resp.Msg)
}
