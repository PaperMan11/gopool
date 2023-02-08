package gopool

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewWorkerStack(t *testing.T) {
	size := 100
	stack := newWorkerStack(size)
	assert.EqualValues(t, 0, stack.len(), "len error")
	assert.Equal(t, true, stack.isEmpty(), "isEmpty error")
	assert.Nil(t, stack.detach(), "dequeue error")
}

func TestWorkerStack(t *testing.T) {
	stack := newWorkerArray(arrayType(-1), 0)

	for i := 0; i < 5; i++ {
		err := stack.insert(&goWorker{recycleTime: time.Now()})
		if err != nil {
			break
		}
	}
	assert.EqualValues(t, 5, stack.len(), "len error")

	expired := time.Now()

	err := stack.insert(&goWorker{recycleTime: expired})
	if err != nil {
		t.Fatal("Enqueue error")
	}

	time.Sleep(time.Second)

	for i := 0; i < 6; i++ {
		err := stack.insert(&goWorker{recycleTime: time.Now()})
		if err != nil {
			t.Fatal("Enqueue error")
		}
	}
	assert.EqualValues(t, 12, stack.len(), "Len error")
	stack.retrieveExpiry(time.Second)
	assert.EqualValues(t, 6, stack.len(), "Len error")
}

func TestSearch(t *testing.T) {
	q := newWorkerStack(0)

	// 1
	expiry1 := time.Now()

	_ = q.insert(&goWorker{recycleTime: time.Now()})
	assert.EqualValues(t, 0, q.binarySearch(0, q.len()-1, time.Now()), "index should be 0")
	assert.EqualValues(t, -1, q.binarySearch(0, q.len()-1, expiry1), "index should be -1")

	// 2
	expiry2 := time.Now()
	_ = q.insert(&goWorker{recycleTime: time.Now()})
	assert.EqualValues(t, -1, q.binarySearch(0, q.len()-1, expiry1), "index should be -1")
	assert.EqualValues(t, 0, q.binarySearch(0, q.len()-1, expiry2), "index should be 0")
	assert.EqualValues(t, 1, q.binarySearch(0, q.len()-1, time.Now()), "index should be 1")

	// more
	for i := 0; i < 5; i++ {
		_ = q.insert(&goWorker{recycleTime: time.Now()})
	}

	expiry3 := time.Now()

	_ = q.insert(&goWorker{recycleTime: expiry3})

	for i := 0; i < 10; i++ {
		_ = q.insert(&goWorker{recycleTime: time.Now()})
	}

	assert.EqualValues(t, 7, q.binarySearch(0, q.len()-1, expiry3), "index should be 7")

}
