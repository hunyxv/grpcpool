package grpcpool

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/hunyxv/grpcpool/internal"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// LogicConn grpc 逻辑连接接口
type LogicConn interface {
	Conn() grpc.ClientConnInterface
	t()
}

var _ LogicConn = (*logicConn)(nil)

type logicConn struct {
	grpc.ClientConnInterface

	gconn *grpcConn
}

func (lc logicConn) Conn() grpc.ClientConnInterface {
	return lc
}

func (logicConn) t() {}

var logicConnPool = sync.Pool{
	New: func() interface{} { return logicConn{} },
}

type grpcConn struct {
	p    *Pool
	conn *grpc.ClientConn

	maxStreamsClient  int
	clientIdleTimeout time.Duration
	current           int32 // 当前剩余可用
	lock              sync.Locker
	ts                time.Time
}

func newGrpcConn(p *Pool, conn *grpc.ClientConn) *grpcConn {
	return &grpcConn{
		p:                 p,
		conn:              conn,
		maxStreamsClient:  p.opt.MaxStreamsClient,
		clientIdleTimeout: p.opt.ClientIdleTimeout,
		current:           int32(p.opt.MaxStreamsClient),
		lock:              internal.NewSpinLock(),
		ts:                time.Now(),
	}
}

func (gc *grpcConn) get() (lc LogicConn, err error) {
	current := atomic.LoadInt32(&gc.current)
	if current == 0 {
		err = errGrpcOverload
		return
	}

	gc.lock.Lock()
	defer gc.lock.Unlock()

	if gc.conn.GetState() == connectivity.Shutdown {
		err = ErrConnClosed
		return
	}

	if gc.current == 0 {
		err = errGrpcOverload
		return
	}

	gc.ts = time.Now()
	gc.current--

	logicconn := logicConnPool.Get().(logicConn)
	logicconn.gconn = gc
	logicconn.ClientConnInterface = gc.conn
	return logicconn, nil
}

func (gc *grpcConn) recycle(lc logicConn) {
	current := atomic.AddInt32(&gc.current, 1)
	if int(current) > gc.maxStreamsClient {
		panic("Unknown error")
	}

	lc.gconn = nil
	lc.ClientConnInterface = nil
	logicConnPool.Put(lc)
}

func (gc *grpcConn) isClosed() bool {
	return gc.conn.GetState() == connectivity.Shutdown
}

func (gc *grpcConn) isIdle() bool {
	return int(atomic.LoadInt32(&gc.current)) == gc.maxStreamsClient
}

func (gc *grpcConn) isTimeout() bool {
	return time.Now().Sub(gc.ts) > gc.clientIdleTimeout
}

func (gc *grpcConn) close() (err error) {
	gc.lock.Lock()
	defer gc.lock.Unlock()

	err = gc.conn.Close()
	if err != nil {
		return
	}
	return
}
