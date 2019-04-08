package locker

import (
	"fmt"
	"time"

	consul "github.com/hashicorp/consul/api"
)

// PreemptiveLockProvider describes an object capable of creating PreemptiveLockers
type PreemptiveLockProvider interface {
	NewPreemptiveLocker(repo, pr string, opts PreemptiveLockerOpts) *PreemptiveLocker
}

// LockProvider describes an object capable of creating and releasing locks
type LockProvider interface {
	AcquireLock(repo, pr string) (Lock, error)
	AcquireNamedLock(string, string) (Lock, error)
}

// ConsulLocker uses Consul to provide a lock service
type ConsulLocker struct {
	pfx                string
	c                  *consul.Client
	datadogServiceName string
	enableTracing      bool
}

// Lock describes an object representing a held lock that can be released
type Lock interface {
	Release() error
}

// ConsulLock is an object representing a lock on a repo/PR combination using Consul
type ConsulLock struct {
	l *consul.Lock
}

// Release frees and cleans up the lock
func (cl *ConsulLock) Release() error {
	if cl.l != nil {
		err := cl.l.Unlock()
		if err != nil {
			return err
		}
		err = cl.l.Destroy()
		if err != nil {
			return err
		}
	}
	cl.l = nil
	return nil
}

// NewConsulLocker returns a ConsulLocker using the Consul agent at caddr and using
// pfx as the lock name prefix
func NewConsulLocker(caddr, pfx, datadogServiceNamePrefix string, enableTracing bool) (*ConsulLocker, error) {
	c, err := consul.NewClient(&consul.Config{
		Address: caddr,
		Scheme:  "http",
	})
	if err != nil {
		return nil, err
	}
	return &ConsulLocker{
		c:                  c,
		pfx:                pfx,
		datadogServiceName: datadogServiceNamePrefix + ".consul",
		enableTracing:      enableTracing,
	}, nil
}

func (cl *ConsulLocker) getlock(key string, ttl string) (Lock, error) {
	lo := &consul.LockOptions{
		Key:         key,
		LockTryOnce: true,
		SessionOpts: &consul.SessionEntry{
			Name:     fmt.Sprintf("session for %s", key),
			TTL:      ttl,
			Behavior: "delete",
		},
	}
	lock, err := cl.c.LockOpts(lo)
	if err != nil {
		return nil, err
	}
	lo.LockWaitTime = 1 * time.Second
	ch, err := lock.Lock(nil)
	if err != nil {
		return nil, err
	}
	if ch == nil { // can not acquire lock
		return nil, nil
	}
	return &ConsulLock{
		l: lock,
	}, nil
}

// NewPreemptiveLocker creates a new PreemptiveLocker for the specified repo and pr
func (cl *ConsulLocker) NewPreemptiveLocker(repo, pr string, opts PreemptiveLockerOpts) *PreemptiveLocker {
	return NewPreemptiveLocker(NewConsulLockFactory(cl.c), cl.c.KV(), cl.c.Session(), fmt.Sprintf("%v%v/%v/lock", cl.pfx, repo, pr), opts, cl.datadogServiceName, true)
}

// AcquireLock attempts to acquire a lock for a specific repo/PR combination or
// returns nil if the lock could not be acquired. Client is responsible for holding
// on to the returned lock and releasing it when finished
func (cl *ConsulLocker) AcquireLock(repo, pr string) (Lock, error) {
	return cl.getlock(fmt.Sprintf("%v%v/%v/lock", cl.pfx, repo, pr), defaultSessionTTL.String())
}

// AcquireNamedLock attempts to acquire a lock with the specified name and TTL or
// returns nil if the lock could not be acquired. Client is responsible for holding
// on to the returned lock and releasing it when finished
func (cl *ConsulLocker) AcquireNamedLock(name string, ttl string) (Lock, error) {
	return cl.getlock(fmt.Sprintf("%v%v", cl.pfx, name), ttl)
}
