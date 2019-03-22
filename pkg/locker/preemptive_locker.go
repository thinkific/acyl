package locker

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"github.com/dollarshaveclub/acyl/pkg/eventlogger"

	"github.com/google/uuid"
	consul "github.com/hashicorp/consul/api"
	"github.com/pkg/errors"
)

var (
	defaultSessionTTL   = 30 * time.Second
	defaultLockWaitTime = (30 * time.Minute) + (30 * time.Second) // global async timeout is 30 minutes
	defaultLockDelay    = 15 * time.Second
)

// PreemptedError is returned when a Lock() operation is preempted
type PreemptedError interface {
	Preempted() bool
}

// PreemptiveLocker represents a distributed lock where callers can be preempted while waiting for the lock to be released or while holding the lock. High level, the algorithm is as follows:
// - Client A calls Lock() which returns immediately, Client A now has the lock. Client A periodically renews the underlying Consul session. If Client A dies, the session expires and the lock is automatically released.
// - Client B calls Lock() which blocks since the lock is held by Client A.
// - Client A receives a value on the channel returned from the Lock() call indicating Client A should release the lock ASAP
// - Client C calls Lock() which blocks
// - Client B's invocation of Lock() returns with an error indicating it was preempted while waiting for the lock to release
// - Client A calls Unlock()
// - Client C's invocation of Lock() returns successfully
//
type PreemptiveLocker struct {
	c          LockFactory
	kv         ConsulKV
	cs         ConsulSession
	l          PreemptableLock
	w          *keyWatcher
	lockKey    string
	pendingKey string
	id         string
	sessionid  string
	opts       PreemptiveLockerOpts
}

// LockFactory describes an object capable of creating a Consul lock
type LockFactory interface {
	LockOpts(*consul.LockOptions) (PreemptableLock, error)
}

// ConsulKV describes an object capable of using Consul as a KV store
type ConsulKV interface {
	Put(*consul.KVPair, *consul.WriteOptions) (*consul.WriteMeta, error)
	Get(string, *consul.QueryOptions) (*consul.KVPair, *consul.QueryMeta, error)
	DeleteCAS(*consul.KVPair, *consul.WriteOptions) (bool, *consul.WriteMeta, error)
}

// ConsulSession describes an object capable of interacting with Consul sessions
type ConsulSession interface {
	Create(se *consul.SessionEntry, q *consul.WriteOptions) (string, *consul.WriteMeta, error)
	RenewPeriodic(initialTTL string, id string, q *consul.WriteOptions, doneCh <-chan struct{}) error
}

// PreemptableLock describes an object that acts as a Lock
type PreemptableLock interface {
	Destroy() error
	Lock(stopCh <-chan struct{}) (<-chan struct{}, error)
	Unlock() error
}

// ConsulLockFactory implements the LockFactory interface for a consul client
type ConsulLockFactory struct {
	c *consul.Client
}

// NewConsulLockFactory creates a new ConsulLockFactory
func NewConsulLockFactory(consul *consul.Client) ConsulLockFactory {
	return ConsulLockFactory{c: consul}
}

// LockOpts returns a preeptable lock with the specified options or an error
func (f ConsulLockFactory) LockOpts(opt *consul.LockOptions) (PreemptableLock, error) {
	return f.c.LockOpts(opt)
}

// PreemptiveLockerOpts contains options for the locks produced by PreemptiveLocker
type PreemptiveLockerOpts struct {
	// SessionTTL is how long the underlying Consul session will live between heartbeats
	// LockWait is how long a preemtive lock will block waiting for the lock to be acquired
	// LockDelay is how long to wait after a lock session has been forcefully invalidated before allowing a new client to acquire the lock
	SessionTTL, LockWait, LockDelay time.Duration
}

// NewPreemptiveLocker returns a new preemptive locker or an error
func NewPreemptiveLocker(factory LockFactory, kv ConsulKV, cs ConsulSession, key string, opts PreemptiveLockerOpts) *PreemptiveLocker {
	if opts.SessionTTL == 0 {
		opts.SessionTTL = defaultSessionTTL
	}
	if opts.LockWait == 0 {
		opts.LockWait = defaultLockWaitTime
	}
	if opts.LockDelay == 0 {
		opts.LockDelay = defaultLockDelay
	}
	return &PreemptiveLocker{
		c:          factory,
		kv:         kv,
		cs:         cs,
		w:          newKeyWatcher(kv, opts.LockWait),
		lockKey:    key,
		pendingKey: fmt.Sprintf("%s/pending", key),
		opts:       opts,
	}
}

