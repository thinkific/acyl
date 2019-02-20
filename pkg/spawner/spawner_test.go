package spawner

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/dollarshaveclub/acyl/pkg/ghclient"
	"github.com/dollarshaveclub/acyl/pkg/mocks"
	"github.com/golang/mock/gomock"
)

func TestRefMatcherMatchingBranch(t *testing.T) {
	rd := &RepoRevisionData{
		BaseBranch:   "master",
		BaseSHA:      "767890",
		SourceBranch: "feature-foo",
		SourceSHA:    "123456",
	}
	bl := []BranchInfo{
		BranchInfo{
			Name: "feature-foo",
			SHA:  "asdf",
		},
		BranchInfo{
			Name: "master",
			SHA:  "1234",
		},
		BranchInfo{
			Name: "feature-bar",
			SHA:  "5678",
		},
	}

	matcher := refMatcher{}
	sha, branch, err := matcher.getRefForOtherRepo(rd, "", bl)

	if err != nil {
		t.Fatalf("error getting branch: %v", err)
	}

	if branch != "feature-foo" {
		t.Fatalf(`bad branch returned (expected branch "%v"): %v`, rd.SourceBranch, branch)
	}

	if sha != "asdf" {
		t.Fatalf(`bad SHA returned (expected commit SHA "asdf"): %v`, sha)
	}
}

func TestRefMatcherMissingBranch(t *testing.T) {
	rd := &RepoRevisionData{
		BaseBranch:   "master",
		BaseSHA:      "767890",
		SourceBranch: "feature-foo",
		SourceSHA:    "123456",
	}
	bl := []BranchInfo{
		BranchInfo{
			Name: "master",
			SHA:  "asdf",
		},
		BranchInfo{
			Name: "feature-bar",
			SHA:  "1234",
		},
	}

	matcher := refMatcher{}
	sha, branch, err := matcher.getRefForOtherRepo(rd, "", bl)

	if err != nil {
		t.Fatalf("error getting branch: %v", err)
	}

	if branch != "master" {
		t.Fatalf(`bad branch returned (expected branch "%v"): %v`, rd.BaseBranch, branch)
	}

	if sha != "asdf" {
		t.Fatalf(`bad SHA returned (expected SHA "asdf"): "%v"`, sha)
	}
}

func TestRefMatcherMissingBranchFallback(t *testing.T) {
	rd := &RepoRevisionData{
		BaseBranch:   "master",
		SourceBranch: "feature-foo",
		BaseSHA:      "foo",
		SourceSHA:    "bar",
	}
	bl := []BranchInfo{
		BranchInfo{
			Name: "release",
			SHA:  "123456",
		},
		BranchInfo{
			Name: "feature-bar",
			SHA:  "asdf",
		},
	}
	override := "release"

	matcher := refMatcher{}
	sha, branch, err := matcher.getRefForOtherRepo(rd, override, bl)

	if err != nil {
		t.Fatalf("error getting branch: %v", err)
	}

	if branch != "release" {
		t.Fatalf(`bad branch returned (expected branch "%v"): %v`, bl[0].Name, branch)
	}

	if sha != "123456" {
		t.Fatalf(`bad SHA returned (expected "%v"): %v`, bl[0].SHA, sha)
	}
}

func TestRefMatcherNoSuitableBranch(t *testing.T) {
	rd := &RepoRevisionData{
		BaseBranch:   "master",
		SourceBranch: "feature-foo",
		BaseSHA:      "foo",
		SourceSHA:    "bar",
	}
	bl := []BranchInfo{
		BranchInfo{
			Name: "release",
			SHA:  "asdf",
		},
		BranchInfo{
			Name: "feature-bar",
			SHA:  "1234",
		},
	}

	matcher := refMatcher{}
	_, _, err := matcher.getRefForOtherRepo(rd, "", bl)

	if err == nil {
		t.Fatalf("should have failed")
	}

	if !strings.Contains(err.Error(), "no suitable branch") {
		t.Fatalf("unexpected error returned: %v", err)
	}
}

var testRepoRevisionData = &RepoRevisionData{
	User:         "joe",
	Repo:         "dollarshaveclub/face-web",
	PullRequest:  1,
	SourceSHA:    "asdf",
	BaseSHA:      "asdfg",
	SourceBranch: "feature-lulz",
	BaseBranch:   "master",
}

var testQAType = &QAType{
	Name:         "face-web",
	TargetRepo:   "dollarshaveclub/face-web",
	TargetBranch: "master",
	OtherRepos: []string{
		"dollarshaveclub/face-assets",
		"dollarshaveclub/rails-site",
		"dollarshaveclub/gatekeeper",
	},
}

