package locker

import (
	"context"
	"math/rand"
	"testing"
	"time"
)

func TestPreemptiveLockerReleaseBeforeLock(t *testing.T) {
	testKey := rand.Int63()
	plf, err := NewPreemptiveLockerFactory(NewFakeLockProvider())
	if err != nil {
		t.Fatalf("error creating new preemptive locker factory: %v", err)
	}
	pl := plf(testKey, "update")

	// Attempting to Release before calling Lock should fail
	err = pl.Release(context.Background())
	if err == nil {
		t.Fatalf("expected error when Releasing before Locking")
	}
}

func TestPreemptLockerLocksOnlyOnce(t *testing.T) {
	testKey := rand.Int63()
	plf, err := NewPreemptiveLockerFactory(
		NewFakeLockProvider(),
		WithLockDelay(time.Second),
		WithLockWait(100*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("error creating new preemptive locker factory: %v", err)
	}
	pl := plf(testKey, "update")
	_, err = pl.Lock(context.Background())
	if err != nil {
		t.Fatalf("unexpected error when locking: %v", err)
	}

	// Attempting to lock again from the same preemptive locker should fail
	_, err = pl.Lock(context.Background())
	if err == nil {
		t.Fatalf("expected error when attempting to lock already held lock")
	}
}

func TestPreemptiveLockerLockAndRelease(t *testing.T) {
	testKey := rand.Int63()
	plf, err := NewPreemptiveLockerFactory(
		NewFakeLockProvider(),
		WithLockDelay(time.Second),
		WithLockWait(time.Second),
	)
	if err != nil {
		t.Fatalf("error creating new preemptive locker factory: %v", err)
	}
	// Should be able to lock
	pl := plf(testKey, "update")
	preempt, err := pl.Lock(context.Background())
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

	pl2 := plf(testKey, "update")
	// New PreemptiveLocker should fail to lock (due to LockWait timeout)
	_, err = pl2.Lock(context.Background())
	if err == nil {
		t.Fatalf("should have failed to acquire second lock on same key")
	}

	select {
	case <-preempt:
		// first lock was preempted, release the lock
		err := pl.Release(context.Background())
		if err != nil {
			t.Fatalf("error releasing the lock: %v", err)
		}
	default:
		t.Fatalf("original lock was not preempted")

	}

	// New PreemptiveLocker should be able to acquire lock now
	pl3 := plf(testKey, "update")
	_, err = pl3.Lock(context.Background())
	if err != nil {
		t.Fatalf("should have been able to lock the key: %v", err)
	}
}
func TestPreemptiveLockerLockDelay(t *testing.T) {
	testKey := rand.Int63()
	lockDelay := time.Second
	plf, err := NewPreemptiveLockerFactory(
		NewFakeLockProvider(),
		WithLockDelay(lockDelay),
		WithLockWait(100*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("error creating new preemptive locker factory: %v", err)
	}
	pl := plf(testKey, "update")
	start := time.Now()
	_, err = pl.Lock(context.Background())
	if err != nil {
		t.Fatalf("unexpected error locking: %v", err)
	}
	d := time.Since(start)
	if d < lockDelay {
		t.Fatalf("lock delay value was not respected: %v", err)
	}
}

func TestNewPreemptiveLockerCancelledContext(t *testing.T) {
	testKey := rand.Int63()
	plf, err := NewPreemptiveLockerFactory(
		NewFakeLockProvider(),
		WithLockDelay(time.Second),
		WithLockWait(100*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("error creating new preemptive locker factory: %v", err)
	}
	pl := plf(testKey, "update")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = pl.Lock(ctx)
	if err == nil {
		t.Fatalf("expected error when locking with a canceled context")
	}
}
