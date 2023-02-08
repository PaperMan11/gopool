package gopool

import (
	"runtime"
	"time"
)

// goWorker是运行任务的实际执行程序，它启动一个接受任务并执行函数调用的goroutine。
type goWorker struct {
	pool        *Pool       // pool who owns this worker.
	task        chan func() // task is a job should be done.
	recycleTime time.Time   // 当将一个worker放回队列时，recycleTime将被更新。
}

// Run 启动一个 goroutine 来重复执行函数调用的过程
func (w *goWorker) run() {
	w.pool.addRunning(1)
	go func() {
		defer func() {
			w.pool.addRunning(-1)
			w.pool.workerCache.Put(w)
			if p := recover(); p != nil {
				if ph := w.pool.options.PanicHandler; ph != nil {
					ph(p)
				} else {
					w.pool.options.Logger.Printf("worker exits from a panic: %v\n", p)
					var buf [4096]byte
					n := runtime.Stack(buf[:], false)
					w.pool.options.Logger.Printf("worker exits from panic: %s\n", string(buf[:n]))
				}
			}
			// 在这里调用Signal()，以防有goroutines等待可用的worker。
			w.pool.cond.Signal()
		}()

		for f := range w.task {
			if f == nil {
				return
			}
			f()
			if ok := w.pool.revertWorker(w); !ok {
				return
			}
		}
	}()
}
