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
	locks map[[2]int32]*fakeLockStoreEntry
	mutex sync.Mutex
}

func (fls *fakeLockStore) store(key1, key2 int32, lock *FakePreemptableLock) {
	fls.mutex.Lock()
	defer fls.mutex.Unlock()
	key := [2]int32{key1, key2}
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

func (fls *fakeLockStore) delete(key1, key2 int32, id uuid.UUID) {
	// assumes the mutex is already held
	key := [2]int32{key1, key2}
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

func (fls *fakeLockStore) lock(ctx context.Context, key1, key2 int32) error {
	fls.mutex.Lock()
	defer fls.mutex.Unlock()
	key := [2]int32{key1, key2}
	entry := fls.locks[key]
	select {
	case <-ctx.Done():
		return ctx.Err()
	case entry.lock <- struct{}{}:
		return nil
	}
}

func (fls *fakeLockStore) unlock(ctx context.Context, key1, key2 int32, id uuid.UUID) error {
	fls.mutex.Lock()
	defer fls.mutex.Unlock()
	key := [2]int32{key1, key2}
	entry := fls.locks[key]
	go func() { <-entry.lock }()

	// remove the key from the slice of lock contenders
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
	return nil
}

func (fls *fakeLockStore) notify(ctx context.Context, key1, key2 int32, id uuid.UUID) error {
	fls.mutex.Lock()
	defer fls.mutex.Unlock()
	key := [2]int32{key1, key2}
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
			locks: make(map[[2]int32]*fakeLockStoreEntry),
		},
	}
}

func (flp *FakeLockProvider) AcquireLock(ctx context.Context, key1, key2 int32, event string) (PreemptableLock, error) {
	id, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}
	l := &FakePreemptableLock{
		id:      id,
		preempt: make(chan NotificationPayload, 1000),
		store:   flp.store,
		event:   event,
		key1:    key1,
		key2:    key2,
	}
	flp.store.store(key1, key2, l)
	return l, nil
}

var _ PreemptableLock = &FakePreemptableLock{}

type FakePreemptableLock struct {
	store   *fakeLockStore
	id      uuid.UUID
	event   string
	key1    int32
	key2    int32
	preempt chan NotificationPayload
}

func (fpl *FakePreemptableLock) Lock(ctx context.Context, lockWait time.Duration) (<-chan NotificationPayload, error) {
	lockCtx, cancel := context.WithTimeout(ctx, lockWait)
	defer cancel()
	err := fpl.store.lock(lockCtx, fpl.key1, fpl.key2)
	if err != nil {
		return nil, fmt.Errorf("unable to lock fake preemptable lock: %v", err)
	}
	return fpl.preempt, nil
}

func (fpl *FakePreemptableLock) Unlock(ctx context.Context) error {
	return fpl.store.unlock(ctx, fpl.key1, fpl.key2, fpl.id)
}

func (fpl *FakePreemptableLock) Notify(ctx context.Context) error {
	return fpl.store.notify(ctx, fpl.key1, fpl.key2, fpl.id)
}
