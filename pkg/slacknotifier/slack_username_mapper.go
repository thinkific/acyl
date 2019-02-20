package slacknotifier

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// SlackUsernameMapper maps Github usernames to Slack usernames. If no Slack username is found, an empty string is returned
type SlackUsernameMapper interface {
	UsernameFromGithubUsername(githubUsername string) (string, error)
}

// RepoBackedSlackUsernameMapper concrete implementation of SlackUsernameMapper backed by a git repository
type RepoBackedSlackUsernameMapper struct {
	client            RepoClient
	repo              string
	mapPath           string
	ref               string
	mapMutex          sync.Mutex
	cachedUsernameMap map[string]string
	lastUpdate        time.Time
	updateInterval    time.Duration
}

// NewRepoBackedSlackUsernameMapper returns an initialized RepoBackedSlackUsernameMapper with the given repo client
func NewRepoBackedSlackUsernameMapper(client RepoClient, repo string, mapPath string, ref string, updateInterval time.Duration) *RepoBackedSlackUsernameMapper {
	return &RepoBackedSlackUsernameMapper{client: client, repo: repo, mapPath: mapPath, updateInterval: updateInterval}
}

// UsernameFromGithubUsername returns the Slack username for the provided GitHub username or nil
func (m *RepoBackedSlackUsernameMapper) UsernameFromGithubUsername(githubUsername string) (string, error) {
	unMap, err := m.currentUsernameMap()
	if err != nil {
		return "", err
	}

	un, ok := unMap[githubUsername]
	if ok == false {
		return "", nil
	}

	return un, nil
}

func (m *RepoBackedSlackUsernameMapper) currentUsernameMap() (map[string]string, error) {
	// Wrapping this entire method in a mutex will cause a good deal of lock contention but I think that's OK.
	// The simplicity of the implementation is worth the performance impact since this code path isn't that active.
	// We should revisit this if it becomes a performance problem.
	m.mapMutex.Lock()
	defer m.mapMutex.Unlock()
	if m.cachedUsernameMap == nil || m.lastUpdate.Add(m.updateInterval).Before(time.Now()) {

		unMap, err := m.fetchUsernameMap()
		if err != nil {
			return nil, err
		}

		m.cachedUsernameMap = unMap
		m.lastUpdate = time.Now()
	}

	return m.cachedUsernameMap, nil
}

func (m *RepoBackedSlackUsernameMapper) fetchUsernameMap() (map[string]string, error) {
	b, err := m.client.GetFileContents(context.Background(), m.repo, m.mapPath, m.ref)
	if err != nil {
		return nil, err
	}

	var am map[string]string
	err = json.Unmarshal(b, &am)

	return am, err
}
