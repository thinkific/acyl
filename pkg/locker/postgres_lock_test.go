package locker

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

func TestPostgresLockHandlesSessionError(t *testing.T) {
	if os.Getenv("POSTGRES_ALREADY_RUNNING") == "" {
		t.Skip()
	}

	db, err := sqlx.Open("postgres", testPostgresURI)
	if err != nil {
		t.Fatalf("unable to open postgres connection: %v", err)
	}
	lock, err := NewPostgresLock(context.Background(), db, rand.Int63(), testPostgresURI, "some-event", LockProviderConfig{})
	if err != nil {
		t.Fatalf("unable to create postgres lock: %v", err)
	}

	preempt, err := lock.Lock(context.Background())
	if err != nil {
		t.Fatalf("error locking postgres lock: %v", err)
	}

	// simulate an error
	lock.psc.sessionErr <- fmt.Errorf("some connection error happened")

	select {
	case <-preempt:
		// got preempted as expected, due to session error
	case <-time.After(time.Second):
		t.Fatalf("expected to get preempted due to session error")
	}
}

func TestPostgresLockProviderFailedInitialConnection(t *testing.T) {
	if os.Getenv("POSTGRES_ALREADY_RUNNING") == "" {
		t.Skip()
	}

	db, err := sqlx.Open("postgres", testPostgresURI)
	if err != nil {
		t.Fatalf("unable to open postgres connection: %v", err)
	}
	err = db.Close()
	if err != nil {
		t.Fatalf("unable to close db")
	}
	_, err = NewPostgresLock(context.Background(), db, rand.Int63(), testPostgresURI, "some-event", LockProviderConfig{})
	if err == nil {
		t.Fatalf("expected error when creating postgres lock with close sql.DB")
	}
}

func TestPostgresContextCancelationDetection(t *testing.T) {
	if os.Getenv("POSTGRES_ALREADY_RUNNING") == "" {
		t.Skip()
	}
	opts := []LockProviderOption{
		WithPostgresBackend(testPostgresURI, ""),
		WithLockTimeout(time.Second),
	}
	lp, err := NewLockProvider(PostgresLockProviderKind, opts...)
	if err != nil {
		t.Fatalf("unable to create lock provider: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	lock, err := lp.New(ctx, rand.Int63(), "some-event")
	if err != nil {
		t.Fatalf("unable to create postgres lock: %v", err)
	}

	preempt, err := lock.Lock(ctx)
	if err != nil {
		t.Fatalf("error locking postgres lock: %v", err)
	}

	cancel()

	select {
	case <-preempt:
		// got preempted as expected, due to context cacnelation
	case <-time.After(time.Second):
		t.Fatalf("expected to get preempted due to session error")
	}
}

func TestPostgresLockNilDB(t *testing.T) {
	if os.Getenv("POSTGRES_ALREADY_RUNNING") == "" {
		t.Skip()
	}

	_, err := NewPostgresLock(context.Background(), nil, rand.Int63(), testPostgresURI, "some-event", LockProviderConfig{})
	if err == nil {
		t.Fatalf("expected error creating a postgres lock with no db")
	}
}