func TestGetRefMapAllMatchingBranches(t *testing.T) {
	fwb := []BranchInfo{
		BranchInfo{
			Name: "feature-lulz",
			SHA:  "asdf",
		},
		BranchInfo{
			Name: "master",
			SHA:  "1234",
		},
	}
	fab := []BranchInfo{
		BranchInfo{
			Name: "feature-lulz",
			SHA:  "asdfassets",
		},
		BranchInfo{
			Name: "master",
			SHA:  "1234assets",
		},
	}
	rsb := []BranchInfo{
		BranchInfo{
			Name: "feature-lulz",
			SHA:  "asdfrails",
		},
		BranchInfo{
			Name: "master",
			SHA:  "1234rails",
		},
	}
	gkb := []BranchInfo{
		BranchInfo{
			Name: "feature-lulz",
			SHA:  "asdfgatekeeper",
		},
		BranchInfo{
			Name: "master",
			SHA:  "1234gatekeeper",
		},
	}
	retBranches := func(ctx context.Context, repo string) ([]BranchInfo, error) {
		switch repo {
		case "dollarshaveclub/face-web":
			return fwb, nil
		case "dollarshaveclub/face-assets":
			return fab, nil
		case "dollarshaveclub/rails-site":
			return rsb, nil
		case "dollarshaveclub/gatekeeper":
			return gkb, nil
		default:
			return nil, fmt.Errorf("unknown repo: %v", repo)
		}
	}
	qas := QASpawner{
		rc:     &ghclient.FakeRepoClient{GetBranchesFunc: retBranches},
		logger: log.New(ioutil.Discard, "", log.LstdFlags),
	}
	brm, err := qas.getRefMap(testQAType, testRepoRevisionData)
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	for k, v := range brm {
		if v.branch != "feature-lulz" {
			t.Fatalf("bad branch returned for %v: %v", k, v.branch)
		}
	}
	if brm["dollarshaveclub/face-web"].sha != fwb[0].SHA {
		t.Fatalf("bad SHA for face-web")
	}
	if brm["dollarshaveclub/face-assets"].sha != fab[0].SHA {
		t.Fatalf("bad SHA for face-assets")
	}
	if brm["dollarshaveclub/rails-site"].sha != rsb[0].SHA {
		t.Fatalf("bad SHA for rails-site")
	}
	if brm["dollarshaveclub/gatekeeper"].sha != gkb[0].SHA {
		t.Fatalf("bad SHA for gatekeeper")
	}
}

func TestGetRefMapSomeMatchingBranches(t *testing.T) {
	fwb := []BranchInfo{
		BranchInfo{
			Name: "feature-lulz",
			SHA:  "asdf",
		},
		BranchInfo{
			Name: "master",
			SHA:  "1234",
		},
	}
	fab := []BranchInfo{
		BranchInfo{
			Name: "feature-lulz",
			SHA:  "asdfassets",
		},
		BranchInfo{
			Name: "master",
			SHA:  "1234assets",
		},
	}
	rsb := []BranchInfo{
		BranchInfo{
			Name: "master",
			SHA:  "1234rails",
		},
	}
	gkb := []BranchInfo{
		BranchInfo{
			Name: "master",
			SHA:  "1234gatekeeper",
		},
	}
	retBranches := func(ctx context.Context, repo string) ([]BranchInfo, error) {
		switch repo {
		case "dollarshaveclub/face-web":
			return fwb, nil
		case "dollarshaveclub/face-assets":
			return fab, nil
		case "dollarshaveclub/rails-site":
			return rsb, nil
		case "dollarshaveclub/gatekeeper":
			return gkb, nil
		default:
			return nil, fmt.Errorf("unknown repo: %v", repo)
		}
	}
	qas := QASpawner{
		rc:     &ghclient.FakeRepoClient{GetBranchesFunc: retBranches},
		logger: log.New(ioutil.Discard, "", log.LstdFlags),
	}
	brm, err := qas.getRefMap(testQAType, testRepoRevisionData)
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if brm["dollarshaveclub/face-web"].branch != "feature-lulz" {
		t.Fatalf("bad branch for face-web")
	}
	if brm["dollarshaveclub/face-assets"].branch != "feature-lulz" {
		t.Fatalf("bad branch for face-assets")
	}
	if brm["dollarshaveclub/rails-site"].branch != "master" {
		t.Fatalf("bad branch for rails-site")
	}
	if brm["dollarshaveclub/gatekeeper"].branch != "master" {
		t.Fatalf("bad branch for gatekeeper")
	}
	if brm["dollarshaveclub/face-web"].sha != fwb[0].SHA {
		t.Fatalf("bad SHA for face-web")
	}
	if brm["dollarshaveclub/face-assets"].sha != fab[0].SHA {
		t.Fatalf("bad SHA for face-assets")
	}
	if brm["dollarshaveclub/rails-site"].sha != rsb[0].SHA {
		t.Fatalf("bad SHA for rails-site")
	}
	if brm["dollarshaveclub/gatekeeper"].sha != gkb[0].SHA {
		t.Fatalf("bad SHA for gatekeeper")
	}
}