func (p *PreemptiveLocker) log(ctx context.Context, msg string, args ...interface{}) {
	eventlogger.GetLogger(ctx).Printf("preemptive locker: "+msg, args...)
}

// Lock locks the lock and returns a channel used to signal if the lock should be released ASAP. If the lock is currently in use, this method will block until the lock is released. If caller is preemptied while waiting for the lock to be released,
// an error is returned.
func (p *PreemptiveLocker) Lock(ctx context.Context) (_ <-chan interface{}, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "lock", tracer.ServiceName("consul"))
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	if len(p.id) != 0 {
		return nil, errors.New("Lock() called again.")
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, errors.Wrap(err, "error getting random id")
	}
	return p.lockWithID(ctx, fmt.Sprintf("%d-%s", time.Now().UTC().Unix(), id.String()))
}

func (p *PreemptiveLocker) lockWithID(ctx context.Context, id string) (<-chan interface{}, error) {
	p.id = id
	p.log(ctx, "consul put: pending key: %v; value: %v", p.pendingKey, id)
	_, err := p.kv.Put(&consul.KVPair{Key: p.pendingKey, Value: []byte(id)}, nil)
	if err != nil {
		return nil, errors.Wrap(err, "error in KV put")
	}

	wout, _ := p.w.Watch(ctx, id, p.pendingKey, []byte(id))

	lock, err := p.newLock()
	if err != nil {
		return nil, errors.Wrap(err, "error getting new lock")
	}

	p.log(ctx, "created new lock: %+v with session id: %v", lock, p.sessionid)

	lout, leout := p.acquireLock(ctx, lock)
	for {
		select {
		case <-wout: // we got preempted while acquiring lock
			p.w.Stop()

			// if lock was acquired, release it
			select {
			case lc := <-lout:
				if lc != nil {
					lock.Unlock()
				}
			default:
			}
			return nil, perror("preempted")
		case lc := <-lout: // acquired lock
			p.l = lock

			// We multiplex the two channels that indicate the lock needs to be released
			// Ideally, this isn't necessary but it's needed since Consul's lock implementation returns a channel that indicates the lock
			// should be released and our key watcher returns a channel that indicates the lock should be released
			// so we have to multiplex both of those channels into one.
			return multiplex(lc, wout), nil
		case le := <-leout: // an error occurred while acquiring lock
			p.w.Stop()
			return nil, le
		case <-ctx.Done(): // context was cancelled
			p.w.Stop()
			return nil, perror("context was cancelled while acquiring lock")
		}
	}
}

// Release releases the lock
func (p *PreemptiveLocker) Release(ctx context.Context) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "release", tracer.ServiceName("consul"))
	p.w.Stop()
	defer func() {
		err = p.l.Unlock() // always unlock
		span.Finish(tracer.WithError(err))
	}()
	var kp *consul.KVPair
	kp, _, _ = p.kv.Get(p.pendingKey, &consul.QueryOptions{AllowStale: false, WaitIndex: uint64(0), WaitTime: p.opts.LockWait})
	if p.id == string(kp.Value[:]) {
		p.kv.DeleteCAS(&consul.KVPair{Key: p.pendingKey, Value: nil}, nil)
	}
	return nil
}

// WasPreempted returns true if the provided error was returned due to the caller being preempted while waiting for the lock to be released
func (p *PreemptiveLocker) WasPreempted(err error) bool {
	_, ok := err.(PreemptedError)
	return ok
}

func (p *PreemptiveLocker) newLock() (PreemptableLock, error) {
	// create session
	se := &consul.SessionEntry{
		Name:     fmt.Sprintf("session for %s", p.lockKey),
		TTL:      p.opts.SessionTTL.String(),
		Behavior: "delete",
		// LockDelay means wait this period after a session is invalidated before allowing new processes to obtain the lock
		// This allows time for the previous lockholder to notice the invalidation and do any cleanup work
		// https://www.consul.io/docs/internals/sessions.html#session-design (last paragraph)
		LockDelay: p.opts.LockDelay,
	}
	id, _, err := p.cs.Create(se, nil)
	if err != nil {
		return nil, errors.Wrap(err, "error creating lock session")
	}
	p.sessionid = id
	// create lock based on session
	lo := &consul.LockOptions{
		Key:          p.lockKey,
		LockWaitTime: 10 * time.Minute,
		Session:      id,
	}
	pl, err := p.c.LockOpts(lo)
	if err != nil {
		return nil, errors.Wrap(err, "error creating preemptive lock with options")
	}
	return pl, nil
}

