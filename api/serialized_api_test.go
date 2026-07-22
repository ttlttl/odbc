package api

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestNativeCallLockSerializesCallers(t *testing.T) {
	EnableNativeCallSerialization()
	const callers = 32
	start := make(chan struct{})
	release := make(chan struct{})
	var entered int32
	var active int32
	var maxActive int32
	var wg sync.WaitGroup

	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			unlock := lockNativeCall()
			current := atomic.AddInt32(&active, 1)
			for {
				maximum := atomic.LoadInt32(&maxActive)
				if current <= maximum || atomic.CompareAndSwapInt32(&maxActive, maximum, current) {
					break
				}
			}
			atomic.AddInt32(&entered, 1)
			<-release
			atomic.AddInt32(&active, -1)
			unlock()
		}()
	}

	close(start)
	for atomic.LoadInt32(&entered) == 0 {
	}
	close(release)
	wg.Wait()

	if maxActive != 1 {
		t.Fatalf("maximum concurrent native calls = %d, want 1", maxActive)
	}
}
