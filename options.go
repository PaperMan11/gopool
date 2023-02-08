package gopool

import "time"

type Option func(opts *Options)

func loadOptions(options ...Option) *Options {
	opts := new(Options)
	for _, option := range options {
		option(opts)
	}
	return opts
}

type Options struct {
	// ticker 定期清理过期的 worker
	ExpiryDuration time.Duration

	// PreAlloc 初始化 Pool 时是否预分配内存(worker)
	PreAlloc bool

	// Max number of goroutine blocking on pool.Submit.
	// 0 (default value) means no such limit.
	MaxBlockingTasks int

	// 当 Nonblocking 为 true 时，Pool 提交永远不会被阻止。
	// 当 Pool.error 时返回 ErrPoolOverload，不能立即提交。
	// 当 Nonblocking 为 true 时，MaxBlockingTasks 无效。
	Nonblocking bool

	// PanicHandler is used to handle panics from each worker goroutine.
	// if nil, panics will be thrown out again from worker goroutines.
	PanicHandler func(interface{})

	Logger Logger

	// 当DisablePurge为true时，workers 不会被清除，而是常驻
	DisablePurge bool
}

func WithOptions(options Options) Option {
	return func(opts *Options) {
		*opts = options
	}
}

// WithExpiryDuration 设置清理 goroutine 的间隔时间
func WithExpiryDuration(expiryDuration time.Duration) Option {
	return func(opts *Options) {
		opts.ExpiryDuration = expiryDuration
	}
}

// WithPreAlloc 指示它是否应该为 worker malloc
func WithPreAlloc(preAlloc bool) Option {
	return func(opts *Options) {
		opts.PreAlloc = preAlloc
	}
}

// WithMaxBlockingTasks 设置当达到池容量时阻塞的 goroutine 的最大数量
func WithMaxBlockingTasks(maxBlockingTasks int) Option {
	return func(opts *Options) {
		opts.MaxBlockingTasks = maxBlockingTasks
	}
}

// WithNonblocking 表示当没有可用的 worker 时，池将返回 nil
func WithNonblocking(nonblocking bool) Option {
	return func(opts *Options) {
		opts.Nonblocking = nonblocking
	}
}

// WithPanicHandler sets up panic handler.
func WithPanicHandler(panicHandler func(interface{})) Option {
	return func(opts *Options) {
		opts.PanicHandler = panicHandler
	}
}

// WithLogger sets up a customized logger.
func WithLogger(logger Logger) Option {
	return func(opts *Options) {
		opts.Logger = logger
	}
}

// WithDisablePurge 指示是否关闭自动清除
func WithDisablePurge(disable bool) Option {
	return func(opts *Options) {
		opts.DisablePurge = disable
	}
}
