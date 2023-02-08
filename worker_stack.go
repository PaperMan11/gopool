package gopool

import "time"

type workerStack struct {
	items  []*goWorker
	expiry []*goWorker // 到期时间（待清理的 worker）
}

func newWorkerStack(size int) *workerStack {
	return &workerStack{
		items: make([]*goWorker, 0),
	}
}

func (ws *workerStack) len() int {
	return len(ws.items)
}

func (ws *workerStack) isEmpty() bool {
	return len(ws.items) == 0
}

func (ws *workerStack) insert(worker *goWorker) error {
	ws.items = append(ws.items, worker)
	return nil
}

func (ws *workerStack) detach() *goWorker {
	l := ws.len()
	if l == 0 {
		return nil
	}

	w := ws.items[l-1]
	ws.items[l-1] = nil // avoid memory leaks
	ws.items = ws.items[:l-1]

	return w
}

func (ws *workerStack) retrieveExpiry(duration time.Duration) []*goWorker {
	n := ws.len()
	if n == 0 {
		return nil
	}

	expiryTime := time.Now().Add(-duration)
	index := ws.binarySearch(0, n-1, expiryTime)

	ws.expiry = ws.expiry[:0]
	if index != -1 {
		ws.expiry = append(ws.expiry, ws.items[:index+1]...)
		m := copy(ws.items, ws.items[index+1:])
		for i := m; i < n; i++ {
			ws.items[i] = nil
		}
		ws.items = ws.items[:m]
	}
	return ws.expiry
}

func (ws *workerStack) binarySearch(l, r int, expiryTime time.Time) int {
	var mid int
	for l <= r {
		mid = (l + r) / 2
		if expiryTime.Before(ws.items[mid].recycleTime) {
			r = mid - 1
		} else {
			l = mid + 1
		}
	}
	return r
}

func (ws *workerStack) reset() {
	for i := 0; i < ws.len(); i++ {
		ws.items[i].task <- nil
		ws.items[i] = nil
	}
	ws.items = ws.items[:0]
}
