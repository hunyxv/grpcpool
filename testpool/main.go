package main

import (
	"fmt"
	"grpcpool"
	"math/rand"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

func init() {
	rand.Seed(time.Now().Unix())
}


func main() {
	go startServer()

	t := time.Now()
	// builder := func() (*grpc.ClientConn, error) {
	// 	return nil, nil
	// }

	pool := grpcpool.New(builder)
	go func(pool *grpcpool.Pool){
		for {
			pool.Debug()
			time.Sleep(time.Second)
		}
	}(pool)
		
	//for{
	wg := new(sync.WaitGroup)
	limiter := rate.NewLimiter(rate.Limit(100000), 1)
	for k := 0; k < 10000; k++ {
		wg.Add(1)
		go do(pool, limiter, wg)
	}
	wg.Wait()
	pool.Debug()
	fmt.Println(time.Now().Sub(t))
	//}
}

func do(pool *grpcpool.Pool, limiter *rate.Limiter, wg *sync.WaitGroup) {
	defer wg.Done()

	for k :=0; k < 1000; k++ {
		if !limiter.Allow(){
			time.Sleep(limiter.Reserve().Delay())
		}

		sub, err := pool.Get()
		if err != nil {
			panic(err)
		}

		// time.Sleep(time.Millisecond * 10)//time.Duration(rand.Intn(15)))
		get(sub)
		pool.Put(sub)
	}
}