func TestGetRefMapOverrideBranch(t *testing.T) {
	testQAType.BranchOverrides = map[string]string{
		"dollarshaveclub/rails-site": "release",
		"dollarshaveclub/gatekeeper": "release",
	}
	defer func() { testQAType.BranchOverrides = nil }()

	fwb := []BranchInfo{
		BranchInfo{
			Name: "feature-lulz",
			SHA:  "asdf",
		},
		BranchInfo{
			Name: "master",
			SHA:  "1234",
		},
	}
	fab := []BranchInfo{
		BranchInfo{
			Name: "feature-lulz",
			SHA:  "asdfassets",
		},
		BranchInfo{
			Name: "master",
			SHA:  "1234assets",
		},
	}
	rsb := []BranchInfo{
		BranchInfo{
			Name: "master",
			SHA:  "1234rails",
		},
		BranchInfo{
			Name: "release",
			SHA:  "5678rails",
		},
	}
	gkb := []BranchInfo{
		BranchInfo{
			Name: "master",
			SHA:  "1234gatekeeper",
		},
		BranchInfo{
			Name: "release",
			SHA:  "5678gatekeeper",
		},
	}
	retBranches := func(ctx context.Context, repo string) ([]BranchInfo, error) {
		switch repo {
		case "dollarshaveclub/face-web":
			return fwb, nil
		case "dollarshaveclub/face-assets":
			return fab, nil
		case "dollarshaveclub/rails-site":
			return rsb, nil
		case "dollarshaveclub/gatekeeper":
			return gkb, nil
		default:
			return nil, fmt.Errorf("unknown repo: %v", repo)
		}
	}
	qas := QASpawner{
		rc:     &ghclient.FakeRepoClient{GetBranchesFunc: retBranches},
		logger: log.New(ioutil.Discard, "", log.LstdFlags),
	}
	brm, err := qas.getRefMap(testQAType, testRepoRevisionData)
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if brm["dollarshaveclub/face-web"].branch != "feature-lulz" {
		t.Fatalf("bad branch for face-web")
	}
	if brm["dollarshaveclub/face-assets"].branch != "feature-lulz" {
		t.Fatalf("bad branch for face-assets")
	}
	if brm["dollarshaveclub/rails-site"].branch != "release" {
		t.Fatalf("bad branch for rails-site")
	}
	if brm["dollarshaveclub/gatekeeper"].branch != "release" {
		t.Fatalf("bad branch for gatekeeper")
	}
	if brm["dollarshaveclub/face-web"].sha != fwb[0].SHA {
		t.Fatalf("bad SHA for face-web")
	}
	if brm["dollarshaveclub/face-assets"].sha != fab[0].SHA {
		t.Fatalf("bad SHA for face-assets")
	}
	if brm["dollarshaveclub/rails-site"].sha != rsb[1].SHA {
		t.Fatalf("bad SHA for rails-site")
	}
	if brm["dollarshaveclub/gatekeeper"].sha != gkb[1].SHA {
		t.Fatalf("bad SHA for gatekeeper")
	}
}

func TestGetRefMapTrackingTagWithOverrideBranch(t *testing.T) {
	testQAType.BranchOverrides = map[string]string{
		"dollarshaveclub/rails-site": "release",
		"dollarshaveclub/gatekeeper": "release",
	}
	defer func() { testQAType.BranchOverrides = nil }()

	fwb := []BranchInfo{
		BranchInfo{
			Name: "feature-lulz",
			SHA:  "asdf",
		},
		BranchInfo{
			Name: "master",
			SHA:  "1234",
		},
	}
	fab := []BranchInfo{
		BranchInfo{
			Name: "feature-lulz",
			SHA:  "asdfassets",
		},
		BranchInfo{
			Name: "master",
			SHA:  "1234assets",
		},
	}
	rsb := []BranchInfo{
		BranchInfo{
			Name: "master",
			SHA:  "1234rails",
		},
		BranchInfo{
			Name: "release",
			SHA:  "5678rails",
		},
	}
	gkb := []BranchInfo{
		BranchInfo{
			Name: "master",
			SHA:  "1234gatekeeper",
		},
		BranchInfo{
			Name: "release",
			SHA:  "5678gatekeeper",
		},
	}
	fwt := []BranchInfo{ // tags
		BranchInfo{
			Name: "foo",
			SHA:  "asdf",
		},
		BranchInfo{
			Name: "bar",
			SHA:  "1234",
		},
	}
	retBranches := func(ctx context.Context, repo string) ([]BranchInfo, error) {
		switch repo {
		case "dollarshaveclub/face-web":
			return fwb, nil
		case "dollarshaveclub/face-assets":
			return fab, nil
		case "dollarshaveclub/rails-site":
			return rsb, nil
		case "dollarshaveclub/gatekeeper":
			return gkb, nil
		default:
			return nil, fmt.Errorf("unknown repo: %v", repo)
		}
	}
	retTags := func(ctx context.Context, repo string) ([]BranchInfo, error) {
		if repo == "dollarshaveclub/face-web" {
			return fwt, nil
		}
		return []BranchInfo{}, nil
	}
	retBranch := func(ctx context.Context, repo, branch string) (BranchInfo, error) {
		switch repo {
		case "dollarshaveclub/face-web":
			return fwb[1], nil
		case "dollarshaveclub/face-assets":
			return fab[1], nil
		case "dollarshaveclub/rails-site":
			return rsb[1], nil
		case "dollarshaveclub/gatekeeper":
			return gkb[1], nil
		default:
			return BranchInfo{}, fmt.Errorf("unknown repo: %v", repo)
		}
	}
	qat := *testQAType
	qat.TrackRefs = []string{"tags/foo"}
	rd := *testRepoRevisionData
	rd.PullRequest = 0
	rd.SourceBranch = ""
	rd.SourceRef = "tags/foo"
	qas := QASpawner{
		rc:     &ghclient.FakeRepoClient{GetBranchesFunc: retBranches, GetTagsFunc: retTags, GetBranchFunc: retBranch},
		logger: log.New(ioutil.Discard, "", log.LstdFlags),
	}
	brm, err := qas.getRefMap(&qat, &rd)
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if brm["dollarshaveclub/face-web"].branch != "" {
		t.Fatalf("bad branch for face-web")
	}
	if brm["dollarshaveclub/face-assets"].branch != "master" {
		t.Fatalf("bad branch for face-assets")
	}
	if brm["dollarshaveclub/rails-site"].branch != "release" {
		t.Fatalf("bad branch for rails-site")
	}
	if brm["dollarshaveclub/gatekeeper"].branch != "release" {
		t.Fatalf("bad branch for gatekeeper")
	}
	if brm["dollarshaveclub/face-web"].sha != fwt[0].SHA {
		t.Fatalf("bad SHA for face-web")
	}
	if brm["dollarshaveclub/face-assets"].sha != fab[1].SHA {
		t.Fatalf("bad SHA for face-assets")
	}
	if brm["dollarshaveclub/rails-site"].sha != rsb[1].SHA {
		t.Fatalf("bad SHA for rails-site")
	}
	if brm["dollarshaveclub/gatekeeper"].sha != gkb[1].SHA {
		t.Fatalf("bad SHA for gatekeeper")
	}
}

