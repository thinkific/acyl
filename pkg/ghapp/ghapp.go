package ghapp

import (
	"context"
	"net/http"

	"github.com/dollarshaveclub/acyl/pkg/models"

	"github.com/dollarshaveclub/acyl/pkg/persistence"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
)

// LogFunc logs a string somewhere
type LogFunc func(string, ...interface{})

// PRCallback is a function that gets called when a validated, parsed PR webhook event is received
// action is the PR webhook action string ("opened", "closed", "labeled", etc), see https://developer.github.com/v3/activity/events/types/#pullrequestevent
// rrd is the parsed repo/revision information from the webhook payload
// ctx is pre-populated with an eventlogger and authenticated GitHub clients (app and installation)
// If the callback returns a non-nil error, the webhook request client will be returned a 500 error with the error details in the body
// If the error is nil, the webhook client will be returned a 202 Accepted response with the eventlog ID
type PRCallback func(ctx context.Context, action string, rrd models.RepoRevisionData) error

// GitHubApp implements a GitHub app
type GitHubApp struct {
	cfg githubapp.Config
	prh *prEventHandler
	ch  *checksEventHandler
}

var (
	GitHubV3APIURL = "https://api.github.com/"
	GitHubV4APIURL = "https://api.github.com/graphql"
)

// NewGitHubApp returns a GitHubApp with the given private key, app ID and webhook secret, or error
// supportedPRActions is at least one PR webhook action that is supported (prcallback will be executed).
// Any unsupported actions will be ignored and the webhook request will be responded with 200 OK, "action not relevant".
func NewGitHubApp(privateKeyPEM []byte, appID uint, webhookSecret string, supportedPRActions []string, prcb PRCallback, dl persistence.DataLayer) (*GitHubApp, error) {
	if len(privateKeyPEM) == 0 {
		return nil, errors.New("invalid private key")
	}
	if appID == 0 {
		return nil, errors.New("invalid app ID")
	}
	if len(webhookSecret) == 0 {
		return nil, errors.New("invalid webhook secret")
	}
	if dl == nil {
		return nil, errors.New("DataLayer is required")
	}
	if prcb == nil {
		return nil, errors.New("PR callback is required")
	}
	if len(supportedPRActions) == 0 {
		return nil, errors.New("at least one supported PR action is required")
	}
	sa := make(map[string]struct{}, len(supportedPRActions))
	for _, a := range supportedPRActions {
		sa[a] = struct{}{}
	}
	c := githubapp.Config{}
	c.V3APIURL = GitHubV3APIURL
	c.V4APIURL = GitHubV4APIURL
	c.App.IntegrationID = int(appID)
	c.App.PrivateKey = string(privateKeyPEM)
	c.App.WebhookSecret = webhookSecret
	cc, err := githubapp.NewDefaultCachingClientCreator(c)
	if err != nil {
		return nil, errors.Wrap(err, "error initializing github app default client")
	}
	return &GitHubApp{
		prh: &prEventHandler{
			ClientCreator:      cc,
			installationID:     int64(appID),
			dl:                 dl,
			supportedPRActions: sa,
			RRDCallback:        prcb,
		},
		ch: &checksEventHandler{
			ClientCreator: cc,
		},
		cfg: c,
	}, nil
}

// Handler returns the http.Handler that should handle the webhook HTTP endpoint
func (gha *GitHubApp) Handler() http.Handler {
	return githubapp.NewDefaultEventDispatcher(gha.cfg, gha.prh, gha.ch)
}
