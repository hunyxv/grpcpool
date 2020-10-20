package grpcpool

import (
	stderr "errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

var (
	ids       uint64
	// ClosedErr grpc 连接已关闭
	ClosedErr = stderr.New("grpc pool has closed")
)

const (
	defaultMaxSubsPerClient  = 100
	defaultClientIdleTimeout = time.Minute
)

type noCopy struct{}

func (*noCopy) Lock() {}

// Builder 创建conn的构造函数
type Builder func() (*grpc.ClientConn, error)

// SubClient grpc 客户端
type SubClient interface {
	Conn() (*grpc.ClientConn, error)
	t()
}

var _ SubClient = (*subClient)(nil)
var subPool = &sync.Pool{
	New: func() interface{} {
		return &subClient{}
	},
}

type subClient struct {
	client *grpcClient
	conn   *grpc.ClientConn
}

func (s *subClient) Conn() (*grpc.ClientConn, error) {
	if s.conn.GetState() == connectivity.Shutdown {
		return nil, ClosedErr
	}
	return s.conn, nil
}

func (s *subClient) t() {}

type grpcClient struct {
	id   uint64
	conn *grpc.ClientConn
	subs int64
	ts   time.Time
}

func (cc *grpcClient) createSubClient() *subClient {
	sub := subPool.Get().(*subClient)
	sub.client = cc
	sub.conn = cc.conn

	return sub
}

// Pool grpc 连接池结构体
type Pool struct {
	noCopy      noCopy
	queue       []*grpcClient
	fullloadCli *sync.Map
	mux         sync.RWMutex
	builder     Builder

	gets uint64
	puts uint64
}

// New 创建 grpc连接池
func New(builder Builder) *Pool {
	return &Pool{
		builder:     builder,
		fullloadCli: new(sync.Map),
	}
}

// Put 释放一个grpc连接
func (p *Pool) Put(s SubClient) {
	sub := s.(*subClient)
	defer func (sub *subClient){
		sub.client = nil
		sub.conn = nil
		subPool.Put(sub)
	}(sub)

	atomic.AddUint64(&(p.puts), 1)
	if _, ok := p.fullloadCli.LoadAndDelete(sub.client.id); ok {
		client := sub.client
		client.ts = time.Now()
		atomic.AddInt64(&client.subs, -1)
		p.mux.Lock()
		p.queue = append(p.queue, client)
		p.mux.Unlock()
		return
	}
	if sub.client.subs == 0 {
		panic("000000")
	}
	atomic.AddInt64(&sub.client.subs, -1)
}

// Get 拿到一个 grpc 连接
func (p *Pool) Get() (SubClient, error) {
	p.mux.Lock()
	defer p.mux.Unlock()

	if len(p.queue) == 0 {
		conn, err := p.builder()
		if err != nil {
			return nil, errors.WithMessage(err, "call builder to create grpc client conn err")
		}

		client := &grpcClient{
			id:   atomic.AddUint64(&ids, 1),
			ts:   time.Now(),
			conn: conn,
			subs: 1,
		}
		p.queue = append(p.queue, client)
		p.gets++
		return client.createSubClient(), nil
	}
	client := p.queue[len(p.queue)-1]

	next := atomic.AddInt64(&(client.subs), 1)
	if next == defaultMaxSubsPerClient {
		p.queue = p.queue[:len(p.queue)-1]
		p.fullloadCli.Store(client.id, client)
	}
	client.ts = time.Now()
	p.gets++
	if len(p.queue) > 0 && time.Since(p.queue[0].ts) > defaultClientIdleTimeout {
		c := p.queue[0]
		c.conn.Close()
		p.queue = p.queue[1:]
	}

	return client.createSubClient(), nil
}

// Close 关闭连接池
func (p *Pool) Close() error {
	p.mux.Lock()
	defer p.mux.Unlock()

	for _, client := range p.queue {
		err := client.conn.Close()
		if err != nil {
			return err
		}
	}

	var err error
	p.fullloadCli.Range(func(_, v interface{}) bool {
		client := v.(*grpcClient)
		err = client.conn.Close()
		return err == nil
	})

	return err
}

// Debug 打印 debug 信息
func (p *Pool) Debug() {
	p.mux.Lock()
	defer p.mux.Unlock()

	var fullnum, n int
	p.fullloadCli.Range(func(k, v interface{}) bool {
		c := v.(*grpcClient)
		n++
		fullnum = fullnum + int(c.subs)
		return true
	})

	// fmt.Printf("size: %d, fullload: %d\n", len(p.queue), n)

	// fmt.Println("详情：\t cid\tsubs")
	// for _, c := range p.queue {
	// 	fmt.Println("\t", c.id, "\t", c.subs)
	// }

	fmt.Printf("gets: %d, puts: %d, size: %d, fullnum: %d\n\n", p.gets, p.puts, len(p.queue), fullnum)
}
