package ghapp

import (
	"net/http"

	"github.com/dollarshaveclub/acyl/pkg/persistence"
	"github.com/dollarshaveclub/acyl/pkg/spawner"

	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
)

// LogFunc logs a string somewhere
type LogFunc func(string, ...interface{})

// GitHubApp implements a GitHub app
type GitHubApp struct {
	cfg githubapp.Config
	cc  githubapp.ClientCreator
	prh *prEventHandler
	es  spawner.EnvironmentSpawner
	dl  persistence.DataLayer
}

// NewGitHubApp returns a GitHubApp with the given private key, app ID and webhook secret, or error
func NewGitHubApp(privateKeyPEM []byte, appID uint, webhookSecret string, es spawner.EnvironmentSpawner, dl persistence.DataLayer) (*GitHubApp, error) {
	if len(privateKeyPEM) == 0 {
		return nil, errors.New("invalid private key")
	}
	if appID == 0 {
		return nil, errors.New("invalid app ID")
	}
	if len(webhookSecret) == 0 {
		return nil, errors.New("invalid webhook secret")
	}
	if es == nil || dl == nil {
		return nil, errors.New("DataLayer and EnvironmentSpawner are required")
	}
	c := githubapp.Config{}
	c.App.IntegrationID = int(appID)
	c.App.PrivateKey = string(privateKeyPEM)
	c.App.WebhookSecret = webhookSecret
	cc, err := githubapp.NewDefaultCachingClientCreator(c)
	if err != nil {
		return nil, errors.Wrap(err, "error initializing github app default client")
	}
	return &GitHubApp{
		cfg: c,
		cc:  cc,
		es:  es,
		dl:  dl,
	}, nil
}

// Handler returns the http.Handler that should handle the webhook HTTP endpoint
func (gha *GitHubApp) Handler() http.Handler {
	gha.prh = &prEventHandler{ClientCreator: gha.cc, es: gha.es, dl: gha.dl}
	return githubapp.NewDefaultEventDispatcher(gha.cfg, gha.prh, &checksEventHandler{gha.cc})
}

// Wait blocks until all async actions created by app handlers have completed
func (gha *GitHubApp) Wait() {
	gha.prh.wg.Wait()
}
