package locker

import (
	"context"
	"fmt"
	"time"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"github.com/dollarshaveclub/acyl/pkg/eventlogger"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

var (
	defaultLockWaitTime = (30 * time.Minute) + (30 * time.Second) // global async timeout is 30 minutes
	defaultLockDelay    = 10 * time.Second
)

// LockProvider describes an object capable of creating locks
type LockProvider interface {
	NewLock(ctx context.Context, key int64, event string) (PreemptableLock, error)
}

// NotificationPayload represents the content of messages sent to the lock holder.
type NotificationPayload struct {
	// ID is required so that we can ensure the message came from another party.
	ID uuid.UUID `json:"id"`

	// Message provides some context as to why the notification was generated.
	// Useful for logging purposes.
	Message string `json:"event"`

	// The key that this Notification Payload pertains to
	LockKey int64
}

// PreemptiveLocker represents a distributed lock where callers can be preempted while waiting for the lock to be released or while holding the lock. High level, the algorithm is as follows:
// - Client A calls Lock() which returns immediately, Client A now has the lock. Client A periodically ensures the session is still alive. If Client A dies, the session expires and the lock is automatically released.
// - Client B calls Lock() which blocks since the lock is held by Client A.
// - Client A receives a value on the channel returned from the Lock() call indicating Client A should release the lock ASAP
// - Client C calls Lock() which blocks
// - Client B's invocation of Lock() returns with an error indicating it was preempted while waiting for the lock to release
// - Client A calls Unlock()
// - Client C's invocation of Lock() returns successfully
//
type PreemptiveLocker struct {
	lp    LockProvider
	lock  PreemptableLock
	conf  PreemptiveLockerConfig
	key   int64
	event string
}

// PreemptableLock describes an object that acts as a Lock that can signal to peers that they should unlock
// Preemptable locks are single use. Once you unlock the lock, underlying resources will be cleaned up
type PreemptableLock interface {
	// Lock locks the preemptable lock. If it's unable to obtain the lock after that timeout period, it should return an error
	Lock(ctx context.Context, lockWait time.Duration) (<-chan NotificationPayload, error)

	// Unlock unlocks the preemptable lock. It should clean up any underlying resources
	Unlock(ctx context.Context) error

	// Notify informs the current lock holder that they should unlock the lock
	Notify(ctx context.Context) error
}

// PreemptiveLockerConfig contains values for adjusting how the PreemptiveLocker behaves
type PreemptiveLockerConfig struct {
	// lockWait is how long a preemtive lock will block waiting for the lock to be acquired
	lockWait time.Duration

	// lockDelay is how long the locker should wait before attempting to acquire the lock
	// This is an imperfect, but practical way of ensuring we don't perform operations before other lock holders have realized that their session has ended
	lockDelay time.Duration

	// tracingServiceName is the service name used for APM
	tracingServiceName string

	// tracingEnabled determines whether the Preemptive Locker should actually report APM
	tracingEnabled bool
}

type PreemptiveLockerOption func(*PreemptiveLockerConfig)

func WithLockWait(lockWait time.Duration) PreemptiveLockerOption {
	return func(config *PreemptiveLockerConfig) {
		config.lockWait = lockWait
	}
}

func WithLockDelay(lockDelay time.Duration) PreemptiveLockerOption {
	return func(config *PreemptiveLockerConfig) {
		config.lockDelay = lockDelay
	}
}

func WithTracingServiceName(name string) PreemptiveLockerOption {
	return func(config *PreemptiveLockerConfig) {
		config.tracingServiceName = name
	}
}

func WithTracingEnabled(enabled bool) PreemptiveLockerOption {
	return func(config *PreemptiveLockerConfig) {
		config.tracingEnabled = enabled
	}
}

type PreemptiveLockerFactory func(key int64, event string) *PreemptiveLocker

// NewPreemptiveLocker returns a new preemptive locker or an error
func NewPreemptiveLockerFactory(provider LockProvider, opts ...PreemptiveLockerOption) (PreemptiveLockerFactory, error) {
	if provider == nil {
		return nil, errors.New("must provide non-nil LockProvider")
	}

	config := PreemptiveLockerConfig{}
	for _, opt := range opts {
		opt(&config)
	}
	if config.lockWait == 0 {
		config.lockWait = defaultLockWaitTime
	}
	if config.lockDelay == 0 {
		config.lockDelay = defaultLockDelay
	}
	var plf PreemptiveLockerFactory
	plf = func(key int64, event string) *PreemptiveLocker {
		return &PreemptiveLocker{
			lp:    provider,
			conf:  config,
			key:   key,
			event: event,
		}
	}
	return plf, nil
}

func (p *PreemptiveLocker) log(ctx context.Context, msg string, args ...interface{}) {
	eventlogger.GetLogger(ctx).Printf("preemptive locker: "+msg, args...)
}

func (p *PreemptiveLocker) startSpanFromContext(ctx context.Context, operationName string) (tracer.Span, context.Context) {
	if !p.conf.tracingEnabled {
		// return no-op span if tracing is disabled
		span, _ := tracer.SpanFromContext(context.Background())
		return span, ctx
	}

	span, ctx := tracer.StartSpanFromContext(ctx, operationName, tracer.ServiceName(p.conf.tracingServiceName))
	span.SetTag("lock_key", p.key)
	span.SetTag("event", p.event)
	return span, ctx
}

// Lock locks the lock and returns a channel used to signal if the lock should be released ASAP. If the lock is currently in use, this method will block until the lock is released. If caller is preempted while waiting for the lock to be released,
// an error is returned.
func (p *PreemptiveLocker) Lock(ctx context.Context) (ch <-chan NotificationPayload, err error) {
	span, ctx := p.startSpanFromContext(ctx, "lock")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

	// Ensure context is hasn't been deleted before beginning an expensive operation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("context was canceled before acquiring lock: %v", ctx.Err())
	default:
	}

	if p.lock != nil {
		return nil, errors.New("single use lock attempted to be reused")
	}
	lock, err := p.lp.NewLock(ctx, p.key, p.event)
	if err != nil {
		return nil, fmt.Errorf("unable to acquire lock")
	}

	p.lock = lock
	err = p.lock.Notify(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to notify other locks: %v", err)
	}

	ch, err = lock.Lock(ctx, p.conf.lockWait)
	if err != nil {
		return nil, fmt.Errorf("unable to lock: %v", err)
	}

	// Wait for the specified LockDelay before returning
	time.Sleep(p.conf.lockDelay)
	return ch, nil
}

// Release releases the lock. Should likely pass context.Background()
func (p *PreemptiveLocker) Release(ctx context.Context) (err error) {
	span, ctx := p.startSpanFromContext(ctx, "release")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

	if p.lock == nil {
		return errors.New("attempting to Release before the lock has been locked")
	}
	err = p.lock.Unlock(ctx)
	if err != nil {
		p.log(ctx, "error unlocking lock: %v", err)
	}
	return err
}
