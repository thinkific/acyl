package locker

import (
	"context"
	"database/sql"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/DavidHuie/gomigrate"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

var testPostgresURI = "postgres://acyl:acyl@localhost:5432/acyl?sslmode=disable"

type testTableCoordinator struct {
	db *sql.DB
}

func (ttc *testTableCoordinator) destroy() error {
	logger := log.New(ioutil.Discard, "", log.LstdFlags)
	migrator, err := gomigrate.NewMigratorWithLogger(ttc.db, gomigrate.Postgres{}, "./migrations", logger)
	defer ttc.db.Close()
	if err != nil {
		return errors.Wrap(err, "error creating migrator")
	}
	if err := migrator.RollbackAll(); err != nil {
		return errors.Wrap(err, "error rolling back migrations")
	}
	return nil
}

func (ttc *testTableCoordinator) setup() error {
	db, err := sql.Open("postgres", testPostgresURI)
	if err != nil {
		return errors.Wrap(err, "error creating postgres connection")
	}
	ttc.db = db
	logger := log.New(ioutil.Discard, "", log.LstdFlags)
	migrator, err := gomigrate.NewMigratorWithLogger(ttc.db, gomigrate.Postgres{}, "./migrations", logger)
	if err != nil {
		return errors.Wrap(err, "error creating migrator")
	}
	if err := migrator.Migrate(); err != nil {
		return errors.Wrap(err, "error applying migration")
	}
	return nil
}

func TestFakePreemptableLock(t *testing.T) {
	runPreemptableLockTests(t, func(t *testing.T, options ...LockProviderOption) (LockProvider, error) {
		return NewLockProvider(FakeLockProviderKind, options...)
	})
}

func TestPostgresPreemptableLock(t *testing.T) {
	if os.Getenv("POSTGRES_ALREADY_RUNNING") == "" {
		t.Skip()
	}
	ttc := &testTableCoordinator{}
	err := ttc.setup()
	if err != nil {
		t.Fatalf("unable to create tables: %v", err)
	}
	defer func() {
		if err := ttc.destroy(); err != nil {
			t.Logf("error destroying tables: %v", err)
		}
	}()
	runPreemptableLockTests(t, func(t *testing.T, options ...LockProviderOption) (LockProvider, error) {
		opts := []LockProviderOption{WithPostgresBackend(testPostgresURI, "")}
		opts = append(opts, options...)
		return NewLockProvider(PostgresLockProviderKind, opts...)
	})
}

// lockProviderFactoryFunc is a test helper function for creating LockProviders
type lpFactoryFunc func(t *testing.T, options ...LockProviderOption) (LockProvider, error)

// runPreemptableLockTests runs all tests against the LockProvider implementation returned by plfunc
func runPreemptableLockTests(t *testing.T, lpFunc lpFactoryFunc) {
	if lpFunc == nil {
		t.Fatalf("plfunc cannot be nil")
	}
	tests := []struct {
		name    string
		tfunc   func(*testing.T, LockProvider)
		options []LockProviderOption
	}{
		{
			name:    "lock and unlock",
			tfunc:   testLockAndUnlock,
			options: []LockProviderOption{WithLockTimeout(defaultPostgresLockWaitTime)},
		},
		{
			name:    "preememption",
			tfunc:   testPreemption,
			options: []LockProviderOption{WithLockTimeout(defaultPostgresLockWaitTime)},
		},
		{
			name:    "obtains correct lock key with concurrent goroutines",
			tfunc:   testLockKeyConcurrent,
			options: []LockProviderOption{WithLockTimeout(defaultPostgresLockWaitTime)},
		},
		{
			name:    "max lock duration is respected",
			tfunc:   testMaxLockDuration,
			options: []LockProviderOption{WithMaxLockDuration(1 * time.Second), WithLockTimeout(defaultPostgresLockWaitTime), WithPreemptionTimeout(100 * time.Millisecond)},
		},
		{
			name:    "preemption timeout is respected",
			tfunc:   testPreemptionTimeout,
			options: []LockProviderOption{WithPreemptionTimeout(100 * time.Millisecond), WithLockTimeout(defaultPostgresLockWaitTime)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.tfunc == nil {
				t.Skip("test func is nil")
			}
			pl, err := lpFunc(t, tt.options...)
			if err != nil {
				t.Fatalf("error creating lock provdier")
			}
			tt.tfunc(t, pl)
		})
	}
}

func testLockAndUnlock(t *testing.T, lp LockProvider) {
	key := rand.Int63()
	lock, err := lp.New(context.Background(), key, "some-event")
	if err != nil {
		t.Fatalf("unable to acquire lock: %v", err)
	}
	preempt, err := lock.Lock(context.Background())

	if err != nil {
		t.Fatalf("unable to lock: %v", err)
	}
	defer func() {
		lock.Unlock(context.Background())
	}()

	newLock, err := lp.New(context.Background(), key, "new-event")
	if err != nil {
		t.Fatalf("unable to lock: %v", err)
	}

	// lock is already acquired. new locks should not be able to lock on the same key
	_, err = newLock.Lock(context.Background())
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
	lock, err := lp.New(context.Background(), key, "some-event")
	if err != nil {
		t.Fatalf("unable to acquire lock: %v", err)
	}

	lock2, err := lp.New(context.Background(), key, "new-event")
	if err != nil {
		t.Fatalf("unable to acquire lock: %v", err)
	}

	preempt, err := lock.Lock(context.Background())
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
		lock.Unlock(context.Background())
	case <-time.After(time.Second):
		t.Fatalf("lock was never preempted after being notified")
	}

	// Now, the second lock should be able to lock
	_, err = lock2.Lock(context.Background())
	if err != nil {
		t.Fatalf("unable to lock: %v", err)
	}
	lock2.Unlock(context.Background())
}

