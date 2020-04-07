package ghapp

import (
	"context"

	"github.com/shurcooL/githubv4"

	"github.com/google/go-github/github"
)

const (
	ghClientContextKey               = "ghapp_github_client"
	ghClientInstallationIDContextKey = "ghapp_installation_id"
)

type GitHubClientContextKey string
type GitHubInstallationIDContextKey string

type GithubAppClientFactory interface {
	NewAppClient() (*github.Client, error)
	NewAppV4Client() (*githubv4.Client, error)
	NewInstallationClient(installationID int64) (*github.Client, error)
	NewInstallationV4Client(installationID int64) (*githubv4.Client, error)
}

// NewGitHubClientContext returns a context with the GitHub client factory from gha embedded as a value
func NewGitHubClientContext(ctx context.Context, installationID int64, gha GithubAppClientFactory) context.Context {
	ctx = context.WithValue(ctx, GitHubInstallationIDContextKey(ghClientInstallationIDContextKey), installationID)
	return context.WithValue(ctx, GitHubClientContextKey(ghClientContextKey), gha)
}

// GetGitHubAppClient returns the GitHub app client embedded in ctx if present or the alternate client provided by caller
func GetGitHubAppClient(ctx context.Context, alt *github.Client) *github.Client {
	ghcf, ok := ctx.Value(GitHubClientContextKey(ghClientContextKey)).(GithubAppClientFactory)
	if ok && ghcf != nil {
		ghc, err := ghcf.NewAppClient()
		if err != nil {
			return alt
		}
		return ghc
	}
	return alt
}

// GetGitHubInstallationClient returns the GitHub installation client embedded in ctx if present or the alternate client provided by caller
func GetGitHubInstallationClient(ctx context.Context, alt *github.Client) *github.Client {
	ghcf, ok := ctx.Value(GitHubClientContextKey(ghClientContextKey)).(GithubAppClientFactory)
	if ok && ghcf != nil {
		iid, ok := ctx.Value(GitHubInstallationIDContextKey(ghClientInstallationIDContextKey)).(int64)
		if !ok {
			return alt
		}
		ghc, err := ghcf.NewInstallationClient(iid)
		if err != nil {
			return alt
		}
		return ghc
	}
	return alt
}
