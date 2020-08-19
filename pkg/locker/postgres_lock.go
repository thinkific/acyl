package locker

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/dollarshaveclub/acyl/pkg/eventlogger"
	"github.com/google/uuid"
	sqltrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/database/sql"
	sqlxtrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/jmoiron/sqlx"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/pkg/errors"
)

const (
	defaultKeepAlivePeriod  = 5 * time.Second
	defaultFailureThreshold = 2
)

type postgresSessionController struct {
	// failureThreshold is the number of consecutive failures before marking connection as failed
	failureThreshold int

	// failreCount is the current count of consecutive failures
	failureCount int

	// keepAlivePeriod determines how often we will ping Postgres to ensure the connection is still working
	keepAlivePeriod time.Duration

	// sessionErr allows us to propagate session errors to the user of this struct
	sessionErr chan error

	// conn is the underlying sql connection
	conn *sql.Conn
}

func newPostgresSessionController(ctx context.Context, keepAlivePeriod time.Duration, conn *sql.Conn, failureThreshold int) *postgresSessionController {
	if keepAlivePeriod == time.Duration(0) {
		keepAlivePeriod = defaultKeepAlivePeriod
	}

	if failureThreshold == 0 {
		failureThreshold = defaultFailureThreshold
	}
	return &postgresSessionController{
		keepAlivePeriod: keepAlivePeriod,
		conn:            conn,
		sessionErr:      make(chan error),
	}
}

// run is a blocking function that periodically checks the health of the connection
func (psc *postgresSessionController) run(ctx context.Context) {
	t := time.NewTicker(psc.keepAlivePeriod)
	for {
		select {
		case <-ctx.Done():
			psc.sessionErr <- ctx.Err()
			return
		case <-t.C:
			err := psc.sendKeepAlive(ctx)
			if err != nil {
				psc.failureCount++
				if psc.failureCount > psc.failureThreshold {
					psc.sessionErr <- err
				}
				return
			}
			psc.failureCount = 0
		}
	}
}

func (psc *postgresSessionController) sendKeepAlive(ctx context.Context) error {
	return psc.conn.PingContext(ctx)
}

type postgresLock struct {
	// id is a unique identifier for this lock. This way, we can determine if the notifications we receive are from other locks
	id uuid.UUID

	// psc allows the lock to determine if there are any issues with the underlying postgres connection
	psc *postgresSessionController

	// key is the key that will be locked. For Acyl, this might typically be the Repo/PR combination
	key string

	// sum32 is the checksum value for the key. Required because postgres advisory locks require uint32 as the key
	sum32 uint32

	// We want to use a single connection as the Advisory Lock we will be using will be a session-level lock
	// This means that the lock is released in 2 conditions:
	// 1. The lock is explicitly released (e.g. via pg_advisory_unlock(key bigint))
	// 2. The session ended (i.e. the tcp connection terminated)
	// Using a single connection will allow us to detect if the session has ended more reliably than using a connection pool
	// conn is the single connection to postgres that will be used for the lock
	conn *sql.Conn

	// postgresURI stores the postgres connection string.
	postgresURI string

	// listener represents a Postgres listener, which watches for Notifications over a defined channel.
	listener *pq.Listener

	// preempted is a channel which contains payloads from the Postgres Listener.
	preempted chan NotificationPayload

	// message represents a message to use when notifying other lock holders that their operation has been preempted
	message string
}

func newPostgresLock(ctx context.Context, db *sqlx.DB, key, connInfo, message string) (pl *postgresLock, err error) {
	id, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}

	// TODO: Consider additional measures to reduce the chance of collision
	sum32, err := hashSum32([]byte(key))
	if err != nil {
		return nil, fmt.Errorf("unable to obtain hash sum32 for key %s: %v", key, err)
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to obtain connection from pool: %v", err)
	}
	defer func() {
		if err != nil {
			conn.Close()
		}
	}()

	psc := newPostgresSessionController(ctx, defaultKeepAlivePeriod, conn, defaultFailureThreshold)

	go func() {
		psc.run(ctx)
	}()

	pl = &postgresLock{
		id:          id,
		psc:         psc,
		key:         key,
		sum32:       sum32,
		conn:        conn,
		postgresURI: connInfo,
		preempted:   make(chan NotificationPayload),
		message:     message,
	}
	return pl, nil
}

// handleEvents is a blocking function.
// It checks the multiple different channels to determine how to proceed
func (pl *postgresLock) handleEvents(ctx context.Context, listener *pq.Listener) {
	for {
		select {
		case <-ctx.Done():
			pl.preempted <- NotificationPayload{
				ID:      pl.id,
				Message: ctx.Err().Error(),
			}
			return
		case err := <-pl.psc.sessionErr:
			if err != nil {
				pl.preempted <- NotificationPayload{
					ID:      pl.id,
					Message: err.Error(),
				}
			}
			return
		case notification := <-listener.Notify:
			if notification == nil {
				pl.log(ctx, "received nil notiifcation")
				return
			}
			payload := notification.Extra
			np := NotificationPayload{}
			if err := json.Unmarshal([]byte(payload), &np); err != nil {
				pl.log(ctx, "could not unmarshal notification payload: %v", err)
				// In the event that we receive an unknown notification payload, we will give up the lock.
				// This could help for debugging since we can execute a Notify query to force the app to release the lock.
				// It could also help if we accidentally make a breaking change to the Notification payload.
				pl.preempted <- NotificationPayload{
					ID:      pl.id,
					Message: "received unknown notification payload",
				}
				return
			}
			if np.ID == pl.id {
				pl.log(ctx, "received our own notification, ignoring")
				return
			}
			// We have received a legimate notification and the lock has been preempted.
			pl.preempted <- np
			return
		}
	}
}

