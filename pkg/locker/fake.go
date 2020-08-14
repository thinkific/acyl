package locker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type fakeLockStoreEntry struct {
	lock       chan struct{}
	contenders []*FakePreemptableLock
}

// fakeLockStore stores all of the local preemptable locks
type fakeLockStore struct {
	locks map[string]*fakeLockStoreEntry
	mutex sync.Mutex
}

func (fls *fakeLockStore) store(key string, lock *FakePreemptableLock) {
	fls.mutex.Lock()
	defer fls.mutex.Unlock()
	_, found := fls.locks[key]
	if !found {
		l := make(chan struct{}, 1)
		fls.locks[key] = &fakeLockStoreEntry{
			contenders: []*FakePreemptableLock{},
			lock:       l,
		}
	}
	fls.locks[key].contenders = append(fls.locks[key].contenders, lock)
}

func (fls *fakeLockStore) delete(key string, id uuid.UUID) {
	// assumes the mutex is already held
	var index int
	for i, lock := range fls.locks[key].contenders {
		if lock.id == id {
			index = i
			break
		}
	}

	length := len(fls.locks[key].contenders)
	fls.locks[key].contenders[index] = fls.locks[key].contenders[length-1]
	fls.locks[key].contenders[length-1] = nil
	fls.locks[key].contenders = fls.locks[key].contenders[:length-1]
}

func (fls *fakeLockStore) lock(ctx context.Context, key string) error {
	fls.mutex.Lock()
	defer fls.mutex.Unlock()
	entry := fls.locks[key]
	select {
	case <-ctx.Done():
		return ctx.Err()
	case entry.lock <- struct{}{}:
		return nil
	}
}

func (fls *fakeLockStore) unlock(ctx context.Context, key string, id uuid.UUID) error {
	fls.mutex.Lock()
	defer fls.mutex.Unlock()
	fls.delete(key, id)
	entry := fls.locks[key]
	go func() { <-entry.lock }()
	return nil
}

func (fls *fakeLockStore) notify(ctx context.Context, key string, id uuid.UUID) error {
	fls.mutex.Lock()
	defer fls.mutex.Unlock()
	if _, ok := fls.locks[key]; !ok {
		return nil
	}
	for _, lock := range fls.locks[key].contenders {
		if lock.id == id {
			continue
		}
		go func(l *FakePreemptableLock) {
			l.preempt <- NotificationPayload{
				ID:        l.id,
				Message:   l.event,
				Preempted: true,
			}
		}(lock)

	}
	return nil
}

// FakeLockProvider can serve as a lock provider with no external dependencies
type FakeLockProvider struct {
	store *fakeLockStore
}

func NewFakeLockProvider() *FakeLockProvider {
	return &FakeLockProvider{
		store: &fakeLockStore{
			locks: make(map[string]*fakeLockStoreEntry),
		},
	}
}

func (flp *FakeLockProvider) AcquireLock(ctx context.Context, key, event string) (PreemptableLock, error) {
	id, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}
	l := &FakePreemptableLock{
		id:      id,
		preempt: make(chan NotificationPayload, 1000),
		store:   flp.store,
		event:   event,
		key:     key,
	}
	flp.store.store(key, l)
	return l, nil
}

type FakePreemptableLock struct {
	store   *fakeLockStore
	id      uuid.UUID
	event   string
	key     string
	preempt chan NotificationPayload
}

func (fpl *FakePreemptableLock) Lock(ctx context.Context, lockWait time.Duration) (<-chan NotificationPayload, error) {
	lockCtx, cancel := context.WithTimeout(ctx, lockWait)
	defer cancel()
	err := fpl.store.lock(lockCtx, fpl.key)
	if err != nil {
		return nil, fmt.Errorf("unable to lock fake preemptable lock: %v", err)
	}
	return fpl.preempt, nil
}

func (fpl *FakePreemptableLock) Unlock(ctx context.Context) error {
	return fpl.store.unlock(ctx, fpl.key, fpl.id)
}

func (fpl *FakePreemptableLock) Notify(ctx context.Context) error {
	return fpl.store.notify(ctx, fpl.key, fpl.id)
}