func testContextCancellation(t *testing.T, lp LockProvider) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := lp.New(ctx, rand.Int63(), "some-event")
	if err == nil {
		t.Fatalf("expected canceled context to prevent the ability to acquire the lock")
	}

	ctx, cancel = context.WithCancel(context.Background())
	lock, err := lp.New(ctx, rand.Int63(), "some-event")
	if err != nil {
		t.Fatalf("unable to acquire the lock")
	}

	cancel()
	_, err = lock.Lock(ctx)
	if err == nil {
		t.Fatalf("expected canceled context to prevent the ability to lock the lock")
	}
}

func testLockKeyConcurrent(t *testing.T, lp LockProvider) {
	testRepo := "foo/bar"
	pr := uint(rand.Intn(math.MaxInt32))
	ch := make(chan int64, 5)
	errGroupCtx, errGroupCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer errGroupCancel()
	g, ctx := errgroup.WithContext(errGroupCtx)

	// All goroutines should receive the same key
	for i := 0; i < 5; i++ {
		g.Go(func() error {
			key, err := lp.LockKey(ctx, testRepo, pr)
			ch <- key
			return err
		})
	}

	if err := g.Wait(); err != nil {
		t.Fatalf("error from lock key: %v", err)
	}
	close(ch)

	keys := []int64{}
	for key := range ch {
		keys = append(keys, key)
	}

	first := keys[0]
	if first == 0 {
		t.Fatalf("lock key is the default value for int64, which likely indicates the value is not being set properly")
	}
	for _, key := range keys {
		if first != key {
			t.Fatalf("expected all keys to be the same. 2 differ: %d, %d", first, key)
		}
	}
}

func testMaxLockDuration(t *testing.T, lp LockProvider) {
	key := rand.Int63()
	lock, err := lp.New(context.Background(), key, "some-event")
	if err != nil {
		t.Fatalf("unable to acquire lock: %v", err)
	}
	_, err = lock.Lock(context.Background())
	if err != nil {
		t.Fatalf("error locking lock: %v", err)
	}

	// Lock holder never unlocked explicitly, but the lock should have been unlocked automatically after the maxLockDuration has passed
	time.Sleep(2 * time.Second)
	lock, err = lp.New(context.Background(), key, "some-event")
	if err != nil {
		t.Fatalf("unable to acquire lock: %v", err)
	}

	_, err = lock.Lock(context.Background())
	if err != nil {
		t.Fatalf("error locking lock: %v", err)
	}
}

func testPreemptionTimeout(t *testing.T, lp LockProvider) {
	key := rand.Int63()
	lock, err := lp.New(context.Background(), key, "some-event")
	if err != nil {
		t.Fatalf("unable to acquire lock: %v", err)
	}
	_, err = lock.Lock(context.Background())
	if err != nil {
		t.Fatalf("error locking lock: %v", err)
	}

	lock2, err := lp.New(context.Background(), key, "some-event")
	if err != nil {
		t.Fatalf("unable to acquire lock: %v", err)
	}

	// After waiting for a preemptionTimeout, the lock should automatically be unlocked.
	// So we should be able to lock.
	lock2.Notify(context.Background())
	time.Sleep(5 * time.Second)

	_, err = lock2.Lock(context.Background())
	if err != nil {
		t.Fatalf("error locking lock: %v", err)
	}
}