func (p *PreemptiveLocker) acquireLock(ctx context.Context, lock PreemptableLock) (<-chan <-chan struct{}, <-chan error) {
	// channels are buffered because either zero or one values will be written to each
	lout := make(chan (<-chan struct{}), 1)
	leout := make(chan error, 1)
	go func() {
		p.log(ctx, "starting periodic session renewal: ttl: %v", p.opts.SessionTTL)
		if err := p.cs.RenewPeriodic(p.opts.SessionTTL.String(), p.sessionid, nil, ctx.Done()); err != nil {
			p.log(ctx, "error renewing session: %v", err)
		}
	}()
	go func() {
		p.log(ctx, "attempting to acquire lock")
		ch, err := lock.Lock(ctx.Done())
		if err != nil {
			p.log(ctx, "error acquiring lock: %v", err)
			leout <- errors.Wrap(err, "error in Lock")
		}
		lout <- ch
	}()

	return lout, leout
}

type perror string

func (pe perror) Error() string {
	return string(pe)
}
func (pe perror) Preempted() bool {
	return true
}

type keyWatcher struct {
	kv            ConsulKV
	cancels       []context.CancelFunc
	watchInterval time.Duration
	waitTime      time.Duration
}

func (p *keyWatcher) log(ctx context.Context, msg string, args ...interface{}) {
	eventlogger.GetLogger(ctx).Printf("preemptive locker: key watcher: "+msg, args...)
}

func newKeyWatcher(kv ConsulKV, waittime time.Duration) *keyWatcher {
	return &keyWatcher{kv: kv, watchInterval: consul.DefaultMonitorRetryTime, waitTime: waittime}
}

func (k *keyWatcher) Watch(ctx context.Context, lockid, key string, current []byte) (<-chan []byte, <-chan error) {
	k.log(ctx, "lock id: %v: starting key watch: %v; current value: %v", lockid, key, string(current))
	canCtx, cancel := context.WithCancel(ctx)
	k.cancels = append(k.cancels, cancel)
	c := make(chan []byte)
	e := make(chan error)

	go func() {
		k.internalWatch(canCtx, lockid, c, e, key, current)
		close(c)
	}()

	return c, e
}

func (k *keyWatcher) Stop() {
	for _, cancel := range k.cancels {
		cancel()
	}
}

func (k *keyWatcher) internalWatch(ctx context.Context, lockid string, ch chan []byte, ech chan error, key string, current []byte) {
	index := uint64(0)
	val := current

	for ctx.Err() == nil {
		time.Sleep(k.watchInterval)
		kv, err := k.getWithRetry(key, index)
		if err != nil {
			ech <- errors.Wrap(err, "error in getWithRetry")
		}

		index = kv.ModifyIndex
		if !bytes.Equal(val, kv.Value) {
			k.log(ctx, "lockid: %v: key watch: new value: %v (old value: %v)", lockid, string(kv.Value), string(val))
			val = kv.Value
			ch <- val
		}
	}
}

func (k *keyWatcher) getWithRetry(key string, index uint64) (*consul.KVPair, error) {
	retryCount := 3
	for {
		var kp *consul.KVPair
		kp, _, err := k.kv.Get(key, &consul.QueryOptions{AllowStale: false, WaitIndex: index, WaitTime: k.waitTime})

		if err != nil {
			if consul.IsServerError(err) && retryCount > 0 {
				retryCount--
				time.Sleep(k.watchInterval)
				continue
			}

			return nil, errors.Wrap(err, "error in KV get")
		}

		return kp, nil
	}
}

type sender int

const (
	leftChan sender = iota
	rightChan
)

type mout struct {
	source sender
	lOut   *struct{}
	rOut   []byte
}

func multiplex(left <-chan struct{}, right <-chan []byte) <-chan interface{} {
	ch := make(chan interface{})
	go func() {
		for {
			select {
			case v, ok := <-left:
				if !ok {
					close(ch)
					return
				}
				ch <- mout{source: leftChan, lOut: &v}
			case v, ok := <-right:
				if !ok {
					close(ch)
					return
				}
				ch <- mout{source: rightChan, rOut: v}
			}
		}
	}()

	return ch
}
