package locker

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

var testCheckInterval = 10 * time.Millisecond

// NewFakeTestLocker takes a kv, cs, and m so that multiple test lockers can share the same datastore, to simulate multiple processes contending for a lock
// FakeRealLockFactory is returned so that tests can manipulate private fields on the struct
func NewFakeTestLocker(repo, pr string, opts PreemptiveLockerOpts, kv *FakeConsulKV, cs *FakeConsulSession, m map[string]*FakeRealPreemptableLock) (*PreemptiveLocker, *FakeRealLockFactory) {
	key := fmt.Sprintf("fake-%v/%v/lock", repo, pr)
	frlf := &FakeRealLockFactory{
		CS:                   cs,
		SessionCheckInterval: testCheckInterval,
		LockWait:             opts.LockWait,
		m:                    m,
	}
	return &PreemptiveLocker{
		c:          frlf,
		kv:         kv,
		cs:         cs,
		w:          newKeyWatcher(kv, opts.LockWait),
		lockKey:    key,
		pendingKey: fmt.Sprintf("%s/pending", key),
		opts:       opts,
	}, frlf
}

// TestPreemptiveLockerLockAndUnlock tests basic lock and unlock from different processes
func TestPreemptiveLockerLockAndUnlock(t *testing.T) {
	kv := NewFakeConsulKV()
	cs := &FakeConsulSession{}
	m := map[string]*FakeRealPreemptableLock{}

	// long session TTL, short lock wait and lock delay
	opts := PreemptiveLockerOpts{
		SessionTTL: 10 * time.Minute,
		LockWait:   10 * time.Millisecond,
		LockDelay:  10 * time.Millisecond,
	}

	// First process
	ftl, lf := NewFakeTestLocker("foo/bar", "1", opts, kv, cs, m)
	lf.SessionCheckInterval = 100 * time.Second // set long intervals so those processes don't run
	ftl.w.watchInterval = 100 * time.Second

	ctx, cf := context.WithCancel(context.Background())
	defer cf()

	// First process takes lock
	preempt, err := ftl.Lock(ctx)
	if err != nil {
		t.Fatalf("lock should have succeeded: %v", err)
	}

	// lock should not be preempted
	select {
	case <-preempt:
		t.Fatalf("should not have been preempted")
	default:
		break
	}

	// First process cannot re-acquire the same lock
	_, err = ftl.Lock(ctx)
	if err == nil {
		t.Fatalf("should have failed to reacquire same lock")
	}

	// Second process
	ftl2, lf2 := NewFakeTestLocker("foo/bar", "1", opts, kv, cs, m)
	lf2.SessionCheckInterval = 100 * time.Second
	ftl2.w.watchInterval = 100 * time.Second

	// Fails to acquire lock (wait exceeds LockWait)
	_, err = ftl2.Lock(ctx)
	if err == nil {
		t.Fatalf("should have failed to acquire second lock on same key")
	}

	// Process one releases lock
	if err := ftl.Release(context.Background()); err != nil {
		t.Fatalf("release should have succeeded: %v", err)
	}

	// Third process
	ftl3, lf3 := NewFakeTestLocker("foo/bar", "1", opts, kv, cs, m)
	lf3.SessionCheckInterval = 100 * time.Second
	ftl3.w.watchInterval = 100 * time.Second

	// Successfully takes lock
	_, err = ftl3.Lock(ctx)
	if err != nil {
		t.Fatalf("third lock on same key should have succeeded after first one was released: %v", err)
	}
}