func TestGetRefMapTrackingTagWithNoMatchingTags(t *testing.T) {
	fwb := []BranchInfo{
		BranchInfo{
			Name: "feature-lulz",
			SHA:  "asdf",
		},
		BranchInfo{
			Name: "master",
			SHA:  "1234",
		},
	}
	fab := []BranchInfo{
		BranchInfo{
			Name: "feature-lulz",
			SHA:  "asdfassets",
		},
		BranchInfo{
			Name: "master",
			SHA:  "1234assets",
		},
	}
	rsb := []BranchInfo{
		BranchInfo{
			Name: "master",
			SHA:  "1234rails",
		},
		BranchInfo{
			Name: "release",
			SHA:  "5678rails",
		},
	}
	gkb := []BranchInfo{
		BranchInfo{
			Name: "master",
			SHA:  "1234gatekeeper",
		},
		BranchInfo{
			Name: "release",
			SHA:  "5678gatekeeper",
		},
	}
	fwt := []BranchInfo{ // tags
		BranchInfo{
			Name: "foo",
			SHA:  "asdf",
		},
		BranchInfo{
			Name: "bar",
			SHA:  "1234",
		},
	}
	retBranches := func(ctx context.Context, repo string) ([]BranchInfo, error) {
		switch repo {
		case "dollarshaveclub/face-web":
			return fwb, nil
		case "dollarshaveclub/face-assets":
			return fab, nil
		case "dollarshaveclub/rails-site":
			return rsb, nil
		case "dollarshaveclub/gatekeeper":
			return gkb, nil
		default:
			return nil, fmt.Errorf("unknown repo: %v", repo)
		}
	}
	retTags := func(ctx context.Context, repo string) ([]BranchInfo, error) {
		if repo == "dollarshaveclub/face-web" {
			return fwt, nil
		}
		return []BranchInfo{}, nil
	}
	retBranch := func(ctx context.Context, repo, branch string) (BranchInfo, error) {
		switch repo {
		case "dollarshaveclub/face-web":
			return fwb[1], nil
		case "dollarshaveclub/face-assets":
			return fab[1], nil
		case "dollarshaveclub/rails-site":
			return rsb[0], nil
		case "dollarshaveclub/gatekeeper":
			return gkb[0], nil
		default:
			return BranchInfo{}, fmt.Errorf("unknown repo: %v", repo)
		}
	}
	qat := *testQAType
	qat.TrackRefs = []string{"tags/foo"}
	rd := *testRepoRevisionData
	rd.PullRequest = 0
	rd.SourceBranch = ""
	rd.SourceRef = "tags/foo"
	qas := QASpawner{
		rc:     &ghclient.FakeRepoClient{GetBranchesFunc: retBranches, GetTagsFunc: retTags, GetBranchFunc: retBranch},
		logger: log.New(ioutil.Discard, "", log.LstdFlags),
	}
	brm, err := qas.getRefMap(&qat, &rd)
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if brm["dollarshaveclub/face-web"].branch != "" {
		t.Fatalf("bad branch for face-web")
	}
	if brm["dollarshaveclub/face-assets"].branch != "master" {
		t.Fatalf("bad branch for face-assets")
	}
	if brm["dollarshaveclub/rails-site"].branch != "master" {
		t.Fatalf("bad branch for rails-site")
	}
	if brm["dollarshaveclub/gatekeeper"].branch != "master" {
		t.Fatalf("bad branch for gatekeeper")
	}
	if brm["dollarshaveclub/face-web"].sha != fwt[0].SHA {
		t.Fatalf("bad SHA for face-web")
	}
	if brm["dollarshaveclub/face-assets"].sha != fab[1].SHA {
		t.Fatalf("bad SHA for face-assets")
	}
	if brm["dollarshaveclub/rails-site"].sha != rsb[0].SHA {
		t.Fatalf("bad SHA for rails-site")
	}
	if brm["dollarshaveclub/gatekeeper"].sha != gkb[0].SHA {
		t.Fatalf("bad SHA for gatekeeper")
	}
}

