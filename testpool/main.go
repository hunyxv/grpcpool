package main

import (
	"log"
	"math"
	"math/rand"
	"net/http"
	"time"

	"github.com/hunyxv/grpcpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"golang.org/x/time/rate"
)

func init() {
	rand.Seed(time.Now().Unix())
}

func main2() {
	go startServer()

	time.Sleep(time.Second)

}

func main() {

	go startServer()

	pool, err := grpcpool.NewPool(builder, grpcpool.WithMaxIdle(3), grpcpool.WithDebug())
	if err != nil {
		panic(err)
	}

	limiter := rate.NewLimiter(rate.Limit(100000), 100000)

	go func() {
		for {
			for radian := float64(0); radian < 3.2; radian += 0.1 {
				limiter.SetLimit(rate.Limit(math.Sin(radian) * 100000))
				time.Sleep(time.Second * 3)
			}
		}
	}()

	for k := 0; k < 100; k++ {
		go do(pool, limiter)
	}

	http.Handle("/metrics", promhttp.Handler())
	log.Println("start...")
	if err := http.ListenAndServe("0.0.0.0:8090", nil); err != nil {
		log.Panic(err)
	}
}

func do(pool *grpcpool.Pool, limiter *rate.Limiter) {
	for {
		if !limiter.Allow() {
			time.Sleep(limiter.Reserve().Delay())
		}

		sub, err := pool.Get()
		if err != nil {
			log.Fatalf("err: %s", err.Error())
		}

		// time.Sleep(time.Millisecond * 10)//time.Duration(rand.Intn(15)))
		sayHello(sub)
		pool.Put(sub)
	}
}
