package ghclient

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"

	"github.com/pkg/errors"
	billy "gopkg.in/src-d/go-billy.v4"
	git "gopkg.in/src-d/go-git.v4"
	gitplumb "gopkg.in/src-d/go-git.v4/plumbing"
	gitcache "gopkg.in/src-d/go-git.v4/plumbing/cache"
	gitfilemode "gopkg.in/src-d/go-git.v4/plumbing/filemode"
	gitobj "gopkg.in/src-d/go-git.v4/plumbing/object"
	gitstorer "gopkg.in/src-d/go-git.v4/plumbing/storer"
	gitfs "gopkg.in/src-d/go-git.v4/storage/filesystem"
)

type lockingRepo struct {
	sync.Mutex
	repo *git.Repository
}

type repos struct {
	sync.RWMutex
	// map of local path to opened local Repository
	pm map[string]*lockingRepo
}

// LocalWrapper is on object that satisfies RepoClient and which can optionally use local git repositories for
// repo names according to RepoPathMap, falling back to Backend if not found
type LocalWrapper struct {
	Backend RepoClient
	// FSFunc is a function that returns a filesystem given a path
	FSFunc func(path string) billy.Filesystem
	// RepoPathMap is a map of GitHub repo names (ex: "owner/repo") to absolute local filesystem path (which must be a valid git repository)
	RepoPathMap map[string]string
	// WorkingTreeRepos are the repos to read contents directly from disk (not via commit SHA)
	WorkingTreeRepos []string

	repoPathMapLock sync.RWMutex
	r               repos
}

var _ RepoClient = &LocalWrapper{}

func (lw *LocalWrapper) openRepo(path string) (*git.Repository, error) {
	if lw.FSFunc == nil {
		return nil, errors.New("FSFunc is nil")
	}
	// adapted copypasta from https://github.com/src-d/go-git/blob/v4.8.1/repository.go for opening a path with an arbitrary billy.Filesystem
	fs := lw.FSFunc(path)
	fi, err := fs.Stat(".git")
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%v: .git directory not found (check if this is a valid git repository)", path)
		}
		return nil, errors.Wrap(err, "error getting .git directory")
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("%v: not a directory", fi.Name())
	}
	dot, err := fs.Chroot(".git")
	if err != nil {
		return nil, errors.Wrap(err, "error in chroot to .git")
	}
	if _, err := dot.Stat(""); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%v: invalid git repository or repo does not exist", path)
		}
		return nil, errors.Wrap(err, "error getting stat info for .git")
	}
	s := gitfs.NewStorage(dot, gitcache.NewObjectLRUDefault())
	return git.Open(s, fs)
}

func (lw *LocalWrapper) getLocalRepo(path string) (*lockingRepo, error) {
	lw.r.RLock()
	lr, ok := lw.r.pm[path]
	lw.r.RUnlock()
	if ok {
		return lr, nil
	}
	r, err := lw.openRepo(path)
	if err != nil {
		return nil, errors.Wrap(err, "error opening local repo")
	}
	lr = &lockingRepo{repo: r}
	lw.r.Lock()
	if lw.r.pm == nil {
		lw.r.pm = map[string]*lockingRepo{}
	}
	lw.r.pm[path] = lr
	lw.r.Unlock()
	return lr, nil
}

// getHashForRef returns the hash for a given ref (branch/tag/sha)
// the caller must already hold the lock for lr
func (lw *LocalWrapper) getHashForRef(lr *lockingRepo, refname string) (gitplumb.Hash, error) {
	var refName func(string) gitplumb.ReferenceName
	findRef := func(iter gitstorer.ReferenceIter, err error) (bool, gitplumb.Hash, error) {
		var found bool
		out := gitplumb.Hash([20]byte{})
		if err != nil {
			return found, out, errors.Wrap(err, "error iterating")
		}
		if err := iter.ForEach(func(ref *gitplumb.Reference) error {
			if ref.Name() == refName(refname) {
				found = true
				out = ref.Hash()
			}
			return nil
		}); err != nil {
			return found, out, errors.Wrap(err, "error processing iterator")
		}
		return found, out, nil
	}
	refName = gitplumb.NewBranchReferenceName
	found, hash, err := findRef(lr.repo.Branches())
	if err != nil {
		return hash, errors.Wrap(err, "error iterating branches")
	}
	if found {
		return hash, nil
	}
	refName = gitplumb.NewTagReferenceName
	found, hash, err = findRef(lr.repo.Tags())
	if err != nil {
		return hash, errors.Wrap(err, "error iterating tags")
	}
	if found {
		return hash, nil
	}
	c, err := lr.repo.CommitObject(gitplumb.NewHash(refname))
	if err != nil {
		return hash, errors.Wrap(err, "error getting commit")
	}
	return c.Hash, nil
}

var ErrBranchNotFound = errors.New("branch not found")
var ErrRefNotFound = errors.New("reference not found")