func TestGetRefMapTrackingTagWithMatchingTags(t *testing.T) {
	testQAType.BranchOverrides = map[string]string{
		"dollarshaveclub/rails-site": "release",
		"dollarshaveclub/gatekeeper": "release",
	}
	defer func() { testQAType.BranchOverrides = nil }()

	fwb := []BranchInfo{
		BranchInfo{
			Name: "feature-lulz",
			SHA:  "asdf",
		},
		BranchInfo{
			Name: "master",
			SHA:  "1234",
		},
	}
	fab := []BranchInfo{
		BranchInfo{
			Name: "feature-lulz",
			SHA:  "asdfassets",
		},
		BranchInfo{
			Name: "master",
			SHA:  "1234assets",
		},
	}
	rsb := []BranchInfo{
		BranchInfo{
			Name: "master",
			SHA:  "1234rails",
		},
		BranchInfo{
			Name: "release",
			SHA:  "5678rails",
		},
	}
	gkb := []BranchInfo{
		BranchInfo{
			Name: "master",
			SHA:  "1234gatekeeper",
		},
		BranchInfo{
			Name: "release",
			SHA:  "5678gatekeeper",
		},
	}
	fwt := []BranchInfo{ // tags
		BranchInfo{
			Name: "foo",
			SHA:  "asdf",
		},
		BranchInfo{
			Name: "bar",
			SHA:  "1234",
		},
	}
	rst := []BranchInfo{ // tags
		BranchInfo{
			Name: "foo",
			SHA:  "99990000",
		},
	}
	retBranches := func(ctx context.Context, repo string) ([]BranchInfo, error) {
		switch repo {
		case "dollarshaveclub/face-web":
			return fwb, nil
		case "dollarshaveclub/face-assets":
			return fab, nil
		case "dollarshaveclub/rails-site":
			return rsb, nil
		case "dollarshaveclub/gatekeeper":
			return gkb, nil
		default:
			return nil, fmt.Errorf("unknown repo: %v", repo)
		}
	}
	retTags := func(ctx context.Context, repo string) ([]BranchInfo, error) {
		switch repo {
		case "dollarshaveclub/face-web":
			return fwt, nil
		case "dollarshaveclub/rails-site":
			return rst, nil
		default:
			return []BranchInfo{}, nil
		}
	}
	retBranch := func(ctx context.Context, repo, branch string) (BranchInfo, error) {
		switch repo {
		case "dollarshaveclub/face-web":
			return fwb[1], nil
		case "dollarshaveclub/face-assets":
			return fab[1], nil
		case "dollarshaveclub/rails-site":
			return rsb[1], nil
		case "dollarshaveclub/gatekeeper":
			return gkb[0], nil
		default:
			return BranchInfo{}, fmt.Errorf("unknown repo: %v", repo)
		}
	}
	qat := *testQAType
	qat.TrackRefs = []string{"tags/foo"}
	rd := *testRepoRevisionData
	rd.PullRequest = 0
	rd.SourceBranch = ""
	rd.SourceRef = "tags/foo"
	qas := QASpawner{
		rc:     &ghclient.FakeRepoClient{GetBranchesFunc: retBranches, GetTagsFunc: retTags, GetBranchFunc: retBranch},
		logger: log.New(ioutil.Discard, "", log.LstdFlags),
	}
	brm, err := qas.getRefMap(&qat, &rd)
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if brm["dollarshaveclub/face-web"].branch != "" {
		t.Fatalf("bad branch for face-web")
	}
	if brm["dollarshaveclub/face-assets"].branch != "master" {
		t.Fatalf("bad branch for face-assets")
	}
	if brm["dollarshaveclub/rails-site"].branch != "" {
		t.Fatalf("bad branch for rails-site")
	}
	if brm["dollarshaveclub/gatekeeper"].branch != "release" {
		t.Fatalf("bad branch for gatekeeper")
	}
	if brm["dollarshaveclub/face-web"].sha != fwt[0].SHA {
		t.Fatalf("bad SHA for face-web")
	}
	if brm["dollarshaveclub/face-assets"].sha != fab[1].SHA {
		t.Fatalf("bad SHA for face-assets")
	}
	if brm["dollarshaveclub/rails-site"].sha != rst[0].SHA {
		t.Fatalf("bad SHA for rails-site")
	}
	if brm["dollarshaveclub/gatekeeper"].sha != gkb[0].SHA {
		t.Fatalf("bad SHA for gatekeeper")
	}
}

