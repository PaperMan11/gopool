package gopool

import (
	"context"
	"gopool/spinlock"
	"sync"
	"sync/atomic"
	"time"
)

type Pool struct {
	// 池的容量，负数表示池的容量是无限的，
	// 无限的池是为了避免由于嵌套使用池而导致的无休止阻塞的潜在问题:
	// 向池提交一个任务，该任务将提交一个新任务到同一个池。
	capacity int32

	// running is the number of the currently running goroutines.
	running int32

	// spinlock
	lock sync.Locker

	// workers is a slice that store the available workers.
	workers workerArray

	// state is used to notice the pool to closed itself.
	state int32

	// cond for waiting to get an idle worker.
	cond *sync.Cond

	// workerCache加快了函数retrieveWorker中可用worker的获取速度。
	workerCache sync.Pool

	// waiting是pool.Submit()上已经被阻塞的goroutine的数量，由pool.lock保护
	waiting int32

	heartbeatDone int32
	stopHeartbeat context.CancelFunc

	now atomic.Value

	options *Options
}

// purgeStaleWorkers 定期清除过期的 worker，它作为一个清道夫在一个单独的 goroutine
func (p *Pool) purgeStaleWorkers(ctx context.Context) {
	heartbeat := time.NewTicker(p.options.ExpiryDuration)

	defer func() {
		heartbeat.Stop()
		atomic.StoreInt32(&p.heartbeatDone, 1)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
		}

		if p.IsClosed() {
			break
		}

		p.lock.Lock()
		expiredWorkers := p.workers.retrieveExpiry(p.options.ExpiryDuration)
		p.lock.Unlock()

		// 通知过时的worker停止工作。
		// 这个通知必须在p.lock之外，因为w.task可能被阻塞，
		// 如果许多worker位于非本地cpu上，可能会消耗大量时间。
		for i := range expiredWorkers {
			expiredWorkers[i].task <- nil
			expiredWorkers[i] = nil
		}

		// 可能存在这样一种情况，即所有的调用方都已被清理(没有任何调用方正在运行)，
		// 或者池容量已调高，而一些调用方仍然卡在“p.cond.wait()”中，
		// 那么它应该唤醒所有这些调用方。
		if p.Running() == 0 || (p.Waiting() > 0 && p.Free() > 0) {
			p.cond.Broadcast()
		}
	}
}

func NewPool(size int, options ...Option) (*Pool, error) {
	opts := loadOptions(options...)

	if size <= 0 {
		size = -1
	}

	if !opts.DisablePurge {
		if expiry := opts.ExpiryDuration; expiry < 0 {
			return nil, ErrInvalidPoolExpiry
		} else if expiry == 0 {
			opts.ExpiryDuration = DefaultCleanIntervalTime
		}
	}

	if opts.Logger == nil {
		opts.Logger = defaultLogger
	}

	p := &Pool{
		capacity: int32(size),
		lock:     spinlock.NewSpinLock(),
		options:  opts,
	}

	p.workerCache.New = func() any {
		return &goWorker{
			pool: p,
			task: make(chan func(), workerChanCap),
		}
	}

	if p.options.PreAlloc {
		if size == -1 {
			return nil, ErrInvalidPreAllocSize
		}
		p.workers = newWorkerArray(loopQueueType, size)
	} else {
		p.workers = newWorkerArray(stackType, 0)
	}

	p.cond = sync.NewCond(p.lock)

	// Start a goroutine to clean up expired workers periodically.
	var ctx context.Context
	ctx, p.stopHeartbeat = context.WithCancel(context.Background())
	if !p.options.DisablePurge {
		go p.purgeStaleWorkers(ctx)
	}

	p.now.Store(time.Now())
	// Ticktock 是一个定期更新池中当前时间的goroutine
	go p.ticktock()

	return p, nil
}

// Ticktock 是一个定期更新池中当前时间的 goroutine
func (p *Pool) ticktock() {
	ticker := time.NewTicker(nowTimeUpdateInterval)

	defer ticker.Stop()

	for range ticker.C {
		p.now.Store(time.Now())
	}
}

func (p *Pool) nowTime() time.Time {
	return p.now.Load().(time.Time)
}

// ---------------------------------------------------------------------------

// Submit 将任务提交到此池。
// 请注意，您可以从当前的Pool. submit()调用Pool. submit()，
// 但需要特别注意的是，一旦当前Pool的容量耗尽，您将被最新的Pool.submit()调用阻塞，
// 为了避免这种情况，您应该使用ants.WithNonblocking(true)实例化一个Pool。
func (p *Pool) Submit(task func()) error {
	if p.IsClosed() {
		return ErrPoolClosed
	}
	var w *goWorker
	if w = p.retrieveWorker(); w == nil { // 返回一个可用的 worker
		return ErrPoolOverload
	}
	w.task <- task
	return nil
}

// Running returns the number of workers currently running.
func (p *Pool) Running() int {
	return int(atomic.LoadInt32(&p.running))
}