func (pl *postgresLock) Notify(ctx context.Context) error {
	np := NotificationPayload{
		ID:        pl.id,
		Message:   pl.message,
		Preempted: true,
	}
	b, err := json.Marshal(np)
	if err != nil {
		return fmt.Errorf("unable to encode NotificationPayload: %v", err)
	}
	q := fmt.Sprintf("NOTIFY  %s, %s", pq.QuoteIdentifier(pl.key), pq.QuoteLiteral(string(b)))
	_, err = pl.conn.ExecContext(ctx, q)
	if err != nil {
		return fmt.Errorf("unable to execute postgres notify command: %v", err)
	}
	return nil
}

func hashSum32(data []byte) (uint32, error) {
	h := fnv.New32a()
	n, err := h.Write(data)
	if n == 0 || err != nil {
		return 0, fmt.Errorf("unable to write to hash: %v", err)
	}
	return h.Sum32(), nil
}

func (pl *postgresLock) Unlock(ctx context.Context) error {
	defer func() {
		// Even if we fail to unlock via Postgres properly, destroying the lock should close the underlying sql.Conn.
		// At that point, Postgres should clean up the connection.
		// TODO (mk): Should we use the passed context or protect users from passing in canceled contexts?
		pl.destroy(context.Background())
	}()

	q := `SELECT pg_advisory_unlock($1)`
	// TODO (mk): Should we use the passed context or protect users from passing in canceled contexts?
	_, err := pl.conn.ExecContext(context.Background(), q, pl.sum32)
	if err != nil {
		return fmt.Errorf("unable to unlock advisory lock: %v", err)
	}
	return nil
}

func (pl *postgresLock) Lock(ctx context.Context, lockWait time.Duration) (<-chan NotificationPayload, error) {
	query := `SELECT pg_advisory_lock($1)`
	advLockContext, cancel := context.WithTimeout(ctx, lockWait)
	defer cancel()
	_, err := pl.conn.ExecContext(advLockContext, query, pl.sum32)
	if err != nil {
		return nil, fmt.Errorf("unable to create pg advisory lock: %v", err)
	}

	handleEvents := func(event pq.ListenerEventType, err error) {
		// TODO (mk): Reconsider how we want to handle these events after we have enough usage
		if err != nil {
			pl.log(ctx, "received error when handling postgres listener event: %v", err)
		}
	}

	listener := pq.NewListener(pl.postgresURI, 10*time.Second, time.Minute, handleEvents)
	err = listener.Listen(pl.key)
	if err != nil {
		return nil, fmt.Errorf("unable to establish listener: %v", err)
	}
	pl.listener = listener
	go func() {
		pl.handleEvents(ctx, pl.listener)
	}()

	return pl.preempted, nil
}

func (pl *postgresLock) destroy(ctx context.Context) {
	if pl.listener != nil {
		err := pl.listener.Close()
		if err != nil {
			pl.log(ctx, "unable to close the listener: %v", err)
		}
	}
	if pl.conn != nil {
		err := pl.conn.Close()
		if err != nil {
			pl.log(ctx, "unable to close the connection")
		}
	}
}

func (pl *postgresLock) log(ctx context.Context, msg string, args ...interface{}) {
	eventlogger.GetLogger(ctx).Printf("postgres lock: "+msg, args...)
}

type PostgresLockProvider struct {
	db       *sqlx.DB
	connInfo string
}

// NewPostgresLockProvider returns a PostgresLockProvider, which implements the LockProvider interface
// It utilizes advisory locks and Notify / Listen in order to provide PreemptableLocks
func NewPostgresLockProvider(postgresURI, datadogServiceName string, enableTracing bool) (*PostgresLockProvider, error) {
	var db *sqlx.DB
	var err error
	if enableTracing {
		sqltrace.Register("postgres", &pq.Driver{}, sqltrace.WithServiceName(datadogServiceName))
		db, err = sqlxtrace.Open("postgres", postgresURI)
	} else {
		db, err = sqlx.Open("postgres", postgresURI)
	}
	if err != nil {
		return nil, errors.Wrap(err, "error opening db")
	}
	return &PostgresLockProvider{db: db, connInfo: postgresURI}, nil
}

func (plp *PostgresLockProvider) AcquireLock(ctx context.Context, key, event string) (PreemptableLock, error) {
	return newPostgresLock(ctx, plp.db, key, plp.connInfo, event)
}
