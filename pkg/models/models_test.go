package models

import (
	"strings"
	"testing"
)

func TestDependencyDeclarationValidateNames(t *testing.T) {
	cases := []struct {
		name          string
		input         DependencyDeclaration
		isOK, isError bool
		errContains   string
	}{
		{
			"valid",
			DependencyDeclaration{
				Direct: []RepoConfigDependency{
					RepoConfigDependency{
						Name:     "foo",
						Requires: []string{"bar"},
					},
					RepoConfigDependency{
						Name: "bar",
					},
				},
				Environment: []RepoConfigDependency{
					RepoConfigDependency{
						Name:     "baz",
						Requires: []string{"bar"},
					},
					RepoConfigDependency{
						Name: "asdf",
					},
				},
			},
			true, false, "",
		},
		{
			"empty name",
			DependencyDeclaration{
				Direct: []RepoConfigDependency{
					RepoConfigDependency{
						Name:     "",
						Requires: []string{"bar"},
					},
					RepoConfigDependency{
						Name: "bar",
					},
				},
				Environment: []RepoConfigDependency{
					RepoConfigDependency{
						Name:     "baz",
						Requires: []string{"bar"},
					},
					RepoConfigDependency{
						Name: "asdf",
					},
				},
			},
			false, true, "empty name",
		},
		{
			"unknown direct",
			DependencyDeclaration{
				Direct: []RepoConfigDependency{
					RepoConfigDependency{
						Name:     "foo",
						Requires: []string{"missing"},
					},
					RepoConfigDependency{
						Name: "bar",
					},
				},
				Environment: []RepoConfigDependency{
					RepoConfigDependency{
						Name:     "baz",
						Requires: []string{"bar"},
					},
					RepoConfigDependency{
						Name: "asdf",
					},
				},
			},
			false, true, "unknown requirement of direct dependency",
		},
		{
			"unknown environment",
			DependencyDeclaration{
				Direct: []RepoConfigDependency{
					RepoConfigDependency{
						Name:     "foo",
						Requires: []string{"bar"},
					},
					RepoConfigDependency{
						Name: "bar",
					},
				},
				Environment: []RepoConfigDependency{
					RepoConfigDependency{
						Name:     "baz",
						Requires: []string{"missing"},
					},
					RepoConfigDependency{
						Name: "asdf",
					},
				},
			},
			false, true, "unknown requirement of environment dependency",
		},
		{
			"direct references environment",
			DependencyDeclaration{
				Direct: []RepoConfigDependency{
					RepoConfigDependency{
						Name:     "foo",
						Requires: []string{"baz"},
					},
					RepoConfigDependency{
						Name: "bar",
					},
				},
				Environment: []RepoConfigDependency{
					RepoConfigDependency{
						Name:     "baz",
						Requires: []string{"bar"},
					},
					RepoConfigDependency{
						Name: "asdf",
					},
				},
			},
			false, true, "unknown requirement of direct dependency",
		},
		{
			"duplicate repo",
			DependencyDeclaration{
				Direct: []RepoConfigDependency{
					RepoConfigDependency{
						Name:     "foo",
						Repo:     "foo/bar",
						Requires: []string{"bar"},
					},
					RepoConfigDependency{
						Name: "bar",
					},
				},
				Environment: []RepoConfigDependency{
					RepoConfigDependency{
						Name:     "baz",
						Repo:     "foo/bar",
						Requires: []string{"bar"},
					},
					RepoConfigDependency{
						Name: "asdf",
					},
				},
			},
			false, true, "duplicate repo dependency",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ok, err := c.input.ValidateNames()
			if err != nil {
				if !c.isError {
					t.Fatalf("should have succeeded: %v", err)
				}
				if !strings.Contains(err.Error(), c.errContains) {
					t.Fatalf("error missing expected string: %v: %v", c.errContains, err)
				}
			}
			if !ok {
				if c.isOK {
					t.Fatalf("should have been ok")
				}
			}
		})
	}
}

func TestRepoConfigRefMap(t *testing.T) {
	rc := RepoConfig{
		Application: RepoConfigAppMetadata{
			Repo:   "foo/bar",
			Ref:    "asdf",
			Branch: "foo",
		},
		Dependencies: DependencyDeclaration{
			Direct: []RepoConfigDependency{
				RepoConfigDependency{
					Name: "baz",
					Repo: "biz/baz",
					AppMetadata: RepoConfigAppMetadata{
						Repo:   "biz/baz",
						Ref:    "1111",
						Branch: "master",
					},
				},
			},
		},
	}
	rm, err := rc.RefMap()
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if v, ok := rm["foo/bar"]; ok {
		if v != rc.Application.Branch {
			t.Fatalf("foo/bar: bad branch: %v", v)
		}
	} else {
		t.Fatalf("foo/bar missing")
	}
	if v, ok := rm["biz/baz"]; ok {
		if v != rc.Dependencies.Direct[0].AppMetadata.Branch {
			t.Fatalf("biz/baz: bad branch: %v", v)
		}
	} else {
		t.Fatalf("biz/baz missing")
	}
}
