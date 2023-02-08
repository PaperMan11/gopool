package gopool

import (
	"errors"
	"log"
	"math"
	"os"
	"runtime"
	"time"
)

const (
	DefaultAntsPoolSize      = math.MaxInt32
	DefaultCleanIntervalTime = time.Second
)

const (
	OPENED = iota
	CLOSED
)

var (
	// 当调用者没有为池提供函数时，将返回 ErrLackPoolFunc
	ErrLackPoolFunc = errors.New("must provide function for pool")

	// 当将一个负数设置为清除某个例程的周期持续时间时，将返回 ErrInvalidPoolExpiry
	ErrInvalidPoolExpiry = errors.New("invalid expiry for pool")

	// 将任务提交到关闭池时将返回 ErrPoolClosed
	ErrPoolClosed = errors.New("this pool has been closed")

	// 当池已满且没有可用的 worker 时，将返回 ErrPoolOverload
	ErrPoolOverload = errors.New("too many goroutines blocked on submit or Nonblocking is set")

	// 当尝试在 PreAlloc 模式下设置负容量时，将返回 ErrInvalidPreAllocSize
	ErrInvalidPreAllocSize = errors.New("can not set up a negative capacity under PreAlloc mode")

	// 操作超时后将返回ErrTimeout
	ErrTimeout = errors.New("operation timed out")

	// workerChanCap determines whether the channel of a worker should be a buffered channel
	// to get the best performance. Inspired by fasthttp at
	// https://github.com/valyala/fasthttp/blob/master/workerpool.go#L139
	workerChanCap = func() int {
		// Use blocking channel if GOMAXPROCS=1.
		// This switches context from sender to receiver immediately,
		// which results in higher performance (under go1.5 at least).
		if runtime.GOMAXPROCS(0) == 1 {
			return 0
		}

		// Use non-blocking workerChan if GOMAXPROCS>1,
		// since otherwise the sender might be dragged down if the receiver is CPU-bound.
		return 1
	}()

	defaultLogger = Logger(log.New(os.Stderr, "", log.LstdFlags))

	// Init an instance pool when importing ants.
	defaultAntsPool, _ = NewPool(DefaultAntsPoolSize)
)

const nowTimeUpdateInterval = 500 * time.Millisecond

type Logger interface {
	Printf(format string, args ...interface{})
}

func Submit(task func()) error {
	return defaultAntsPool.Submit(task)
}

func Running() int {
	return defaultAntsPool.Running()
}

func Cap() int {
	return defaultAntsPool.Cap()
}

func Free() int {
	return defaultAntsPool.Free()
}

func Release() {
	defaultAntsPool.Release()
}

func Reboot() {
	defaultAntsPool.Reboot()
}
