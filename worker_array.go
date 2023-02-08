package gopool

import (
	"errors"
	"time"
)

var (
	// errQueueIsFull will be returned when the worker queue is full.
	errQueueIsFull = errors.New("the queue is full")

	// errQueueIsReleased will be returned when trying to insert item to a released worker queue.
	errQueueIsReleased = errors.New("the queue length is zero")
)

type workerArray interface {
	len() int
	isEmpty() bool
	insert(worker *goWorker) error                     // insert worker
	detach() *goWorker                                 // 分发 worker
	retrieveExpiry(duration time.Duration) []*goWorker // 检索 workers
	reset()
}

type arrayType int

const (
	stackType arrayType = 1 << iota
	loopQueueType
)

func newWorkerArray(aType arrayType, size int) workerArray {
	switch aType {
	case stackType:
		return newWorkerStack(size)
	case loopQueueType:
		return newWorkerLoopQueue(size)
	default:
		return newWorkerStack(size)
	}
}
