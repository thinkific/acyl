package locker

import (
	"context"
	"testing"
	"time"
)

func TestPreemptiveLockerReleaseBeforeLock(t *testing.T) {
	lp := NewFakeLockProvider()
	testKey := "foo"
	pl := NewPreemptiveLocker(lp, testKey, PreemptiveLockerOpts{
		LockDelay: time.Second,
		LockWait:  100 * time.Millisecond,
	})

	// Attempting to Release before calling Lock should fail
	err := pl.Release(context.Background())
	if err == nil {
		t.Fatalf("expected error when Releasing before Locking")
	}
}

func TestPreemptLockerLocksOnlyOnce(t *testing.T) {
	lp := NewFakeLockProvider()
	testKey := "foo"
	testEvent := "update"
	pl := NewPreemptiveLocker(lp, testKey, PreemptiveLockerOpts{
		LockDelay: time.Second,
		LockWait:  100 * time.Millisecond,
	})

	_, err := pl.Lock(context.Background(), testEvent)
	if err != nil {
		t.Fatalf("unexpected error when locking: %v", err)
	}

	// Attempting to lock again from the same preemptive locker should fail
	_, err = pl.Lock(context.Background(), testEvent)
	if err == nil {
		t.Fatalf("expected error when attempting to lock already held lock")
	}
}

func TestPreemptiveLockerLockAndRelease(t *testing.T) {
	lp := NewFakeLockProvider()
	testKey := "foo"
	testEvent := "update"
	pl := NewPreemptiveLocker(lp, testKey, PreemptiveLockerOpts{
		LockDelay: time.Second,
		LockWait:  100 * time.Millisecond,
	})

	// Should be able to lock
	preempt, err := pl.Lock(context.Background(), testEvent)
	if err != nil {
		t.Fatalf("unexpected error when locking: %v", err)
	}

	// Lock should not be preempted
	select {
	case <-preempt:
		t.Fatalf("should not have been preempted")
	default:
		break
	}

	pl2 := NewPreemptiveLocker(lp, testKey, PreemptiveLockerOpts{
		LockDelay: time.Second,
		LockWait:  time.Second,
	})

	// New PreemptiveLocker should fail to lock (due to LockWait timeout)
	_, err = pl2.Lock(context.Background(), testEvent)
	if err == nil {
		t.Fatalf("should have failed to acquire second lock on same key")
	}

	go func() {
		select {
		case <-preempt:
			// first lock was preempted, release the lock
			err := pl.Release(context.Background())
			if err != nil {
				t.Errorf("error releasing the lock: %v", err)
			}
		}
	}()

	pl3 := NewPreemptiveLocker(lp, testKey, PreemptiveLockerOpts{
		LockDelay: time.Second,
		LockWait:  time.Second,
	})

	// New PreemptiveLocker should be able to acquire lock now
	_, err = pl3.Lock(context.Background(), testEvent)
	if err != nil {
		t.Fatalf("should have been able to lock the key: %v", err)
	}
}
func TestPreemptiveLockerLockDelay(t *testing.T) {
	lp := NewFakeLockProvider()
	testKey := "foo"
	testEvent := "update"
	lockDelay := time.Second
	pl := NewPreemptiveLocker(lp, testKey, PreemptiveLockerOpts{
		LockDelay: lockDelay,
		LockWait:  100 * time.Millisecond,
	})
	start := time.Now()
	_, err := pl.Lock(context.Background(), testEvent)
	if err != nil {
		t.Fatalf("unexpected error locking: %v", err)
	}
	d := time.Since(start)
	if d < lockDelay {
		t.Fatalf("lock delay value was not respected: %v", err)
	}
}

func TestNewPreemptiveLockerCancelledContext(t *testing.T) {
	lp := NewFakeLockProvider()
	testKey := "foo"
	testEvent := "update"
	lockDelay := time.Second
	pl := NewPreemptiveLocker(lp, testKey, PreemptiveLockerOpts{
		LockDelay: lockDelay,
		LockWait:  100 * time.Millisecond,
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := pl.Lock(ctx, testEvent)
	if err == nil {
		t.Fatalf("expected error when locking with a canceled context")
	}
}