// TestPreemptiveLockerCancelledContext tests an attempted lock with a cancelled context
func TestPreemptiveLockerCancelledContext(t *testing.T) {
	kv := NewFakeConsulKV()
	cs := &FakeConsulSession{}
	m := map[string]*FakeRealPreemptableLock{}

	// long lock wait
	opts := PreemptiveLockerOpts{
		SessionTTL: 10 * time.Minute,
		LockWait:   100 * time.Second,
		LockDelay:  10 * time.Millisecond,
	}

	// first process
	ftl, lf := NewFakeTestLocker("foo/bar", "1", opts, kv, cs, m)
	lf.SessionCheckInterval = 100 * time.Second
	ftl.w.watchInterval = 100 * time.Second

	ctx, cf := context.WithCancel(context.Background())
	defer cf()

	// take initial lock
	_, err := ftl.Lock(ctx)
	if err != nil {
		t.Fatalf("initial lock should have succeeded: %v", err)
	}

	// second process
	ftl2, lf2 := NewFakeTestLocker("foo/bar", "1", opts, kv, cs, m)
	lf2.SessionCheckInterval = 100 * time.Second
	ftl2.w.watchInterval = 100 * time.Second

	ctx2, cf2 := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cf2() // cancel the context
	}()

	// should fail to acquire the lock because context is cancelled
	_, err = ftl2.Lock(ctx2)
	if err == nil {
		t.Fatalf("should have returned a context cancelled error")
	}
	if !strings.Contains(err.Error(), "context was cancelled") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestPreemptiveLockerExpiredSession tests that when a session expires the lock is released and the previous lockholder gets notified to release the lock
func TestPreemptiveLockerExpiredSession(t *testing.T) {
	kv := NewFakeConsulKV()
	cs := &FakeConsulSession{}
	m := map[string]*FakeRealPreemptableLock{}

	// long lock wait
	opts := PreemptiveLockerOpts{
		SessionTTL: 10 * time.Minute,
		LockWait:   100 * time.Second,
		LockDelay:  10 * time.Millisecond,
	}

	// first process
	ftl, lf := NewFakeTestLocker("foo/bar", "1", opts, kv, cs, m)
	lf.SessionCheckInterval = 10 * time.Millisecond // low session check interval
	ftl.w.watchInterval = 100 * time.Second

	ctx, cf := context.WithCancel(context.Background())
	defer cf()

	// get initial lock
	preempt, err := ftl.Lock(ctx)
	if err != nil {
		t.Fatalf("initial lock should have succeeded: %v", err)
	}

	// manually expire the session
	if err := lf.CS.SetExpires(ftl.sessionid, time.Now().UTC().Add(-1*time.Millisecond)); err != nil {
		t.Fatalf("error expiring session: %v", err)
	}

	time.Sleep(lf.SessionCheckInterval * 10) // sleep for SessionCheckInterval + lock delay

	// make sure original lockholder was preempted
	select {
	case <-preempt:
		break
	default:
		t.Fatalf("should have been preempted")
	}

	// second process should now successfully lock
	ftl2, lf2 := NewFakeTestLocker("foo/bar", "1", opts, kv, cs, m)
	lf2.SessionCheckInterval = 100 * time.Second
	ftl2.w.watchInterval = 100 * time.Second

	_, err = ftl2.Lock(ctx)
	if err != nil {
		t.Fatalf("second lock on same key should have succeeded after first one was invalidated: %v", err)
	}
}

// TestPreemptiveLockerPreemption tests that a new lock contender causes the existing lockholder to be preempted
func TestPreemptiveLockerPreemption(t *testing.T) {
	kv := NewFakeConsulKV()
	cs := &FakeConsulSession{}
	m := map[string]*FakeRealPreemptableLock{}

	// long lock wait
	opts := PreemptiveLockerOpts{
		SessionTTL: 10 * time.Minute,
		LockWait:   100 * time.Second,
		LockDelay:  10 * time.Millisecond,
	}

	// first process
	ftl, lf := NewFakeTestLocker("foo/bar", "1", opts, kv, cs, m)
	lf.SessionCheckInterval = 100 * time.Second
	ftl.w.watchInterval = 10 * time.Millisecond // low key watch interval

	ctx, cf := context.WithCancel(context.Background())
	defer cf()

	// First process takes lock
	preempt, err := ftl.Lock(ctx)
	if err != nil {
		t.Fatalf("lock should have succeeded: %v", err)
	}

	// lock should not be preempted yet
	select {
	case <-preempt:
		t.Fatalf("should not have been preempted")
	default:
		break
	}

	// simulate abort and lock release when preempted
	go func() {
		<-preempt
		t.Logf("process 1: releasing lock")
		ftl.Release(context.Background())
	}()

	// second process
	ftl2, lf2 := NewFakeTestLocker("foo/bar", "1", opts, kv, cs, m)
	lf2.SessionCheckInterval = 100 * time.Second
	ftl2.w.watchInterval = 100 * time.Second

	// Second process contends lock, has to wait for process one to abort and release
	preempt2, err := ftl2.Lock(ctx)
	if err != nil {
		t.Fatalf("second lock should have eventually succeeded: %v", err)
	}

	// second process lock should not be preempted
	select {
	case <-preempt2:
		t.Fatalf("process two should not have been preempted")
	default:
		break
	}
}