func TestEnsureOnePRSingleEnv(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	be := mocks.NewMockAcylBackend(ctrl)
	dl := mocks.NewMockDataLayer(ctrl)
	txn := mocks.NewMockTransaction(ctrl)
	txn.EXPECT().StartSegmentNow().AnyTimes()
	qa := QAEnvironment{
		Name: "testing-env",
	}
	qas := QASpawner{
		aminoBackend: be,
		dl:           dl,
	}
	ctx := NewNRTxnContext(context.Background(), txn)
	rd := RepoRevisionData{
		Repo:         "foo/bar",
		PullRequest:  1,
		SourceBranch: "feature",
		BaseBranch:   "master",
	}
	qt := &QAType{
		TargetBranch: "master",
		OtherRepos:   []string{},
	}
	cm := map[string]string{
		"foo/bar": "asdf",
	}
	rm := map[string]string{
		"foo/bar": "feature",
	}
	dl.EXPECT().GetQAEnvironmentsByRepoAndPR(gomock.Any(), gomock.Any()).Times(1).Return([]QAEnvironment{qa}, nil)
	dl.EXPECT().SetQAEnvironmentRepoData(qa.Name, gomock.Any()).Times(1).Return(nil)
	dl.EXPECT().SetQAEnvironmentCommitSHAMap(qa.Name, gomock.Any()).Times(1).Return(nil)
	dl.EXPECT().SetQAEnvironmentRefMap(qa.Name, gomock.Any()).Times(1).Return(nil)
	dl.EXPECT().SetQAEnvironmentCreated(qa.Name, gomock.Any()).Times(1).Return(nil)
	dl.EXPECT().SetQAEnvironmentStatus(qa.Name, Updating).Times(1).Return(nil)
	dl.EXPECT().GetQAEnvironmentConsistently(qa.Name).Times(1).Return(&qa, nil)
	qa2, err := qas.ensureOne(ctx, Updating, rd, qt, cm, rm)
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if qa2.Name != qa.Name {
		t.Fatalf("bad QA name: %v", qa2.Name)
	}
}

func TestEnsureOneTrackingSingleEnv(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	be := mocks.NewMockAcylBackend(ctrl)
	dl := mocks.NewMockDataLayer(ctrl)
	txn := mocks.NewMockTransaction(ctrl)
	txn.EXPECT().StartSegmentNow().AnyTimes()
	qa := QAEnvironment{
		Name:        "testing-env",
		Repo:        "foo/bar",
		PullRequest: 0,
		SourceRef:   "master",
	}
	qas := QASpawner{
		aminoBackend: be,
		dl:           dl,
	}
	ctx := NewNRTxnContext(context.Background(), txn)
	rd := qa.RepoRevisionDataFromQA()
	qt := &QAType{
		TargetBranch: "master",
		OtherRepos:   []string{},
		TrackRefs:    []string{"master"},
	}
	cm := map[string]string{
		"foo/bar": "asdf",
	}
	rm := map[string]string{
		"foo/bar": "feature",
	}
	dl.EXPECT().GetQAEnvironmentsByRepoAndPR(gomock.Any(), gomock.Any()).Times(1).Return([]QAEnvironment{qa}, nil)
	dl.EXPECT().SetQAEnvironmentRepoData(qa.Name, gomock.Any()).Times(1).Return(nil)
	dl.EXPECT().SetQAEnvironmentCommitSHAMap(qa.Name, gomock.Any()).Times(1).Return(nil)
	dl.EXPECT().SetQAEnvironmentRefMap(qa.Name, gomock.Any()).Times(1).Return(nil)
	dl.EXPECT().SetQAEnvironmentCreated(qa.Name, gomock.Any()).Times(1).Return(nil)
	dl.EXPECT().SetQAEnvironmentStatus(qa.Name, Updating).Times(1).Return(nil)
	dl.EXPECT().GetQAEnvironmentConsistently(qa.Name).Times(1).Return(&qa, nil)
	qa2, err := qas.ensureOne(ctx, Updating, *rd, qt, cm, rm)
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if qa2.Name != qa.Name {
		t.Fatalf("bad QA name: %v", qa2.Name)
	}
}