func (lw *LocalWrapper) localGetBranch(path, branch string) (BranchInfo, error) {
	lr, err := lw.getLocalRepo(path)
	if err != nil {
		return BranchInfo{}, errors.Wrap(err, "error getting local repo")
	}
	lr.Lock()
	defer lr.Unlock()
	iter, err := lr.repo.Branches()
	if err != nil {
		return BranchInfo{}, errors.Wrap(err, "error iterating branches")
	}
	out := BranchInfo{}
	if err := iter.ForEach(func(ref *gitplumb.Reference) error {
		if ref.Name() == gitplumb.NewBranchReferenceName(branch) {
			out.Name = branch
			out.SHA = ref.Hash().String()
		}
		return nil
	}); err != nil {
		return BranchInfo{}, errors.Wrap(err, "error processing branches")
	}
	if out.Name == "" {
		return BranchInfo{}, ErrBranchNotFound
	}
	return out, nil
}

func (lw *LocalWrapper) localGetBranches(path string) ([]BranchInfo, error) {
	lr, err := lw.getLocalRepo(path)
	if err != nil {
		return nil, errors.Wrap(err, "error getting local repo")
	}
	lr.Lock()
	defer lr.Unlock()
	iter, err := lr.repo.Branches()
	if err != nil {
		return nil, errors.Wrap(err, "error iterating branches")
	}
	out := []BranchInfo{}
	if err := iter.ForEach(func(ref *gitplumb.Reference) error {
		out = append(out, BranchInfo{Name: ref.Name().Short(), SHA: ref.Hash().String()})
		return nil
	}); err != nil {
		return nil, errors.Wrap(err, "error processing branches")
	}
	return out, nil
}

func (lw *LocalWrapper) localGetCommitMessage(path, sha string) (string, error) {
	lr, err := lw.getLocalRepo(path)
	if err != nil {
		return "", errors.Wrap(err, "error getting local repo")
	}
	lr.Lock()
	defer lr.Unlock()
	c, err := lr.repo.CommitObject(gitplumb.NewHash(sha))
	if err != nil {
		return "", errors.Wrap(err, "error getting commit object")
	}
	return c.Message, nil
}

func (lw *LocalWrapper) isWorkingTreeRepo(repo string) bool {
	for _, wtr := range lw.WorkingTreeRepos {
		if wtr == repo {
			return true
		}
	}
	return false
}

func (lw *LocalWrapper) localGetFileContents(repopath, repo, filepath, ref string) ([]byte, error) {
	lr, err := lw.getLocalRepo(repopath)
	if err != nil {
		return nil, errors.Wrap(err, "error getting local repo")
	}
	if lw.isWorkingTreeRepo(repo) {
		// if it's the triggering repo, read the file that's on disk (the working copy) to pick up any changes that
		// haven't been committed to the repo yet
		fs := lw.FSFunc(repopath)
		f, err := fs.Open(filepath)
		if err != nil {
			return nil, errors.Wrap(err, "error opening file")
		}
		defer f.Close()
		return ioutil.ReadAll(f)
	}
	lr.Lock()
	defer lr.Unlock()
	h, err := lw.getHashForRef(lr, ref)
	if err != nil {
		return nil, errors.Wrap(err, "error getting ref")
	}
	c, err := lr.repo.CommitObject(h)
	if err != nil {
		return nil, errors.Wrap(err, "error getting commit for ref")
	}
	tree, err := lr.repo.TreeObject(c.TreeHash)
	if err != nil {
		return nil, errors.Wrap(err, "error getting tree for ref")
	}
	f, err := tree.File(filepath)
	if err != nil {
		return nil, errors.Wrap(err, "error getting file object")
	}
	rc, err := f.Reader()
	if err != nil {
		return nil, errors.Wrap(err, "error getting reader")
	}
	defer rc.Close()
	contents, err := ioutil.ReadAll(rc)
	if err != nil {
		return nil, errors.Wrap(err, "error reading file")
	}
	return contents, nil
}

