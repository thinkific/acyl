package locker

import (
	"context"
	"fmt"
	"math/rand"
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
	envToLockKey map[string]int64
	locks        map[int64]*fakeLockStoreEntry
	mutex        sync.Mutex
}

func (fls *fakeLockStore) store(key int64, lock *FakePreemptableLock) {
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

func (fls *fakeLockStore) lock(ctx context.Context, key int64) error {
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

func (fls *fakeLockStore) unlock(ctx context.Context, key int64, id uuid.UUID) error {
	fls.mutex.Lock()
	defer fls.mutex.Unlock()
	entry := fls.locks[key]
	length := len(fls.locks[key].contenders)
	if length == 0 {
		return nil
	}

	// remove the key from the slice of lock contenders
	var index int
	for i, lock := range fls.locks[key].contenders {
		if lock.id == id {
			index = i
			break
		}
	}

	fls.locks[key].contenders[index] = fls.locks[key].contenders[length-1]
	fls.locks[key].contenders[length-1] = nil
	fls.locks[key].contenders = fls.locks[key].contenders[:length-1]
	go func() { <-entry.lock }()
	return nil
}

func (fls *fakeLockStore) notify(ctx context.Context, key int64, id uuid.UUID) error {
	fls.mutex.Lock()
	defer fls.mutex.Unlock()
	if _, ok := fls.locks[key]; !ok {
		return nil
	}
	for _, lock := range fls.locks[key].contenders {
		if lock.id == id {
			continue
		}
		np := NotificationPayload{
			LockKey: key,
			ID:      lock.id,
			Message: lock.event,
		}
		go lock.handleNotification(np)
	}
	return nil
}

var _ LockProvider = &FakeLockProvider{}

// FakeLockProvider can serve as a lock provider with no external dependencies
type FakeLockProvider struct {
	store *fakeLockStore
	conf  LockProviderConfig
}

func newFakeLockProvider(conf LockProviderConfig) *FakeLockProvider {
	return &FakeLockProvider{
		store: &fakeLockStore{
			locks:        make(map[int64]*fakeLockStoreEntry),
			envToLockKey: make(map[string]int64),
		},
		conf: conf,
	}
}

func (flp *FakeLockProvider) LockKey(ctx context.Context, repo string, pr uint) (int64, error) {
	flp.store.mutex.Lock()
	defer flp.store.mutex.Unlock()
	envKey := fmt.Sprintf("%s/%d", repo, pr)
	lockKey, ok := flp.store.envToLockKey[envKey]
	if !ok {
		lockKey = rand.Int63()
		flp.store.envToLockKey[envKey] = lockKey
	}
	return lockKey, nil
}

func (flp *FakeLockProvider) New(ctx context.Context, key int64, event string) (PreemptableLock, error) {
	id, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}
	l := &FakePreemptableLock{
		id:      id,
		preempt: make(chan NotificationPayload),
		store:   flp.store,
		event:   event,
		key:     key,
		conf:    flp.conf,
	}
	flp.store.store(key, l)
	return l, nil
}

var _ PreemptableLock = &FakePreemptableLock{}

type FakePreemptableLock struct {
	store   *fakeLockStore
	id      uuid.UUID
	event   string
	key     int64
	preempt chan NotificationPayload
	conf    LockProviderConfig
}

func (fpl *FakePreemptableLock) Lock(ctx context.Context) (<-chan NotificationPayload, error) {
	lockCtx, cancel := context.WithTimeout(ctx, fpl.conf.lockTimeout)
	defer cancel()
	err := fpl.store.lock(lockCtx, fpl.key)
	if err != nil {
		return nil, fmt.Errorf("unable to lock fake preemptable lock: %v", err)
	}
	go func() {
		time.Sleep(fpl.conf.maxLockDuration)
		np := NotificationPayload{
			ID:      fpl.id,
			Message: "reached max lock duration",
			LockKey: fpl.key,
		}
		fpl.handleNotification(np)
	}()

	return fpl.preempt, nil
}

func (fpl *FakePreemptableLock) Unlock(ctx context.Context) error {
	return fpl.store.unlock(ctx, fpl.key, fpl.id)
}

func (fpl *FakePreemptableLock) Notify(ctx context.Context) error {
	return fpl.store.notify(ctx, fpl.key, fpl.id)
}

func (fpl *FakePreemptableLock) handleNotification(np NotificationPayload) {
	select {
	case fpl.preempt <- np:
		return
	case <-time.After(fpl.conf.preemptionTimeout):
		fpl.Unlock(context.Background())
	}
}
