package locker

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

func TestPostgresLockHandlesSessionError(t *testing.T) {
	if os.Getenv("POSTGRES_ALREADY_RUNNING") == "" {
		t.Skip()
	}

	db, err := sqlx.Open("postgres", testpostgresURI)
	if err != nil {
		t.Fatalf("unable to open postgres connection: %v", err)
	}
	lock, err := newPostgresLock(context.Background(), db, 0, 0, testpostgresURI, "some-event")
	if err != nil {
		t.Fatalf("unable to create postgres lock: %v", err)
	}

	preempt, err := lock.Lock(context.Background(), time.Second)
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

	db, err := sqlx.Open("postgres", testpostgresURI)
	if err != nil {
		t.Fatalf("unable to open postgres connection: %v", err)
	}
	err = db.Close()
	if err != nil {
		t.Fatalf("unable to close db")
	}
	_, err = newPostgresLock(context.Background(), db, 1, 1, testpostgresURI, "some-event")
	if err == nil {
		t.Fatalf("expected error when creating postgres lock with close sql.DB")
	}
}

func TestPostgresContextCancelationDetection(t *testing.T) {
	if os.Getenv("POSTGRES_ALREADY_RUNNING") == "" {
		t.Skip()
	}

	db, err := sqlx.Open("postgres", testpostgresURI)
	if err != nil {
		t.Fatalf("unable to open postgres connection: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	lock, err := newPostgresLock(ctx, db, 0, 0, testpostgresURI, "some-event")
	if err != nil {
		t.Fatalf("unable to create postgres lock: %v", err)
	}

	preempt, err := lock.Lock(context.Background(), time.Second)
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
