package context

import (
	stdlibctx "context"

	"github.com/google/go-github/github"
	"github.com/pkg/errors"
)

const (
	ghClientContextKey = "ghapp_github_client"
)

type GitHubClientContextKey string

type githubAppClientFactory interface {
	NewAppClient() (*github.Client, error)
}

// NewGitHubClientContext returns a context with the GitHub client from gha embedded as a value
func NewGitHubClientContext(ctx stdlibctx.Context, gha githubAppClientFactory) (stdlibctx.Context, error) {
	ac, err := gha.NewAppClient()
	if err != nil {
		return ctx, errors.Wrap(err, "error getting app client")
	}
	return stdlibctx.WithValue(ctx, GitHubClientContextKey(ghClientContextKey), ac), nil
}

// GetGitHubClient returns the GitHub client embedded in ctx if present or the alternate client provided by caller
func GetGitHubClient(ctx stdlibctx.Context, alt *github.Client) *github.Client {
	ghc, ok := ctx.Value(GitHubClientContextKey(ghClientContextKey)).(*github.Client)
	if ok && ghc != nil {
		return ghc
	}
	return alt
}