// TestPreemptiveLockerMultiPreemption tests that a waiting lock contender gets preempted when a more recent contender appears
func TestPreemptiveLockerMultiPreemption(t *testing.T) {
	kv := NewFakeConsulKV()
	cs := &FakeConsulSession{}
	m := map[string]*FakeRealPreemptableLock{}

	// long lock wait
	opts := PreemptiveLockerOpts{
		SessionTTL: 10 * time.Minute,
		LockWait:   100 * time.Second,
		LockDelay:  10 * time.Millisecond,
	}

	// first process
	ftl, lf := NewFakeTestLocker("foo/bar", "1", opts, kv, cs, m)
	lf.SessionCheckInterval = 100 * time.Second
	ftl.w.watchInterval = 10 * time.Millisecond // low key watch interval

	ctx, cf := context.WithCancel(context.Background())
	defer cf()

	// First process takes lock
	preempt, err := ftl.Lock(ctx)
	if err != nil {
		t.Fatalf("lock should have succeeded: %v", err)
	}

	// lock should not be preempted yet
	select {
	case <-preempt:
		t.Fatalf("should not have been preempted")
	default:
		break
	}

	// channels are used by the simulated processes as signals that they've completed their part of the test
	process2 := make(chan struct{})
	process3 := make(chan struct{})

	// first process ignores the preemption by process 2, waits for process 2 to abort and for process 3 to contend the lock
	go func() {
		<-preempt
		<-process2 // wait for process 2 to exit
		<-process3 // wait for process 3 to contend
		ftl.Release(context.Background())
	}()

	// second process
	ftl2, lf2 := NewFakeTestLocker("foo/bar", "1", opts, kv, cs, m)
	lf2.SessionCheckInterval = 100 * time.Second
	ftl2.w.watchInterval = 10 * time.Millisecond // low key watch interval

	// Second process contends lock and will wait until preempted by process 3
	go func() {
		defer close(process2)
		ctx2, cf2 := context.WithCancel(context.Background())
		// we have to make sure the context gets cancelled, otherwise we leave a dangling goroutine
		// contending the FakeRealPreemptableLock lock channel. In real life, the process context would
		// always get cancelled.
		defer cf2()
		_, err := ftl2.Lock(ctx2)
		if err == nil {
			t.Fatalf("second lock should have returned a preempted error")
		}
		if !strings.Contains(err.Error(), "preempted") {
			t.Fatalf("unexpected error: %v", err)
		}
	}()

	// allow time for the second process to contend
	time.Sleep(10 * time.Millisecond)

	// third process
	opts.LockWait = 20 * time.Millisecond
	ftl3, lf3 := NewFakeTestLocker("foo/bar", "1", opts, kv, cs, m)
	lf3.SessionCheckInterval = 100 * time.Second
	ftl3.w.watchInterval = 100 * time.Second

	go func() {
		time.Sleep(10 * time.Millisecond) // allow time for process 3 to contend
		close(process3)
	}()

	// third process takes lock
	preempt3, err := ftl3.Lock(ctx)
	if err != nil {
		t.Fatalf("process 3 lock should have succeeded: %v", err)
	}

	// lock should not be preempted yet
	select {
	case <-preempt3:
		t.Fatalf("process 3 should not have been preempted")
	default:
		break
	}
}
