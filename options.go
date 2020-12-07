package grpcpool

import (
	"log"
	"math"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	defaultGrpcPoolSize      = math.MaxInt32
	defaultMaxStreamsClient  = 100
	defaultMaxIdle           = 3
	defaultCleanIntervalTime = time.Second
	defaultClientIdleTimeout = time.Minute
)

// Logger is used for logging formatted messages.
type Logger interface {
	// Printf must have the same semantics as log.Printf.
	Printf(format string, args ...interface{})
}

// Option .
type Option func(*option)

type option struct {
	// Pool size
	GrpcPoolSize int

	// MaxStreamsClient http2 defaultMaxStreamsClient = 100
	MaxStreamsClient int

	// Maximum number of idle connections in the pool
	MaxIdle int

	// CleanIntervalTime is the interval time to clean idle connections
	CleanIntervalTime time.Duration

	// ClientIdleTimeout idle connection timeout
	ClientIdleTimeout time.Duration

	// When Nonblocking is true, Pool.Get will never be blocked.
	// ErrPoolOverload will be returned when Pool is exhausted.
	Nonblocking bool

	// Logger is the customized logger for logging info, if it is not set,
	// default standard logger from log package is used.
	Logger Logger

	// Debug
	Debug bool
}

var defaultOption = option{
	GrpcPoolSize:      defaultGrpcPoolSize,
	MaxStreamsClient:  defaultMaxStreamsClient,
	MaxIdle:           defaultMaxIdle,
	ClientIdleTimeout: defaultClientIdleTimeout,
	CleanIntervalTime: defaultCleanIntervalTime,
	Logger:            Logger(log.New(os.Stderr, "", log.LstdFlags)),
}

func getDefaultOpt() *option {
	opt := defaultOption
	return &opt
}

// WithGrpcPoolSize returns a Option which sets the value for pool size
func WithGrpcPoolSize(size int) Option {
	return func(opt *option) {
		opt.GrpcPoolSize = size
	}
}

// WithMaxStreamsClient returns a Option which set the value for
// http2 client maxConcurrentStreams
func WithMaxStreamsClient(num int) Option {
	return func(opt *option) {
		opt.MaxStreamsClient = num
	}
}

// WithMaxIdle set number of idle connections in the pool.
func WithMaxIdle(num int) Option {
	return func(opt *option) {
		opt.MaxIdle = num
	}
}

// WithCleanIntervalTime set interval time to clean up
// idle connections or create new tcp connection
func WithCleanIntervalTime(t time.Duration) Option {
	return func(opt *option) {
		opt.CleanIntervalTime = t
	}
}

// WithClientIdleTimeout .
func WithClientIdleTimeout(t time.Duration) Option {
	return func(opt *option) {
		opt.ClientIdleTimeout = t
	}
}

// WithNonblocking returns a Option which Pool.Get can never be blocked.
// ErrPoolOverload will be returned when Pool is exhausted.
func WithNonblocking() Option {
	return func(opt *option) {
		opt.Nonblocking = true
	}
}

// WithLogger returns a Option which sets the value for pool logger
func WithLogger(logger Logger) Option {
	return func(opt *option) {
		opt.Logger = logger
	}
}

// WithDebug .
func WithDebug() Option {
	return func(opt *option) {
		opt.Debug = true
	}
}

var (
	statistics = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "statistics",
			Help: "Number of 'get' and 'put'",
		},
		[]string{"get_put"},
	)

	connection = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "connection",
			Help: "Number of connection",
		},
		[]string{"conn"},
	)
)
