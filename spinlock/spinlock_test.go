package spinlock

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

/*
Benchmark result for three types of locks:
	goos: linux
	goarch: amd64
	pkg: gopool/spinlock
	cpu: Intel(R) Core(TM) i5-10500 CPU @ 3.10GHz
	BenchmarkMutex-2        		43780077                26.40 ns/op            0 B/op          0 allocs/op
	BenchmarkSpinLock-2     		54831673                21.73 ns/op            0 B/op          0 allocs/op
	BenchmarkBackOffSpinLock-2      83039551                14.51 ns/op            0 B/op          0 allocs/op
*/

type originSpinLock uint32

func (sl *originSpinLock) Lock() {
	for !atomic.CompareAndSwapUint32((*uint32)(sl), 0, 1) {
		runtime.Gosched()
	}
}

func (sl *originSpinLock) Unlock() {
	atomic.StoreUint32((*uint32)(sl), 0)
}

func NewOriginSpinLock() sync.Locker {
	return new(originSpinLock)
}

func BenchmarkMutex(b *testing.B) {
	m := sync.Mutex{}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			m.Lock()
			//nolint:staticcheck
			m.Unlock()
		}
	})
}

func BenchmarkSpinLock(b *testing.B) {
	spin := NewOriginSpinLock()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			spin.Lock()
			//nolint:staticcheck
			spin.Unlock()
		}
	})
}

func BenchmarkBackOffSpinLock(b *testing.B) {
	spin := NewSpinLock()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			spin.Lock()
			//nolint:staticcheck
			spin.Unlock()
		}
	})
}
