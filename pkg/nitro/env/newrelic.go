package env

import (
	"context"

	newrelic "github.com/newrelic/go-agent"
)

const (
	nrTxnContextKey = "newrelic_txn"
)

type acylContextKey string

// newNRTxnContext returns a context with the New Relic transaction embedded as a value
func newNRTxnContext(ctx context.Context, txn newrelic.Transaction) context.Context {
	return context.WithValue(ctx, acylContextKey(nrTxnContextKey), txn)
}

// getNRTxnFromContext returns the New Relic transaction (or nil) and a boolean indicating whether it exists
func getNRTxnFromContext(ctx context.Context) (newrelic.Transaction, bool) {
	txn, ok := ctx.Value(acylContextKey(nrTxnContextKey)).(newrelic.Transaction)
	return txn, ok
}
