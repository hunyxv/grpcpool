package grpcpool

import (
	"errors"
	"math"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hunyxv/grpcpool/internal"
	"github.com/prometheus/client_golang/prometheus"

	"google.golang.org/grpc"
)

var (
	// ErrPoolClosed pool 连接已关闭
	ErrPoolClosed = errors.New("grpc pool has closed")

	// ErrConnClosed grpc 连接已关闭
	ErrConnClosed = errors.New("the grpc connection has closed")

	// ErrPoolOverload 连接池资源已满载
	ErrPoolOverload = errors.New("pool overload")

	// errgrpcOverload grpc clientConn 已满载
	errGrpcOverload = errors.New("grpc overload")
)

const (
	// OPENED represents that the pool is opened.
	OPENED = iota

	// CLOSED represents that the pool is closed.
	CLOSED
)

// Builder 创建conn的构造函数
type Builder func() (*grpc.ClientConn, error)

// Pool grpc 连接池
type Pool struct {
	state   int32
	mux     *sync.RWMutex
	cond    *sync.Cond
	opt     *option
	conns   []*grpcConn
	builder Builder

	r  *rand.Rand
	ch chan struct{}
	noCopy
}

// NewPool create a grpc pool
func NewPool(builder Builder, opts ...Option) (pool *Pool, err error) {
	opt := getDefaultOpt()
	for _, f := range opts {
		f(opt)
	}

	if opt.Debug {
		prometheus.MustRegister(statistics)
		//prometheus.MustRegister(statistics_put)
		prometheus.MustRegister(connection)
	}

	pool = &Pool{
		mux:     new(sync.RWMutex),
		builder: builder,
		cond:    sync.NewCond(internal.NewSpinLock()),
		conns:   make([]*grpcConn, 0, opt.MaxIdle),
		opt:     opt,
		r:       rand.New(rand.NewSource(time.Now().UnixNano())),
		ch:      make(chan struct{}, 0),
	}

	for i := 0; i < pool.opt.MaxIdle; i++ {
		conn, err := pool.builder()
		if err != nil {
			return nil, err
		}

		gconn := newGrpcConn(pool, conn)
		pool.conns = append(pool.conns, gconn)
		if opt.Debug {
			connection.WithLabelValues("conn").Add(1)
		}
	}

	go pool.cleanPeriodically()
	return
}

// Get get a grpc logic connection
func (p *Pool) Get() (logicconn LogicConn, err error) {
	if atomic.LoadInt32(&p.state) == CLOSED {
		return nil, ErrPoolClosed
	}

	p.mux.RLock()

	l := len(p.conns)
	if l <= p.opt.MaxIdle {
		logicconn, err = p.conns[time.Now().UnixNano()%int64(l)].get()
		p.mux.RUnlock()
		if err == errGrpcOverload {
			err = p.createNewGrpcConn(l)
			if err != nil {
				return
			}
			runtime.Gosched()
			return p.Get()
		}
		if p.opt.Debug {
			statistics.WithLabelValues("get").Add(1)
		}
		return logicconn, nil
	}

	l = int(math.Round(float64(l) * 0.8))
	index := time.Now().UnixNano() % int64(l)
	logicconn, err = p.conns[index].get()
	if err == ErrConnClosed || err == errGrpcOverload {
		for i := int(index) + 1; i < len(p.conns); i++ {
			logicconn, err = p.conns[i].get()
			if err != nil {
				continue
			}
		}
		if logicconn == nil {
			ll := len(p.conns)
			p.mux.RUnlock()
			err = p.createNewGrpcConn(ll)
			if err != nil {
				return
			}
			runtime.Gosched()
			return p.Get()
		}
	}
	p.mux.RUnlock()
	if p.opt.Debug {
		statistics.WithLabelValues("get").Add(1)
	}
	return
}

// Put release grpc logic connection
func (p *Pool) Put(lc LogicConn) {
	if atomic.LoadInt32(&p.state) == CLOSED {
		return
	}

	logicconn := lc.(logicConn)
	grpcconn := logicconn.gconn
	grpcconn.recycle(logicconn)
	if p.opt.Debug {
		statistics.WithLabelValues("put").Add(1)
	}
}

func (p *Pool) cleanPeriodically() {
	heartbeat := time.NewTicker(p.opt.CleanIntervalTime)
	defer heartbeat.Stop()

	for {
		select {
		case <-heartbeat.C:
			p.mux.Lock()
			for i := len(p.conns); i < p.opt.MaxIdle; i++ {
				conn, err := p.builder()
				if err != nil {
					panic(err.Error())
				}

				gconn := newGrpcConn(p, conn)
				p.conns = append(p.conns, gconn)
			}

			var idleCount int
			l := len(p.conns)
			for i := 0; i < l; {
				if p.conns[i].isClosed() || p.conns[i].isTimeout() {
					grpcconn := p.conns[i]
					if err := grpcconn.close(); err != nil {
						p.opt.Logger.Printf("warning: %s\n", err.Error())
					}
					copy(p.conns[i:], p.conns[i+1:])
					p.conns[l-1] = nil
					p.conns = p.conns[:l-1]
					l--
					continue
				}

				if p.conns[i].isIdle() {
					idleCount++
					if idleCount > p.opt.MaxIdle {
						grpcconn := p.conns[i]
						if err := grpcconn.close(); err != nil {
							p.opt.Logger.Printf("warning: %s\n", err.Error())
						}
						copy(p.conns[i:], p.conns[i+1:])
						p.conns[l-1] = nil
						p.conns = p.conns[:l-1]
						l--
						if p.opt.Debug {
							connection.WithLabelValues("conn").Sub(1)
						}
						continue
					}
				}
				i++
			}
			p.opt.Logger.Printf("conn: %d", len(p.conns))
			p.mux.Unlock()
		case <-p.ch:
			return
		}
	}
}

// Close close pool
func (p *Pool) Close() {
	p.mux.Lock()
	defer p.mux.Unlock()

	close(p.ch)
	conns := p.conns
	for _, conn := range conns {
		err := conn.close()
		if err != nil {
			p.opt.Logger.Printf("warning: %s\n", err.Error())
		}
	}

	p.conns = p.conns[:0]
	p.state = CLOSED
	return
}

func (p *Pool) createNewGrpcConn(l int) (err error) {
	if l != len(p.conns) {
		return
	}

	p.mux.Lock()
	defer p.mux.Unlock()

	if l != len(p.conns) || len(p.conns) > p.opt.GrpcPoolSize {
		return
	}

	clientConn, err := p.builder()
	if err != nil {
		return
	}
	if p.opt.Debug {
		connection.WithLabelValues("conn").Add(1)
	}
	p.conns = append(p.conns, newGrpcConn(p, clientConn))
	return
}

type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) UnLock() {}