func TestEnsureOnePRStaleEnvs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	be := mocks.NewMockAcylBackend(ctrl)
	dl := mocks.NewMockDataLayer(ctrl)
	txn := mocks.NewMockTransaction(ctrl)
	mc := mocks.NewMockCollector(ctrl)
	mc.EXPECT().Operation(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	txn.EXPECT().StartSegmentNow().AnyTimes()
	qa1 := QAEnvironment{
		Name:         "testing-env",
		Repo:         "foo/bar",
		PullRequest:  1,
		SourceBranch: "feature",
		BaseBranch:   "master",
		Created:      time.Date(2017, 2, 0, 0, 0, 0, 0, time.UTC),
	}
	qa2 := QAEnvironment{
		Name:         "stale-env1",
		Repo:         "foo/bar",
		PullRequest:  1,
		SourceBranch: "feature",
		BaseBranch:   "master",
		Created:      time.Date(2017, 1, 0, 5, 0, 0, 0, time.UTC),
	}
	qa3 := QAEnvironment{
		Name:         "stale-env2",
		Repo:         "foo/bar",
		PullRequest:  1,
		SourceBranch: "feature",
		BaseBranch:   "master",
		Created:      time.Date(2017, 1, 0, 4, 0, 0, 0, time.UTC),
	}
	qas := QASpawner{
		aminoBackend: be,
		dl:           dl,
		omc:          mc,
	}
	envs := []QAEnvironment{qa2, qa3, qa1}
	ctx := NewNRTxnContext(context.Background(), txn)
	rd := qa1.RepoRevisionDataFromQA()
	qt := &QAType{
		TargetBranch: "master",
		OtherRepos:   []string{},
	}
	cm := map[string]string{
		"foo/bar": "asdf",
	}
	rm := map[string]string{
		"foo/bar": "feature",
	}
	dl.EXPECT().GetExtantQAEnvironments(qa1.Repo, qa1.PullRequest).AnyTimes().Return(envs, nil)
	dl.EXPECT().GetQAEnvironmentsByRepoAndPR(gomock.Any(), gomock.Any()).Times(1).Return(envs, nil)
	dl.EXPECT().SetQAEnvironmentRepoData(qa1.Name, gomock.Any()).Times(1).Return(nil)
	dl.EXPECT().SetQAEnvironmentCommitSHAMap(qa1.Name, gomock.Any()).Times(1).Return(nil)
	dl.EXPECT().SetQAEnvironmentRefMap(qa1.Name, gomock.Any()).Times(1).Return(nil)
	dl.EXPECT().SetQAEnvironmentCreated(qa1.Name, gomock.Any()).Times(1).Return(nil)
	dl.EXPECT().SetQAEnvironmentStatus(qa1.Name, Updating).Times(1).Return(nil)
	dl.EXPECT().GetQAEnvironmentConsistently(qa1.Name).Times(1).Return(&qa1, nil)
	be.EXPECT().DestroyEnvironment(gomock.Any(), gomock.Any(), gomock.Any()).Times(2).Return(nil)
	dl.EXPECT().SetQAEnvironmentStatus(qa2.Name, Destroyed).Times(1).Return(nil)
	dl.EXPECT().SetQAEnvironmentStatus(qa3.Name, Destroyed).Times(1).Return(nil)
	qa, err := qas.ensureOne(ctx, Updating, *rd, qt, cm, rm)
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if qa.Name != qa1.Name {
		t.Fatalf("bad QA name: %v", qa.Name)
	}
}

