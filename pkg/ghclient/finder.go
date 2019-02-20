package ghclient

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/afero"
	desfacer "gopkg.in/jfontan/go-billy-desfacer.v0"
	git "gopkg.in/src-d/go-git.v4"
	gitcache "gopkg.in/src-d/go-git.v4/plumbing/cache"
	gitfs "gopkg.in/src-d/go-git.v4/storage/filesystem"
)

// RepoFinder is an object that searches the local filesystem for git repositories configured for use with GitHub
type RepoFinder struct {
	// GitHubHostname is the GitHub hostname to use when finding GitHub repos
	// (some people use fake GitHub hostnames to toggle SSH keys ("me.github.com") via ~/.ssh/config)
	GitHubHostname string
	FSFunc         func(path string) afero.Fs
	LF             func(string, ...interface{})
}

func (rf *RepoFinder) log(msg string, args ...interface{}) {
	if rf.LF != nil {
		rf.LF(msg, args...)
	}
}

func openRepo(fs afero.Fs, path string) (*git.Repository, error) {
	dotgitpath := filepath.Join(path, ".git")
	fi, err := fs.Stat(dotgitpath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%v: .git directory not found (check if this is a valid git repository)", path)
		}
		return nil, errors.Wrap(err, "error getting .git directory")
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("%v: not a directory", fi.Name())
	}
	dot, err := desfacer.New(fs).Chroot(dotgitpath)
	if err != nil {
		return nil, errors.Wrap(err, "error in chroot to .git")
	}
	if _, err := dot.Stat(""); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%v: invalid git repository or repo does not exist", path)
		}
		return nil, errors.Wrap(err, "error getting stat info for .git")
	}
	return git.Open(gitfs.NewStorage(dot, gitcache.NewObjectLRUDefault()), desfacer.New(fs))
}

// Find recursively searches searchPaths and finds any valid git repositories with remotes configured for GitHub.
// It returns a map of GitHub repo name ("owner/repo") to absolute local filesystem path, or error.
func (rf *RepoFinder) Find(searchPaths []string) (map[string]string, error) {
	if rf.FSFunc == nil {
		return nil, errors.New("FSFunc is nil")
	}
	repos := map[string]string{}
	fs := rf.FSFunc("")
	wf := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if filepath.Base(path) == "lost+found" {
			// Linux special case, don't descend into lost+found
			return filepath.SkipDir
		}
		if !info.IsDir() {
			return nil
		}
		fi, err := fs.Stat(filepath.Join(path, ".git"))
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return errors.Wrap(err, "error getting .git info")
		}
		if !fi.IsDir() {
			return nil
		}
		// path is a directory and contains a ".git" directory
		// verify that it's a valid repo and check the remotes
		repo, err := openRepo(fs, path)
		if err != nil {
			rf.log("%v: has .git but isn't a valid repo: %v", path, err)
			return filepath.SkipDir
		}
		cfg, err := repo.Config()
		if err != nil {
			rf.log("%v: error getting repo config: %v", path, err)
			return filepath.SkipDir
		}
		origin, ok := cfg.Remotes["origin"]
		if !ok {
			return filepath.SkipDir
		}
		// check if at least one origin URL matches GitHub
		reponame, ok := getGithubName(origin.URLs, rf.GitHubHostname)
		if ok {
			repos[reponame] = path
			rf.log("%v: GitHub repo %v found", path, reponame)
			return filepath.SkipDir
		}
		return filepath.SkipDir
	}
	for _, sp := range searchPaths {
		if err := afero.Walk(fs, sp, wf); err != nil {
			return nil, errors.Wrapf(err, "error searching path: %v", sp)
		}
	}
	return repos, nil
}

func getGithubName(originURLs []string, gitHubHostname string) (string, bool) {
	getRepoName := func(url, pfx string) string {
		return strings.Replace(url[0:len(url)-len(".git")], pfx, "", 1)
	}
	for _, url := range originURLs {
		if strings.HasPrefix(url, "https://github.com/") {
			if strings.HasSuffix(url, ".git") {
				return getRepoName(url, "https://github.com/"), true
			}
		}
		if strings.HasPrefix(url, "git@"+gitHubHostname) {
			if strings.HasSuffix(url, ".git") {
				return getRepoName(url, "git@"+gitHubHostname+":"), true
			}
		}
	}
	return "", false
}

// LocalRepoInfo models information about a local git repo
type LocalRepoInfo struct {
	GitHubRepoName, HeadBranch, HeadSHA string
}

// RepoInfo gets LocalRepoInfo for the git repo at path, which must be a valid repo with GitHub remotes.
func RepoInfo(fs afero.Fs, path, gitHubHostname string) (LocalRepoInfo, error) {
	out := LocalRepoInfo{}
	repo, err := openRepo(fs, path)
	if err != nil {
		return out, errors.Wrap(err, "error opening repo")
	}
	head, err := repo.Head()
	if err != nil {
		return out, errors.Wrap(err, "error getting HEAD for repo")
	}
	if !head.Name().IsBranch() {
		return out, fmt.Errorf("HEAD is not a branch: %v", head.Name().String())
	}
	out.HeadBranch = head.Name().Short()
	out.HeadSHA = head.Hash().String()
	cfg, err := repo.Config()
	if err != nil {
		return out, errors.Wrap(err, "error getting repo config")
	}
	origin, ok := cfg.Remotes["origin"]
	if !ok {
		return out, errors.New("repo doesn't have an origin remote")
	}
	// check if at least one origin URL matches GitHub
	reponame, ok := getGithubName(origin.URLs, gitHubHostname)
	if !ok {
		return out, errors.New("repo doesn't have GitHub remotes configured (check that --github-hostname is set correctly)")
	}
	out.GitHubRepoName = reponame
	return out, nil
}