func (lw *LocalWrapper) localGetDirectoryContents(repopath, repo, dirpath, ref string) (map[string]FileContents, error) {
	lr, err := lw.getLocalRepo(repopath)
	if err != nil {
		return nil, errors.Wrap(err, "error getting local repo")
	}
	out := map[string]FileContents{}
	if lw.isWorkingTreeRepo(repo) {
		// if it's the triggering repo, read files that are on disk (the working copy) to pick up any changes that
		// haven't been committed to the repo yet
		fs := lw.FSFunc("")
		wf := func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			outpath := strings.Replace(path, fs.Join(repopath, dirpath)+"/", "", 1)
			if info.Mode()&os.ModeSymlink != 0 {
				target, err := os.Readlink(path)
				if err != nil {
					return errors.Wrap(err, "error reading symlink")
				}
				out[outpath] = FileContents{
					Path:          path,
					Symlink:       true,
					SymlinkTarget: target,
				}
				return nil
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			f, err := fs.Open(path)
			if err != nil {
				return errors.Wrap(err, "error opening file")
			}
			defer f.Close()
			contents, err := ioutil.ReadAll(f)
			if err != nil {
				return errors.Wrap(err, "error reading triggering repo file")
			}
			out[outpath] = FileContents{
				Path:     path,
				Contents: contents,
			}
			return nil
		}
		if err := billyfsWalk(fs, fs.Join(repopath, dirpath), wf); err != nil {
			return nil, errors.Wrap(err, "error getting local directory contents")
		}
		return out, nil
	}
	lr.Lock()
	defer lr.Unlock()
	h, err := lw.getHashForRef(lr, ref)
	if err != nil {
		return nil, errors.Wrap(err, "error getting ref")
	}
	c, err := lr.repo.CommitObject(h)
	if err != nil {
		return nil, errors.Wrap(err, "error getting commit for ref")
	}
	tree, err := lr.repo.TreeObject(c.TreeHash)
	if err != nil {
		return nil, errors.Wrap(err, "error getting tree for ref")
	}
	if dirpath != "" && dirpath != "." {
		tree, err = tree.Tree(dirpath)
		if err != nil {
			return nil, errors.Wrap(err, "error getting subtree at path")
		}
	}
	if err := tree.Files().ForEach(func(f *gitobj.File) error {
		switch f.Mode {
		case gitfilemode.Dir:
			// ignore bare directories
			break
		case gitfilemode.Deprecated:
			fallthrough
		case gitfilemode.Executable:
			fallthrough
		case gitfilemode.Regular:
			rc, err := f.Reader()
			if err != nil {
				return errors.Wrap(err, "error getting reader")
			}
			defer rc.Close()
			contents, err := ioutil.ReadAll(rc)
			if err != nil {
				return errors.Wrap(err, "error reading file")
			}
			out[f.Name] = FileContents{
				Path:     f.Name,
				Contents: contents,
			}
		case gitfilemode.Symlink:
			rc, err := f.Reader()
			if err != nil {
				return errors.Wrap(err, "error getting reader for symlink")
			}
			defer rc.Close()
			target, err := ioutil.ReadAll(rc)
			if err != nil {
				return errors.Wrap(err, "error reading symlink")
			}
			out[f.Name] = FileContents{
				Path:          f.Name,
				Symlink:       true,
				SymlinkTarget: string(target),
			}
		default:
			return fmt.Errorf("unsupported object file mode: %v", f.Mode)
		}
		return nil
	}); err != nil {
		return nil, errors.Wrap(err, "error iterating over files")
	}
	return out, nil
}

func (lw *LocalWrapper) GetBranch(ctx context.Context, repo, branch string) (BranchInfo, error) {
	lw.repoPathMapLock.RLock()
	p, ok := lw.RepoPathMap[repo]
	lw.repoPathMapLock.RUnlock()
	if ok {
		return lw.localGetBranch(p, branch)
	}
	return lw.Backend.GetBranch(ctx, repo, branch)
}

func (lw *LocalWrapper) GetBranches(ctx context.Context, repo string) ([]BranchInfo, error) {
	lw.repoPathMapLock.RLock()
	p, ok := lw.RepoPathMap[repo]
	lw.repoPathMapLock.RUnlock()
	if ok {
		return lw.localGetBranches(p)
	}
	return lw.Backend.GetBranches(ctx, repo)
}

func (lw *LocalWrapper) GetCommitMessage(ctx context.Context, repo, sha string) (string, error) {
	lw.repoPathMapLock.RLock()
	p, ok := lw.RepoPathMap[repo]
	lw.repoPathMapLock.RUnlock()
	if ok {
		return lw.localGetCommitMessage(p, sha)
	}
	return lw.Backend.GetCommitMessage(ctx, repo, sha)
}

func (lw *LocalWrapper) GetFileContents(ctx context.Context, repo string, path string, ref string) ([]byte, error) {
	lw.repoPathMapLock.RLock()
	p, ok := lw.RepoPathMap[repo]
	lw.repoPathMapLock.RUnlock()
	if ok {
		return lw.localGetFileContents(p, repo, path, ref)
	}
	return lw.Backend.GetFileContents(ctx, repo, path, ref)
}

func (lw *LocalWrapper) GetDirectoryContents(ctx context.Context, repo, path, ref string) (map[string]FileContents, error) {
	lw.repoPathMapLock.RLock()
	p, ok := lw.RepoPathMap[repo]
	lw.repoPathMapLock.RUnlock()
	if ok {
		return lw.localGetDirectoryContents(p, repo, path, ref)
	}
	return lw.Backend.GetDirectoryContents(ctx, repo, path, ref)
}

// stubs to satisfy the interface
func (lw *LocalWrapper) GetTags(context.Context, string) ([]BranchInfo, error)          { return nil, nil }
func (lw *LocalWrapper) SetStatus(context.Context, string, string, *CommitStatus) error { return nil }
func (lw *LocalWrapper) GetPRStatus(context.Context, string, uint) (string, error)      { return "", nil }
