package notifier

import (
	"github.com/nlopes/slack"
)

type FakeSlackAPIClient struct {
	PostFunc func(channel, text string, params slack.PostMessageParameters) (string, string, error)
}

func (fs *FakeSlackAPIClient) PostMessage(channel, text string, params slack.PostMessageParameters) (string, string, error) {
	if fs.PostFunc != nil {
		return fs.PostFunc(channel, text, params)
	}
	return "", "", nil
}
