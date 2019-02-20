package locker

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	consul "github.com/hashicorp/consul/api"
	"github.com/pkg/errors"
)

var _ PreemptiveLockProvider = &FakePreemptiveLockProvider{}

// FakePreemptiveLockProvider fakes out all aspects of a preemptive lock
type FakePreemptiveLockProvider struct {
	ChannelFactory func() chan struct{}
}

func (fplp *FakePreemptiveLockProvider) NewPreemptiveLocker(repo, pr string, opts PreemptiveLockerOpts) *PreemptiveLocker {
	key := fmt.Sprintf("fake-%v/%v/lock", repo, pr)
	kv := NewFakeConsulKV()
	return &PreemptiveLocker{
		c:          &FakeLockFactory{ChannelFactory: fplp.ChannelFactory},
		kv:         kv,
		cs:         &FakeConsulSession{},
		w:          newKeyWatcher(kv, opts.LockWait),
		lockKey:    key,
		pendingKey: fmt.Sprintf("%s/pending", key),
		opts:       opts,
	}
}

var DefaultSessionCheckInterval time.Duration = 100 * time.Millisecond

// FakeRealLockFactory is a functional fake of a Consul lock that uses a mutex
type FakeRealLockFactory struct {
	CS *FakeConsulSession
	// SessionCheckInterval is the delay between checks to see if a lock's session has expired. If zero, DefaultSessionCheckInterval is used.
	SessionCheckInterval, LockWait time.Duration
	m                              map[string]*FakeRealPreemptableLock
}

var _ LockFactory = &FakeRealLockFactory{}

func (frlf *FakeRealLockFactory) destroy(key string) {
	delete(frlf.m, key)
}

// LockOpts returns a new PreemptableLock for lo.Key or the existing one, if found
func (frlf *FakeRealLockFactory) LockOpts(lo *consul.LockOptions) (PreemptableLock, error) {
	if l, ok := frlf.m[lo.Key]; ok {
		return l, nil
	}
	if sess, _, err := frlf.CS.Info(lo.Session, nil); sess == nil || err != nil {
		return nil, fmt.Errorf("session query error or session not found: %v", err)
	}
	l := &FakeRealPreemptableLock{
		lo:            lo,
		df:            func() { frlf.destroy(lo.Key) },
		cs:            frlf.CS,
		checkinterval: frlf.SessionCheckInterval,
		lockwait:      frlf.LockWait,
		lockchan:      make(chan struct{}, 1),
		preempt:       make(chan struct{}),
		stop:          make(chan struct{}),
	}
	if l.checkinterval == 0 {
		l.checkinterval = DefaultSessionCheckInterval
	}
	if frlf.m == nil {
		frlf.m = make(map[string]*FakeRealPreemptableLock)
	}
	frlf.m[lo.Key] = l
	return l, nil
}

// FakeRealPreemptableLock is a fake PreemptableLock that functions as closely as possible as a Consul lock.
type FakeRealPreemptableLock struct {
	lockchan                chan struct{}
	lo                      *consul.LockOptions
	df                      func()
	cs                      *FakeConsulSession
	checkinterval, lockwait time.Duration
	preempt, stop           chan struct{}
}

var _ PreemptableLock = &FakeRealPreemptableLock{}

func (frpl *FakeRealPreemptableLock) Destroy() error {
	close(frpl.stop)
	close(frpl.preempt)
	close(frpl.lockchan)
	frpl.df()
	return nil
}
func (frpl *FakeRealPreemptableLock) checksession() {
	for {
		select {
		case <-frpl.stop:
			return
		case <-time.After(frpl.checkinterval):
			if expired, _ := frpl.cs.Expired(frpl.lo.Session); expired {
				// if the session is expired, signal the lock holder to release
				close(frpl.preempt)
				// wait LockDelay, then simulate releasing the lock if the lockholder hasn't released it
				sess, _, _ := frpl.cs.Info(frpl.lo.Session, nil)
				time.Sleep(sess.LockDelay)
				if len(frpl.lockchan) > 0 {
					<-frpl.lockchan
				}
			}
			return
		}
	}
}
func (frpl *FakeRealPreemptableLock) Lock(stopCh <-chan struct{}) (<-chan struct{}, error) {
	select {
	case <-time.After(frpl.lockwait):
		return nil, errors.New("reached wait time maximum")
	case frpl.lockchan <- struct{}{}:
		go frpl.checksession()
		return frpl.preempt, nil
	case <-stopCh:
		return nil, errors.New("stop channel was closed")
	}
}
func (frpl *FakeRealPreemptableLock) Unlock() error {
	<-frpl.lockchan
	return nil
}

var _ LockFactory = &FakeLockFactory{}

// FakeLockFactory returns a FakePreemptableLock using ChannelFactory
type FakeLockFactory struct {
	ChannelFactory func() chan struct{}
}

func (flf *FakeLockFactory) LockOpts(*consul.LockOptions) (PreemptableLock, error) {
	return &FakePreemptableLock{Preempt: flf.ChannelFactory()}, nil
}

var _ PreemptableLock = &FakePreemptableLock{}

// FakePreempatableLock is a faked out version of a preemptable lock
type FakePreemptableLock struct {
	Preempt chan struct{}
}

