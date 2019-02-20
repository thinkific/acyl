package env

import (
	"net/http"
	"time"

	newrelic "github.com/newrelic/go-agent"
)

// FakeNewRelicApplication conforms to the newrelic.Application interface but does nothing
type FakeNewRelicApplication struct {
}

func (fnra FakeNewRelicApplication) StartTransaction(name string, w http.ResponseWriter, r *http.Request) newrelic.Transaction {
	return &FakeNewRelicTransaction{}
}

func (fnra FakeNewRelicApplication) RecordCustomEvent(eventType string, params map[string]interface{}) error {
	return nil
}

func (fnra FakeNewRelicApplication) WaitForConnection(timeout time.Duration) error {
	return nil
}

func (fnra FakeNewRelicApplication) Shutdown(timeout time.Duration) {}

// FakeNewRelicTransaction conforms to the newrelic.Transaction interface but does nothing
type FakeNewRelicTransaction struct {
}

func (fnrt FakeNewRelicTransaction) Header() http.Header {
	return make(http.Header)
}

func (fnrt FakeNewRelicTransaction) Write(b []byte) (int, error) {
	return len(b), nil
}

func (fnrt FakeNewRelicTransaction) WriteHeader(n int) {}

func (fnrt FakeNewRelicTransaction) End() error {
	return nil
}

func (fnrt FakeNewRelicTransaction) Ignore() error {
	return nil
}

func (fnrt FakeNewRelicTransaction) SetName(name string) error {
	return nil
}

func (fnrt FakeNewRelicTransaction) NoticeError(err error) error {
	return nil
}

func (fnrt FakeNewRelicTransaction) AddAttribute(key string, value interface{}) error {
	return nil
}

func (fnrt FakeNewRelicTransaction) StartSegmentNow() newrelic.SegmentStartTime {
	return newrelic.SegmentStartTime{}
}