func TestEnsureOneTrackingStale(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	be := mocks.NewMockAcylBackend(ctrl)
	dl := mocks.NewMockDataLayer(ctrl)
	txn := mocks.NewMockTransaction(ctrl)
	mc := mocks.NewMockCollector(ctrl)
	mc.EXPECT().Operation(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	txn.EXPECT().StartSegmentNow().AnyTimes()
	qa1 := QAEnvironment{
		Name:        "testing-env",
		Repo:        "foo/bar",
		PullRequest: 0,
		SourceRef:   "master",
		Created:     time.Date(2017, 2, 0, 0, 0, 0, 0, time.UTC),
	}
	qa2 := QAEnvironment{
		Name:        "stale-env1",
		Repo:        "foo/bar",
		PullRequest: 0,
		SourceRef:   "master",
		Created:     time.Date(2017, 1, 0, 5, 0, 0, 0, time.UTC),
	}
	qa3 := QAEnvironment{
		Name:        "stale-env2",
		Repo:        "foo/bar",
		PullRequest: 0,
		SourceRef:   "master",
		Created:     time.Date(2017, 1, 0, 4, 0, 0, 0, time.UTC),
	}
	qas := QASpawner{
		aminoBackend: be,
		dl:           dl,
		omc:          mc,
	}
	envs := []QAEnvironment{qa2, qa3, qa1}
	ctx := NewNRTxnContext(context.Background(), txn)
	rd := qa1.RepoRevisionDataFromQA()
	qt := &QAType{
		TargetBranch: "master",
		OtherRepos:   []string{},
		TrackRefs:    []string{"master"},
	}
	cm := map[string]string{
		"foo/bar": "asdf",
	}
	rm := map[string]string{
		"foo/bar": "master",
	}
	dl.EXPECT().GetExtantQAEnvironments(qa1.Repo, qa1.PullRequest).AnyTimes().Return(envs, nil)
	dl.EXPECT().GetQAEnvironmentsByRepoAndPR(gomock.Any(), gomock.Any()).Times(1).Return(envs, nil)
	dl.EXPECT().SetQAEnvironmentRepoData(qa1.Name, gomock.Any()).Times(1).Return(nil)
	dl.EXPECT().SetQAEnvironmentCommitSHAMap(qa1.Name, gomock.Any()).Times(1).Return(nil)
	dl.EXPECT().SetQAEnvironmentRefMap(qa1.Name, gomock.Any()).Times(1).Return(nil)
	dl.EXPECT().SetQAEnvironmentCreated(qa1.Name, gomock.Any()).Times(1).Return(nil)
	dl.EXPECT().SetQAEnvironmentStatus(qa1.Name, Updating).Times(1).Return(nil)
	dl.EXPECT().GetQAEnvironmentConsistently(qa1.Name).Times(1).Return(&qa1, nil)
	be.EXPECT().DestroyEnvironment(gomock.Any(), gomock.Any(), gomock.Any()).Times(2).Return(nil)
	dl.EXPECT().SetQAEnvironmentStatus(qa2.Name, Destroyed).Times(1).Return(nil)
	dl.EXPECT().SetQAEnvironmentStatus(qa3.Name, Destroyed).Times(1).Return(nil)
	qa, err := qas.ensureOne(ctx, Updating, *rd, qt, cm, rm)
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if qa.Name != qa1.Name {
		t.Fatalf("bad QA name: %v", qa.Name)
	}
}

func TestEnsureOneTrackingNewEnv(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	be := mocks.NewMockAcylBackend(ctrl)
	dl := mocks.NewMockDataLayer(ctrl)
	txn := mocks.NewMockTransaction(ctrl)
	ng := mocks.NewMockNameGenerator(ctrl)
	txn.EXPECT().StartSegmentNow().AnyTimes()
	qas := QASpawner{
		aminoBackend: be,
		dl:           dl,
		ng:           ng,
	}
	ctx := NewNRTxnContext(context.Background(), txn)
	rd := RepoRevisionData{
		Repo:        "foo/bar",
		PullRequest: 0,
		SourceRef:   "master",
	}
	qt := &QAType{
		TargetBranch: "master",
		OtherRepos:   []string{},
		TrackRefs:    []string{"master"},
	}
	cm := map[string]string{
		"foo/bar": "asdf",
	}
	rm := map[string]string{
		"foo/bar": "feature",
	}
	newname := "new-test-env"
	ng.EXPECT().New().Times(1).Return(newname, nil)
	dl.EXPECT().GetQAEnvironmentsByRepoAndPR(gomock.Any(), gomock.Any()).Times(1).Return([]QAEnvironment{}, nil)
	dl.EXPECT().CreateQAEnvironment(gomock.Any()).Times(1).Return(nil)
	gomock.InOrder(
		dl.EXPECT().GetQAEnvironmentConsistently(newname).Times(1).Return(nil, nil),
		dl.EXPECT().GetQAEnvironmentConsistently(newname).Times(1).Return(&QAEnvironment{Name: newname}, nil),
	)
	qa, err := qas.ensureOne(ctx, Updating, rd, qt, cm, rm)
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if qa.Name != newname {
		t.Fatalf("bad QA name: %v", qa.Name)
	}
}

func TestEnsureOnePRNewEnv(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	be := mocks.NewMockAcylBackend(ctrl)
	dl := mocks.NewMockDataLayer(ctrl)
	txn := mocks.NewMockTransaction(ctrl)
	ng := mocks.NewMockNameGenerator(ctrl)
	txn.EXPECT().StartSegmentNow().AnyTimes()
	qas := QASpawner{
		aminoBackend: be,
		dl:           dl,
		ng:           ng,
	}
	ctx := NewNRTxnContext(context.Background(), txn)
	rd := RepoRevisionData{
		Repo:         "foo/bar",
		PullRequest:  1,
		SourceBranch: "feature",
		BaseBranch:   "master",
	}
	qt := &QAType{
		TargetBranch: "master",
		OtherRepos:   []string{},
	}
	cm := map[string]string{
		"foo/bar": "asdf",
	}
	rm := map[string]string{
		"foo/bar": "feature",
	}
	newname := "new-test-env"
	ng.EXPECT().New().Times(1).Return(newname, nil)
	dl.EXPECT().GetQAEnvironmentsByRepoAndPR(gomock.Any(), gomock.Any()).Times(1).Return([]QAEnvironment{}, nil)
	dl.EXPECT().CreateQAEnvironment(gomock.Any()).Times(1).Return(nil)
	gomock.InOrder(
		dl.EXPECT().GetQAEnvironmentConsistently(newname).Times(1).Return(nil, nil),
		dl.EXPECT().GetQAEnvironmentConsistently(newname).Times(1).Return(&QAEnvironment{Name: newname}, nil),
	)
	qa, err := qas.ensureOne(ctx, Updating, rd, qt, cm, rm)
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if qa.Name != newname {
		t.Fatalf("bad QA name: %v", qa.Name)
	}
}