// Free 返回可用的goroutine的数量，-1表示这个池是无限的
func (p *Pool) Free() int {
	c := p.Cap()
	if c < 0 {
		return -1
	}
	return c - p.Running()
}

// Waiting returns the number of tasks which are waiting be executed.
func (p *Pool) Waiting() int {
	return int(atomic.LoadInt32(&p.waiting))
}

// Cap returns the capacity of this pool.
func (p *Pool) Cap() int {
	return int(atomic.LoadInt32(&p.capacity))
}

// 调优将更改 pool 的容量，请注意，它对无限池或预分配池无效
func (p *Pool) Tune(size int) {
	capacity := p.Cap()
	if capacity == -1 || size <= 0 || size == capacity || p.options.PreAlloc {
		return
	}

	atomic.StoreInt32(&p.capacity, int32(size))
	if size > capacity {
		if size-capacity == 1 {
			p.cond.Signal()
			return
		}
		p.cond.Broadcast()
	}
}

// IsClosed indicates whether the pool is closed.
func (p *Pool) IsClosed() bool {
	return atomic.LoadInt32(&p.state) == CLOSED
}

// Release closes this pool and releases the worker queue.
func (p *Pool) Release() {
	if !atomic.CompareAndSwapInt32(&p.state, OPENED, CLOSED) {
		return
	}
	p.lock.Lock()
	p.workers.reset()
	p.lock.Unlock()
	// There might be some callers waiting in retrieveWorker(), so we need to wake them up to prevent
	// those callers blocking infinitely.
	p.cond.Broadcast()
}

// ReleaseTimeout is like Release but with a timeout,
// it waits all workers to exit before timing out.
func (p *Pool) ReleaseTimeout(timeout time.Duration) error {
	if p.IsClosed() || p.stopHeartbeat == nil {
		return ErrPoolClosed
	}

	p.stopHeartbeat()
	p.stopHeartbeat = nil
	p.Release()

	endTime := time.Now().Add(timeout)
	for time.Now().Before(endTime) {
		if p.Running() == 0 && (p.options.DisablePurge || atomic.LoadInt32(&p.heartbeatDone) == 1) {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return ErrTimeout
}

// Reboot reboots a closed pool.
func (p *Pool) Reboot() {
	if atomic.CompareAndSwapInt32(&p.state, CLOSED, OPENED) {
		atomic.StoreInt32(&p.heartbeatDone, 0)
		var ctx context.Context
		ctx, p.stopHeartbeat = context.WithCancel(context.Background())
		if !p.options.DisablePurge {
			go p.purgeStaleWorkers(ctx)
		}
	}
}

// ---------------------------------------------------------------------------

func (p *Pool) addRunning(delta int) {
	atomic.AddInt32(&p.running, int32(delta))
}

func (p *Pool) addWaiting(delta int) {
	atomic.AddInt32(&p.waiting, int32(delta))
}

// retrieveWorker returns an available worker to run the tasks.
func (p *Pool) retrieveWorker() (w *goWorker) {
	spawnWorker := func() {
		w = p.workerCache.Get().(*goWorker)
		w.run()
	}

	p.lock.Lock()

	w = p.workers.detach()
	if w != nil { // first try to fetch the worker from the queue
		p.lock.Unlock()
	} else if capacity := p.Cap(); capacity == -1 || capacity > p.Running() {
		// if the worker queue is empty and we don't run out of the pool capacity,
		// then just spawn a new worker goroutine.
		p.lock.Unlock()
		spawnWorker()
	} else { // 否则，我们将不得不阻止它们，并等待至少一个worker被放回池中
		if p.options.Nonblocking {
			p.lock.Unlock()
			return
		}
	retry:
		if p.options.MaxBlockingTasks != 0 && p.Waiting() >= p.options.MaxBlockingTasks {
			p.lock.Unlock()
			return
		}
		p.addWaiting(1)
		p.cond.Wait() // block and wait for an available worker
		p.addWaiting(-1)

		if p.IsClosed() {
			p.lock.Unlock()
			return
		}

		var nw int
		if nw = p.Running(); nw == 0 {
			p.lock.Unlock()
			spawnWorker()
			return
		}
		if w = p.workers.detach(); w == nil {
			if nw < p.Cap() {
				p.lock.Unlock()
				spawnWorker()
				return
			}
			goto retry
		}
		p.lock.Unlock()
	}
	return
}

// revertWorker 将一个 worker 放回空闲池，回收 goroutines
func (p *Pool) revertWorker(worker *goWorker) bool {
	if capacity := p.Cap(); (capacity > 0 && p.Running() > capacity) || p.IsClosed() {
		p.cond.Broadcast()
		return false
	}

	worker.recycleTime = p.nowTime()
	p.lock.Lock()

	// To avoid memory leaks, add a double check in the lock scope.
	// Issue: https://github.com/panjf2000/ants/issues/113
	if p.IsClosed() {
		p.lock.Unlock()
		return false
	}

	err := p.workers.insert(worker)
	if err != nil {
		p.lock.Unlock()
		return false
	}

	// notify
	p.cond.Signal()
	p.lock.Unlock()
	return true
}
