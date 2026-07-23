package api

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
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

func TestSQLCancelUsesNativeCallLock(t *testing.T) {
	EnableNativeCallSerialization()

	unlock := lockNativeCall()
	started := make(chan struct{})
	entered := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		close(started)
		serializedSQLCancel(SQLHSTMT(SQL_NULL_HSTMT), func(SQLHSTMT) SQLRETURN {
			close(entered)
			return SQL_SUCCESS
		})
	}()
	<-started

	select {
	case <-entered:
		unlock()
		t.Fatal("SQLCancel entered the native driver while another call held the serialization lock")
	case <-time.After(50 * time.Millisecond):
	}

	unlock()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("SQLCancel did not proceed after the serialization lock was released")
	}
}
