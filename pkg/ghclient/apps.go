package ghclient

import (
	"context"
	"fmt"

	"github.com/google/go-github/github"
)

// GitHubAppInstallationClient describes a GitHub client that returns user-scoped metadata regarding an app installation
type GitHubAppInstallationClient interface {
	GetUserAppInstallations(ctx context.Context) (AppInstallations, error)
	GetUserAppRepos(ctx context.Context, appID int64) ([]string, error)
	GetUser(ctx context.Context) (string, error)
}

type AppInstallation struct {
	ID int64
}

type AppInstallations []AppInstallation

func (ai AppInstallations) IDPresent(id int64) bool {
	for _, inst := range ai {
		if inst.ID == id {
			return true
		}
	}
	return false
}

func appInstallationsFromGitHubInstallations(in []*github.Installation) AppInstallations {
	out := make(AppInstallations, len(in))
	for i, inst := range in {
		if inst != nil {
			if inst.ID != nil {
				out[i].ID = *inst.ID
			}
		}
	}
	return out
}

// GetUserAppInstallationCount returns the number of app installations that are accessible to the authenticated user
// This method only uses the static token associated with the GitHubClient and not anything present in the context
// GitHubClient should be populated with the user token returned by the oauth login endpoint via the oauth callback handler
func (ghc *GitHubClient) GetUserAppInstallations(ctx context.Context) (AppInstallations, error) {
	lopt := &github.ListOptions{PerPage: 100}
	out := []*github.Installation{}
	for {
		ctx, cf := context.WithTimeout(ctx, ghTimeout)
		defer cf()
		insts, resp, err := ghc.c.Apps.ListUserInstallations(ctx, lopt)
		if err != nil {
			return nil, fmt.Errorf("error listing user installations: %v", err)
		}
		out = append(out, insts...)
		if resp.NextPage == 0 {
			return appInstallationsFromGitHubInstallations(out), nil
		}
		lopt.Page = resp.NextPage
	}
}

// GetUserAppRepos gets repositories that are accessible to the authenticated user for an app installation
// This method only uses the static token associated with the GitHubClient and not anything present in the context
// GitHubClient should be populated with the user token returned by the oauth login endpoint via the oauth callback handler
func (ghc *GitHubClient) GetUserAppRepos(ctx context.Context, instID int64) ([]string, error) {
	lopt := &github.ListOptions{PerPage: 100}
	out := []string{}
	for {
		ctx, cf := context.WithTimeout(ctx, ghTimeout)
		defer cf()
		repos, resp, err := ghc.c.Apps.ListUserRepos(ctx, instID, lopt)
		if err != nil {
			return nil, fmt.Errorf("error listing user repos: %v", err)
		}
		for _, repo := range repos {
			if repo != nil && repo.FullName != nil {
				out = append(out, *repo.FullName)
			}
		}
		if resp.NextPage == 0 {
			return out, nil
		}
		lopt.Page = resp.NextPage
	}
}

// GetUser gets the authenticated user login name
// This method only uses the static token associated with the GitHubClient and not anything present in the context
// GitHubClient should be populated with the user token returned by the oauth login endpoint via the oauth callback handler
func (ghc *GitHubClient) GetUser(ctx context.Context) (string, error) {
	ctx, cf := context.WithTimeout(ctx, ghTimeout)
	defer cf()
	user, _, err := ghc.c.Users.Get(ctx, "")
	if err != nil {
		return "", fmt.Errorf("error getting current authenticated user: %v", err)
	}
	return user.GetLogin(), nil
}