func (fpl *FakePreemptableLock) Destroy() error { return nil }
func (fpl *FakePreemptableLock) Lock(stopCh <-chan struct{}) (<-chan struct{}, error) {
	return fpl.Preempt, nil
}
func (fpl *FakePreemptableLock) Unlock() error { return nil }

type lockingFakeConsulData struct {
	sync.RWMutex
	data map[string]*consul.KVPair
}

// FakeConsulKV is a faked out version of a Consul KV, for use with a real lock
type FakeConsulKV struct {
	d lockingFakeConsulData
}

func NewFakeConsulKV() *FakeConsulKV {
	return &FakeConsulKV{d: lockingFakeConsulData{data: make(map[string]*consul.KVPair)}}
}

func (fck *FakeConsulKV) Put(kvp *consul.KVPair, wo *consul.WriteOptions) (*consul.WriteMeta, error) {
	fck.d.Lock()
	defer fck.d.Unlock()
	fck.d.data[kvp.Key] = kvp
	return &consul.WriteMeta{}, nil
}

func (fck *FakeConsulKV) Get(key string, qo *consul.QueryOptions) (*consul.KVPair, *consul.QueryMeta, error) {
	fck.d.RLock()
	defer fck.d.RUnlock()
	kv, ok := fck.d.data[key]
	if !ok {
		return nil, nil, errors.New("kv not found")
	}
	return kv, &consul.QueryMeta{}, nil
}

func (fck *FakeConsulKV) DeleteCAS(kv *consul.KVPair, wo *consul.WriteOptions) (bool, *consul.WriteMeta, error) {
	fck.d.Lock()
	defer fck.d.Unlock()
	delete(fck.d.data, kv.Key)
	return true, nil, nil
}

type fakesession struct {
	sync.RWMutex
	expires time.Time
	sess    *consul.SessionEntry
}

// FakeConsulKV is a faked out version of a Consul Session store, for use with a real lock
type FakeConsulSession struct {
	sync.RWMutex
	m map[string]*fakesession
}

// Expired checks if the session is expired (this is not part of the Consul interface)
func (fcs *FakeConsulSession) Expired(id string) (bool, error) {
	fcs.RLock()
	defer fcs.RUnlock()
	fs, ok := fcs.m[id]
	if !ok {
		return false, errors.New("session not found")
	}
	fs.RLock()
	defer fs.RUnlock()
	return time.Now().UTC().After(fs.expires), nil
}

// SetExpires manually sets the expired timestamp for a session (this is not part of the Consul interface)
func (fcs *FakeConsulSession) SetExpires(id string, expires time.Time) error {
	fcs.RLock()
	defer fcs.RUnlock()
	fs, ok := fcs.m[id]
	if !ok {
		return errors.New("session not found")
	}
	fs.Lock()
	fs.expires = expires
	fs.Unlock()
	return nil
}

func (fcs *FakeConsulSession) Info(id string, q *consul.QueryOptions) (*consul.SessionEntry, *consul.QueryMeta, error) {
	fcs.RLock()
	defer fcs.RUnlock()
	fs, ok := fcs.m[id]
	if !ok {
		return nil, nil, nil
	}
	fs.RLock()
	defer fs.RUnlock()
	sess := *fs.sess // copy value so pointer does not escape mutex protection
	return &sess, nil, nil
}

func (fcs *FakeConsulSession) Create(se *consul.SessionEntry, q *consul.WriteOptions) (string, *consul.WriteMeta, error) {
	fcs.Lock()
	defer fcs.Unlock()
	se.ID = uuid.New().String()
	if fcs.m == nil {
		fcs.m = make(map[string]*fakesession)
	}
	ttl, err := time.ParseDuration(se.TTL)
	if err != nil {
		return "", nil, errors.Wrap(err, "error parsing TTL")
	}
	fcs.m[se.ID] = &fakesession{
		expires: time.Now().UTC().Add(ttl),
		sess:    se,
	}
	return se.ID, nil, nil
}

func (fcs *FakeConsulSession) RenewPeriodic(initialTTL string, id string, q *consul.WriteOptions, doneCh <-chan struct{}) error {
	ttl, err := time.ParseDuration(initialTTL)
	if err != nil {
		return errors.Wrap(err, "error parsing initial TTL")
	}

	fcs.RLock()
	fs, ok := fcs.m[id]
	if !ok {
		fcs.RUnlock()
		return errors.New("session not found: " + id)
	}
	fcs.RUnlock()

	waitDur := ttl / 2
	lastRenewTime := time.Now().UTC()
	for {
		if time.Since(lastRenewTime) > ttl {
			return errors.New("session not renewed within ttl period")
		}
		select {
		case <-time.After(waitDur):
			now := time.Now().UTC()
			fs.RLock()
			if now.After(fs.expires) {
				fs.RUnlock()
				return errors.New("session expired")
			}
			fs.RUnlock()
			fs.Lock()
			fs.expires = now.Add(ttl)
			fs.Unlock()
			lastRenewTime = now
		case <-doneCh:
			fcs.Lock()
			delete(fcs.m, id)
			fcs.Unlock()
			return nil
		}
	}
}
