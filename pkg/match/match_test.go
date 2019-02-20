package match

import (
	"strings"
	"testing"
)

func TestGetRefForRepo(t *testing.T) {
	cases := []struct {
		name                    string
		inputRI                 RepoInfo
		inputBranches           []BranchInfo
		outputSHA, outputBranch string
		isError                 bool
		errContains             string
	}{
		{"matching", RepoInfo{SourceBranch: "feature-foo", BaseBranch: "master", BranchMatch: true}, []BranchInfo{BranchInfo{Name: "feature-foo", SHA: "asdf"}, BranchInfo{Name: "master"}}, "asdf", "feature-foo", false, ""},
		{"fallback to base branch", RepoInfo{SourceBranch: "feature-foo", BaseBranch: "master", BranchMatch: true}, []BranchInfo{BranchInfo{Name: "master", SHA: "asdf"}}, "asdf", "master", false, ""},
		{"fallback to default branch", RepoInfo{SourceBranch: "feature-foo", BaseBranch: "master", BranchMatch: true, DefaultBranch: "release"}, []BranchInfo{BranchInfo{Name: "release", SHA: "asdf"}}, "asdf", "release", false, ""},
		{"no branch matching with default branch", RepoInfo{SourceBranch: "feature-foo", BaseBranch: "master", BranchMatch: false, DefaultBranch: "release"}, []BranchInfo{BranchInfo{Name: "release", SHA: "asdf"}}, "asdf", "release", false, ""},
		{"no branch matching without default branch", RepoInfo{SourceBranch: "feature-foo", BaseBranch: "master", BranchMatch: false, DefaultBranch: ""}, []BranchInfo{BranchInfo{Name: "master", SHA: "asdf"}}, "asdf", "master", false, ""},
		{"no branch matching with missing default branch", RepoInfo{SourceBranch: "feature-foo", BaseBranch: "master", BranchMatch: false, DefaultBranch: "release"}, []BranchInfo{BranchInfo{Name: "master", SHA: "asdf"}}, "", "", true, "branch matching is disabled but repo is missing"},
		{"no suitable branch", RepoInfo{SourceBranch: "feature-foo", BaseBranch: "release", BranchMatch: true}, []BranchInfo{BranchInfo{Name: "master", SHA: "asdf"}}, "", "", true, "no suitable branch"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sha, branch, err := GetRefForRepo(c.inputRI, c.inputBranches)
			if err != nil {
				if !c.isError {
					t.Fatalf("should have succeeded: %v", err)
				}
				if !strings.Contains(err.Error(), c.errContains) {
					t.Fatalf("error missing string (%v): %v", c.errContains, err)
				}
				return
			}
			if sha != c.outputSHA {
				t.Fatalf("bad SHA (expected %v): %v", c.outputSHA, sha)
			}
			if branch != c.outputBranch {
				t.Fatalf("bad branch (expected %v): %v", c.outputBranch, branch)
			}
		})
	}
}
