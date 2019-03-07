package notifier

import (
	"errors"
	"testing"

	"github.com/nlopes/slack"
)

type testBackend struct {
	f func(n Notification) error
}

func (tb *testBackend) Send(n Notification) error {
	return tb.f(n)
}

func TestFanOut(t *testing.T) {
	tb := testBackend{f: func(n Notification) error { return nil }}
	tb2 := testBackend{f: func(n Notification) error { return nil }}
	mr := MultiRouter{Backends: []Backend{&tb, &tb2}}
	if err := mr.FanOut(Notification{}); err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	tb2.f = func(n Notification) error { return errors.New("error") }
	if err := mr.FanOut(Notification{}); err == nil {
		t.Fatalf("should have failed")
	}
}

func TestFakeSlackAPIClient(t *testing.T) {
	fc := FakeSlackAPIClient{}
	if _, _, err := fc.PostMessage("", "", slack.PostMessageParameters{}); err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	fc = FakeSlackAPIClient{
		PostFunc: func(channel, text string, params slack.PostMessageParameters) (string, string, error) {
			return "", "", nil
		},
	}
	if _, _, err := fc.PostMessage("", "", slack.PostMessageParameters{}); err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	fc = FakeSlackAPIClient{
		PostFunc: func(channel, text string, params slack.PostMessageParameters) (string, string, error) {
			return "", "", errors.New("error")
		},
	}
	if _, _, err := fc.PostMessage("", "", slack.PostMessageParameters{}); err == nil {
		t.Fatalf("should have failed")
	}
}

func TestNotificationEventKey(t *testing.T) {
	cases := []struct {
		name  string
		input NotificationEvent
	}{
		{
			name: "create", input: CreateEnvironment,
		},
		{
			name: "update", input: UpdateEnvironment,
		},
		{
			name: "destroy", input: DestroyEnvironment,
		},
		{
			name: "success", input: Success,
		},
		{
			name: "failure", input: Failure,
		},
		{
			name: "destroy", input: EnvironmentLimitExceeded,
		},
		{
			name: "<unknown>", input: 99999,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if key := c.input.Key(); key != c.name {
				t.Fatalf("bad key: %v", key)
			}
		})
	}
}
