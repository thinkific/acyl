package ghapp

import (
	"net/http"
	"sync"

	"github.com/dollarshaveclub/acyl/pkg/ghclient"

	"github.com/dollarshaveclub/acyl/pkg/persistence"
	"github.com/dollarshaveclub/acyl/pkg/spawner"

	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
)

// LogFunc logs a string somewhere
type LogFunc func(string, ...interface{})

// GitHubApp implements a GitHub app
type GitHubApp struct {
	typepath string
	cfg      githubapp.Config
	cc       githubapp.ClientCreator
	prh      *prEventHandler
	rc       ghclient.RepoClient
	es       spawner.EnvironmentSpawner
	dl       persistence.DataLayer
}

// NewGitHubApp returns a GitHubApp with the given private key, app ID and webhook secret, or error
func NewGitHubApp(privateKeyPEM []byte, appID uint, webhookSecret string, typePath string, rc ghclient.RepoClient, es spawner.EnvironmentSpawner, dl persistence.DataLayer) (*GitHubApp, error) {
	if len(privateKeyPEM) == 0 {
		return nil, errors.New("invalid private key")
	}
	if appID == 0 {
		return nil, errors.New("invalid app ID")
	}
	if len(webhookSecret) == 0 {
		return nil, errors.New("invalid webhook secret")
	}
	if typePath == "" {
		return nil, errors.New("invalid type path")
	}
	if es == nil || dl == nil || rc == nil {
		return nil, errors.New("RepoClient, DataLayer and EnvironmentSpawner are required")
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
		typepath: typePath,
		cfg:      c,
		cc:       cc,
		rc:       rc,
		es:       es,
		dl:       dl,
	}, nil
}

// Handler returns the http.Handler that should handle the webhook HTTP endpoint
// Pass in a pointer to the global wait group that will be used by goroutines started by this handler
func (gha *GitHubApp) Handler(wg *sync.WaitGroup) http.Handler {
	if wg == nil {
		wg = &sync.WaitGroup{}
	}
	gha.prh = &prEventHandler{ClientCreator: gha.cc, typePath: gha.typepath, wg: wg, rc: gha.rc, es: gha.es, dl: gha.dl}
	return githubapp.NewDefaultEventDispatcher(gha.cfg, gha.prh, &checksEventHandler{gha.cc})
}
