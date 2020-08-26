package locker

import (
	"context"
	"math/rand"
	"os"
	"testing"
	"time"
)

var testpostgresURI = "postgres://acyl:acyl@localhost:5432/acyl?sslmode=disable"

func TestFakePreemptableLock(t *testing.T) {
	runTests(t, func(t *testing.T) LockProvider {
		return NewFakeLockProvider()
	})
}

func TestPostgresPreemptableLock(t *testing.T) {
	if os.Getenv("POSTGRES_ALREADY_RUNNING") == "" {
		t.Skip()
	}
	runTests(t, func(t *testing.T) LockProvider {
		pl, err := NewPostgresLockProvider(testpostgresURI, "postgres_locker", false)
		if err != nil {
			t.Fatalf("unable to create postgres lock provider: %v", err)
		}
		return pl
	})
}

// pLFactoryFunc is a function that returns an empty Preemptable Locker
type pLFactoryFunc func(t *testing.T) LockProvider

// runTests runs all tests against the LockProvider implementation returned by plfunc
func runTests(t *testing.T, plfunc pLFactoryFunc) {
	if plfunc == nil {
		t.Fatalf("plfunc cannot be nil")
	}
	tests := []struct {
		name  string
		tfunc func(*testing.T, LockProvider)
	}{
		{
			name:  "lock and unlock",
			tfunc: testLockAndUnlock,
		},
		{
			name:  "preememption",
			tfunc: testPreemption,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.tfunc == nil {
				t.Skip("test func is nil")
			}
			tt.tfunc(t, plfunc(t))
		})
	}
}

func testLockAndUnlock(t *testing.T, lp LockProvider) {
	key := rand.Int63()
	lock, err := lp.NewLock(context.Background(), key, "some-event")
	if err != nil {
		t.Fatalf("unable to acquire lock: %v", err)
	}
	preempt, err := lock.Lock(context.Background(), 5*time.Second)

	if err != nil {
		t.Fatalf("unable to lock: %v", err)
	}
	defer func() {
		err := lock.Unlock(context.Background())
		if err != nil {
			t.Fatalf("error unlocking: %v", err)
		}
	}()

	newLock, err := lp.NewLock(context.Background(), key, "new-event")
	if err != nil {
		t.Fatalf("unable to lock: %v", err)
	}

	// lock is already acquired. new locks should not be able to lock on the same key
	_, err = newLock.Lock(context.Background(), time.Second)
	if err == nil {
		t.Fatalf("expected error trying to lock")
	}

	// original lock should not be preempted
	select {
	case <-preempt:
		t.Fatalf("lock should not have been preempted")
	case <-time.After(1 * time.Second):
	}
}

func testPreemption(t *testing.T, lp LockProvider) {
	key := rand.Int63()
	lock, err := lp.NewLock(context.Background(), key, "some-event")
	if err != nil {
		t.Fatalf("unable to acquire lock: %v", err)
	}

	lock2, err := lp.NewLock(context.Background(), key, "new-event")
	if err != nil {
		t.Fatalf("unable to acquire lock: %v", err)
	}

	preempt, err := lock.Lock(context.Background(), time.Second)
	if err != nil {
		t.Fatalf("unable to lock: %v", err)
	}

	// Notifying a different lock with the same key should let the user know the lock has been preempted
	err = lock2.Notify(context.Background())
	if err != nil {
		t.Fatalf("unable to lock: %v", err)
	}

	select {
	case <-preempt:
		err := lock.Unlock(context.Background())
		if err != nil {
			t.Fatalf("error unlocking: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("lock was never preempted after being notified")
	}

	// Now, the second lock should be able to lock
	_, err = lock2.Lock(context.Background(), time.Second)
	if err != nil {
		t.Fatalf("unable to lock: %v", err)
	}
	err = lock2.Unlock(context.Background())
	if err != nil {
		t.Fatalf("error unlocking: %v", err)
	}
}

func testContextCancellation(t *testing.T, lp LockProvider) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := lp.NewLock(ctx, rand.Int63(), "some-event")
	if err == nil {
		t.Fatalf("expected canceled context to prevent the ability to acquire the lock")
	}

	ctx, cancel = context.WithCancel(context.Background())
	lock, err := lp.NewLock(ctx, rand.Int63(), "some-event")
	if err != nil {
		t.Fatalf("unable to acquire the lock")
	}

	cancel()
	_, err = lock.Lock(ctx, time.Second)
	if err == nil {
		t.Fatalf("expected canceled context to prevent the ability to lock the lock")
	}
}
