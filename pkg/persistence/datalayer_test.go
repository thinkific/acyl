package persistence

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/dollarshaveclub/acyl/pkg/models"
)

func TestDataLayerCreateQAEnvironment(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	qae := QAEnvironment{
		Name:         "something-clever",
		Repo:         "dollarshaveclub/asdf",
		PullRequest:  1,
		User:         "alicejones",
		SourceBranch: "feature-4-lulz",
		SourceSHA:    "asdk9339kdkde9e9e",
		QAType:       "asdf",
	}
	err := dl.CreateQAEnvironment(&qae)
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
}

func TestDataLayerGetQAEnvironment(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	qae, err := dl.GetQAEnvironment("foo-bar")
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if qae == nil {
		t.Fatalf("should have found env")
	}
	if qae.Name != "foo-bar" {
		t.Fatalf("wrong env: %v", qae.Name)
	}
}

func TestDataLayerGetQAEnvironmentConsistently(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	qae, err := dl.GetQAEnvironmentConsistently("foo-bar")
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if qae == nil {
		t.Fatalf("should have found env")
	}
	if qae.Name != "foo-bar" {
		t.Fatalf("wrong env: %v", qae.Name)
	}
}

func TestDataLayerGetQAEnvironments(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	qs, err := dl.GetQAEnvironments()
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if len(qs) != 7 {
		t.Fatalf("wrong count: %v", len(qs))
	}
}

func TestDataLayerDeleteQAEnvironment(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	err := dl.DeleteQAEnvironment("foo-bar-2")
	if err == nil {
		t.Fatalf("delete should have failed because environment is not destroyed")
	}
	if err := dl.CreateK8sEnv(&models.KubernetesEnvironment{EnvName: "foo-bar-2", Namespace: "foo-bar"}); err != nil {
		t.Fatalf("create k8senv should have succeeded: %v", err)
	}
	if err := dl.CreateHelmReleasesForEnv([]models.HelmRelease{models.HelmRelease{EnvName: "foo-bar-2", Release: "foo"}}); err != nil {
		t.Fatalf("k8senv create should have succeeded: %v", err)
	}
	if err := dl.SetQAEnvironmentStatus("foo-bar-2", models.Destroyed); err != nil {
		t.Fatalf("set status should have succeeded: %v", err)
	}
	err = dl.DeleteQAEnvironment("foo-bar-2")
	if err != nil {
		t.Fatalf("delete should have succeeded: %v", err)
	}
	qae, err := dl.GetQAEnvironmentConsistently("foo-bar-2")
	if err != nil {
		t.Fatalf("get should have succeeded: %v", err)
	}
	k8senv, err := dl.GetK8sEnv("foo-bar-2")
	if err != nil {
		t.Fatalf("get k8senv should have succeeded: %v", err)
	}
	if k8senv != nil {
		t.Fatalf("k8senv should have been deleted")
	}
	hr, err := dl.GetHelmReleasesForEnv("foo-bar-2")
	if err != nil {
		t.Fatalf("get helm releases should have succeeded: %v", err)
	}
	if len(hr) != 0 {
		t.Fatalf("helm releases should have been deleted")
	}
	if qae != nil {
		t.Fatalf("should have not found env")
	}
}

func TestDataLayerGetQAEnvironmentsByStatus(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	qs, err := dl.GetQAEnvironmentsByStatus("success")
	if err != nil {
		t.Fatalf("success get should have succeeded: %v", err)
	}
	if len(qs) != 5 {
		t.Fatalf("wrong success count: %v", len(qs))
	}
	qs, err = dl.GetQAEnvironmentsByStatus("destroyed")
	if err != nil {
		t.Fatalf("destroy get should have succeeded: %v", err)
	}
	if len(qs) != 1 {
		t.Fatalf("wrong destroyed count: %v", len(qs))
	}
}

func TestDataLayerGetRunningQAEnvironments(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	qs, err := dl.GetRunningQAEnvironments()
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if len(qs) != 5 {
		t.Fatalf("wrong running count: %v", len(qs))
	}
}

func TestDataLayerGetQAEnvironmentsByRepoAndPR(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	qs, err := dl.GetQAEnvironmentsByRepoAndPR("dollarshaveclub/foo-baz", 99)
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if len(qs) != 2 {
		t.Fatalf("wrong running count: %v", len(qs))
	}
	for _, q := range qs {
		if !strings.Contains(q.Name, "foo-baz") {
			t.Fatalf("wrong name: %v", q.Name)
		}
	}
}

func TestDataLayerGetQAEnvironmentsByRepo(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	qs, err := dl.GetQAEnvironmentsByRepo("dollarshaveclub/foo-bar")
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if len(qs) != 2 {
		t.Fatalf("wrong running count: %v", len(qs))
	}
	var found int
	for _, qa := range qs {
		if qa.Name == "foo-bar" {
			found++
		}
		if qa.Name == "foo-bar-2" {
			found++
		}
	}
	if found != 2 {
		t.Fatalf("unexpected qa list: %v", qs)
	}
}

func TestDataLayerSetQAEnvironmentStatus(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	err := dl.SetQAEnvironmentStatus("foo-bar", models.Spawned)
	if err != nil {
		t.Fatalf("set should have succeeded: %v", err)
	}
	qae, err := dl.GetQAEnvironmentConsistently("foo-bar")
	if err != nil {
		t.Fatalf("get should have succeeded: %v", err)
	}
	if qae.Name != "foo-bar" {
		t.Fatalf("wrong name: %v", qae.Name)
	}
	if qae.Status != models.Spawned {
		t.Fatalf("wrong status: %v", qae.Status)
	}
}

func TestDataLayerSetQAEnvironmentRepoData(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	rd := RepoRevisionData{
		User:         "bobsmith",
		Repo:         "foo/asdf",
		PullRequest:  110,
		SourceSHA:    "asdf1234",
		BaseSHA:      "qwerty5678",
		SourceBranch: "foo",
		BaseBranch:   "master",
	}
	err := dl.SetQAEnvironmentRepoData("foo-bar", &rd)
	if err != nil {
		t.Fatalf("set should have succeeded: %v", err)
	}
	qae, err := dl.GetQAEnvironmentConsistently("foo-bar")
	if err != nil {
		t.Fatalf("get should have succeeded: %v", err)
	}
	if qae.Name != "foo-bar" {
		t.Fatalf("wrong name: %v", qae.Name)
	}
	if qae.Repo != "foo/asdf" {
		t.Fatalf("wrong repo: %v", qae.Repo)
	}
	if qae.PullRequest != 110 {
		t.Fatalf("wrong pull request: %v", qae.PullRequest)
	}
	if qae.SourceSHA != rd.SourceSHA {
		t.Fatalf("wrong source sha: %v", qae.SourceSHA)
	}
	if qae.BaseSHA != rd.BaseSHA {
		t.Fatalf("wrong base sha: %v", qae.BaseSHA)
	}
	if qae.SourceBranch != rd.SourceBranch {
		t.Fatalf("wrong source branch: %v", qae.SourceBranch)
	}
	if qae.BaseBranch != rd.BaseBranch {
		t.Fatalf("wrong base branch: %v", qae.BaseBranch)
	}
	if qae.User != rd.User {
		t.Fatalf("wrong user: %v", qae.User)
	}
}

func TestDataLayerSetQAEnvironmentRefMap(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	err := dl.SetQAEnvironmentRefMap("foo-bar", map[string]string{"dollarshaveclub/foo-bar": "some-branch"})
	if err != nil {
		t.Fatalf("set should have succeeded: %v", err)
	}
	qae, err := dl.GetQAEnvironmentConsistently("foo-bar")
	if err != nil {
		t.Fatalf("get should have succeeded: %v", err)
	}
	if qae.Name != "foo-bar" {
		t.Fatalf("wrong name: %v", qae.Name)
	}
	if len(qae.RefMap) != 1 {
		t.Fatalf("wrong refmap size: %v", len(qae.RefMap))
	}
	if v, ok := qae.RefMap["dollarshaveclub/foo-bar"]; ok {
		if v != "some-branch" {
			t.Fatalf("wrong branch: %v", v)
		}
	} else {
		t.Fatalf("refmap missing key")
	}
}

func TestDataLayerSetQAEnvironmentCommitSHAMap(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	sha := "37a1218def12549a56e4e48be95d9cdf9a20d45d"
	err := dl.SetQAEnvironmentCommitSHAMap("foo-bar", map[string]string{"dollarshaveclub/foo-bar": sha})
	if err != nil {
		t.Fatalf("set should have succeeded: %v", err)
	}
	qae, err := dl.GetQAEnvironmentConsistently("foo-bar")
	if err != nil {
		t.Fatalf("get should have succeeded: %v", err)
	}
	if qae.Name != "foo-bar" {
		t.Fatalf("wrong name: %v", qae.Name)
	}
	if len(qae.CommitSHAMap) != 1 {
		t.Fatalf("wrong CommitSHAMap size: %v", len(qae.RefMap))
	}
	if v, ok := qae.CommitSHAMap["dollarshaveclub/foo-bar"]; ok {
		if v != sha {
			t.Fatalf("wrong commit sha: %v", v)
		}
	} else {
		t.Fatalf("CommitSHAMap missing key")
	}
}

func TestDataLayerSetQAEnvironmentCreated(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	ts := time.Now().UTC()
	err := dl.SetQAEnvironmentCreated("foo-bar", ts)
	if err != nil {
		t.Fatalf("set should have succeeded: %v", err)
	}
	qae, err := dl.GetQAEnvironmentConsistently("foo-bar")
	if err != nil {
		t.Fatalf("get should have succeeded: %v", err)
	}
	if qae.Name != "foo-bar" {
		t.Fatalf("wrong name: %v", qae.Name)
	}
	ts = ts.Truncate(1 * time.Microsecond)
	if !qae.Created.Truncate(1 * time.Microsecond).Equal(ts) {
		t.Fatalf("wrong created timestamp: %v (wanted: %v)", qae.Created, ts)
	}
}

func TestDataLayerGetExtantQAEnvironments(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	qs, err := dl.GetExtantQAEnvironments("dollarshaveclub/foo-baz", 99)
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if len(qs) != 1 {
		t.Fatalf("wrong extant count: %v", len(qs))
	}
	if qs[0].Name != "foo-baz" {
		t.Fatalf("wrong name: %v", qs[0].Name)
	}
}

func TestDataLayerSetAminoEnvironmentID(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	err := dl.SetAminoEnvironmentID("foo-bar", 1000)
	if err != nil {
		t.Fatalf("set should have succeeded: %v", err)
	}
	qae, err := dl.GetQAEnvironmentConsistently("foo-bar")
	if err != nil {
		t.Fatalf("get should have succeeded: %v", err)
	}
	if qae.Name != "foo-bar" {
		t.Fatalf("wrong name: %v", qae.Name)
	}
	if qae.AminoEnvironmentID != 1000 {
		t.Fatalf("wrong amino env id: %v", qae.AminoEnvironmentID)
	}
}

func TestDataLayerSetAminoServiceToPort(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	err := dl.SetAminoServiceToPort("foo-bar", map[string]int64{"oracle": 6666})
	if err != nil {
		t.Fatalf("set should have succeeded: %v", err)
	}
	qae, err := dl.GetQAEnvironmentConsistently("foo-bar")
	if err != nil {
		t.Fatalf("get should have succeeded: %v", err)
	}
	if qae.Name != "foo-bar" {
		t.Fatalf("wrong name: %v", qae.Name)
	}
	if v, ok := qae.AminoServiceToPort["oracle"]; ok {
		if v != 6666 {
			t.Fatalf("wrong port: %v", v)
		}
	} else {
		t.Fatalf("AminoServiceToPort missing key")
	}
}

func TestDataLayerSetAminoKubernetesNamespace(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	err := dl.SetAminoKubernetesNamespace("foo-bar", "k8s-foo-bar")
	if err != nil {
		t.Fatalf("set should have succeeded: %v", err)
	}
	qae, err := dl.GetQAEnvironmentConsistently("foo-bar")
	if err != nil {
		t.Fatalf("get should have succeeded: %v", err)
	}
	if qae.Name != "foo-bar" {
		t.Fatalf("wrong name: %v", qae.Name)
	}
	if qae.AminoKubernetesNamespace != "k8s-foo-bar" {
		t.Fatalf("wrong amino k8s namespace: %v", qae.AminoKubernetesNamespace)
	}
}

func TestDataLayerAddEvent(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	qae, err := dl.GetQAEnvironment("foo-bar")
	if err != nil {
		t.Fatalf("first get should have succeeded: %v", err)
	}
	if qae.Name != "foo-bar" {
		t.Fatalf("wrong env: %v", qae.Name)
	}
	i := len(qae.Events)
	err = dl.AddEvent("foo-bar", "this is a msg")
	if err != nil {
		t.Fatalf("add should have succeeded: %v", err)
	}
	qae, err = dl.GetQAEnvironmentConsistently("foo-bar")
	if err != nil {
		t.Fatalf("second get should have succeeded: %v", err)
	}
	if len(qae.Events) != i+1 {
		t.Fatalf("bad event count: %v", len(qae.Events))
	}
	if qae.Events[i].Message != "this is a msg" {
		t.Fatalf("bad msg: %v", qae.Events[1].Message)
	}
}

func TestDataLayerGetQAEnvironmentBySourceSHA(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	qae, err := dl.GetQAEnvironmentBySourceSHA("93jdhsksjusnc839")
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if qae.Name != "foo-baz" {
		t.Fatalf("wrong env: %v", qae.Name)
	}
}

func TestDataLayerGetQAEnvironmentBySourceBranch(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	qas, err := dl.GetQAEnvironmentsBySourceBranch("feature-crash-the-site")
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if len(qas) != 1 {
		t.Fatalf("bad qa count: %v (wanted 1)", len(qas))
	}
	if qas[0].Name != "foo-baz" {
		t.Fatalf("wrong env: %v", qas[0].Name)
	}
}

func TestDataLayerGetQAEnvironmentByUser(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	qas, err := dl.GetQAEnvironmentsByUser("alicejones")
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if len(qas) != 1 {
		t.Fatalf("bad qa count: %v (wanted 1)", len(qas))
	}
	if qas[0].Name != "foo-bar-2" {
		t.Fatalf("wrong env: %v", qas[0].Name)
	}
}

func TestDataLayerSearch(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}

	sopts := models.EnvSearchParameters{
		Repo:         "dollarshaveclub/foo-bar",
		Pr:           0,
		SourceSHA:    "",
		SourceBranch: "",
		User:         "",
		Status:       0,
		TrackingRef:  "",
	}

	defer tdl.TearDown()
	var qas []QAEnvironment
	var err error
	qas, err = dl.Search(sopts)
	if err != nil {
		t.Fatalf("repo: should have succeeded: %v", err)
	}
	if len(qas) != 2 {
		t.Fatalf("repo: unexpected length: %v", len(qas))
	}

	sopts.Pr = 99

	qas, err = dl.Search(sopts)
	if err != nil {
		t.Fatalf("repo/pr: should have succeeded: %v", err)
	}
	if len(qas) != 1 {
		t.Fatalf("repo/pr: unexpected length: %v", len(qas))
	}
	if qas[0].Name != "foo-bar" {
		t.Fatalf("repo/pr: bad qa: %v", qas[0].Name)
	}
	sopts.Repo = ""
	sopts.Pr = 0
	sopts.SourceSHA = "372829sskskskdkdke499393"

	qas, err = dl.Search(sopts)
	if err != nil {
		t.Fatalf("source sha: should have succeeded: %v", err)
	}
	if len(qas) != 1 {
		t.Fatalf("source sha: unexpected length: %v", len(qas))
	}
	if qas[0].Name != "biz-baz" {
		t.Fatalf("source sha: bad qa: %v", qas[0].Name)
	}
	sopts.SourceSHA = ""
	sopts.SourceBranch = "feature-crash-the-site"
	qas, err = dl.Search(sopts)
	if err != nil {
		t.Fatalf("source branch: should have succeeded: %v", err)
	}
	if len(qas) != 1 {
		t.Fatalf("source branch:unexpected length: %v", len(qas))
	}
	if qas[0].Name != "foo-baz" {
		t.Fatalf("source branch:bad qa: %v", qas[0].Name)
	}
	sopts.SourceBranch = ""
	sopts.User = "bobsmith"
	qas, err = dl.Search(sopts)
	if err != nil {
		t.Fatalf("user: should have succeeded: %v", err)
	}
	if len(qas) != 1 {
		t.Fatalf("user: unexpected length: %v", len(qas))
	}
	if qas[0].Name != "foo-bar" {
		t.Fatalf("user: bad qa: %v", qas[0].Name)
	}
	sopts.User = ""
	sopts.Status = models.Success
	qas, err = dl.Search(sopts)
	if err != nil {
		t.Fatalf("status: should have succeeded: %v", err)
	}
	if len(qas) != 5 {
		t.Fatalf("status: unexpected length: %v", len(qas))
	}
	sopts.Repo = "dollarshaveclub/foo-bar"
	sopts.SourceBranch = "feature-sell-moar-stuff"
	sopts.User = "alicejones"
	qas, err = dl.Search(sopts)
	if err != nil {
		t.Fatalf("multi 1: should have succeeded: %v", err)
	}
	if len(qas) != 1 {
		t.Fatalf("multi 1: unexpected length: %v", len(qas))
	}
	if qas[0].Name != "foo-bar-2" {
		t.Fatalf("multi 1: bad qa: %v", qas[0].Name)
	}
	sopts.Repo = "dollarshaveclub/biz-baz"
	sopts.SourceBranch = ""
	sopts.User = ""
	qas, err = dl.Search(sopts)
	if err != nil {
		t.Fatalf("repo + status success: should have succeeded: %v", err)
	}
	if len(qas) != 2 {
		t.Fatalf("repo + status success: unexpected length: %v", len(qas))
	}
	sopts.Status = models.Destroyed
	qas, err = dl.Search(sopts)
	if err != nil {
		t.Fatalf("repo + status destroyed: should have succeeded: %v", err)
	}
	if len(qas) != 1 {
		t.Fatalf("repo + status destroyed: unexpected length: %v", len(qas))
	}
	if qas[0].Name != "biz-baz" {
		t.Fatalf("repo + status destroyed: unexpected name: %v", qas[0].Name)
	}
	sopts.Repo = "dollarshaveclub/foo-bar"
	sopts.SourceBranch = "feature-sell-moar-stuff"
	sopts.User = "notalicejones"
	sopts.Status = models.Success
	qas, err = dl.Search(sopts)
	if err != nil {
		t.Fatalf("multi 2: should have succeeded: %v", err)
	}
	if len(qas) != 0 {
		t.Fatalf("multi 2: expected no results: %v", qas)
	}
}

func TestDataLayerSearchWithTrackingRef(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	sopts := models.EnvSearchParameters{
		Repo:         "dollarshaveclub/biz-baz",
		Pr:           0,
		SourceSHA:    "",
		SourceBranch: "",
		User:         "",
		Status:       0,
		TrackingRef:  "master",
	}

	defer tdl.TearDown()
	var qas []QAEnvironment
	var err error
	qas, err = dl.Search(sopts)
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if len(qas) != 1 {
		t.Fatalf("bad qa count: %v", len(qas))
	}
	if qas[0].Name != "biz-biz2" {
		t.Fatalf("bad qa name: %v", qas[0].Name)
	}
}

func TestDataLayerGetMostRecentEmpty(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	qas, err := dl.GetMostRecent(0)
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if len(qas) != 0 {
		t.Fatalf("should have empty results: %v", qas)
	}
}

func TestDataLayerGetMostRecentFiveDays(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	qas, err := dl.GetMostRecent(5)
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if len(qas) != 5 {
		t.Fatalf("should have returned all envs: %v", qas)
	}
}

func TestDataLayerGetMostRecentTwoDays(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	qas, err := dl.GetMostRecent(2)
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if len(qas) != 2 {
		t.Fatalf("should have returned two envs: %v", qas)
	}
}

func TestDataLayerGetMostRecentOneDay(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	qas, err := dl.GetMostRecent(1)
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if len(qas) != 1 {
		t.Fatalf("should have returned one env: %v", qas)
	}
}

func TestDataLayerGetMostRecentOrdering(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	qas, err := dl.GetMostRecent(5)
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if len(qas) != 5 {
		t.Fatalf("should have returned all envs: %v", qas)
	}
	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		if !strings.HasPrefix(qas[i].Created.String(), now.AddDate(0, 0, -i).Format("2006-01-02")) {
			t.Fatalf("bad ordering: %v\n", qas)
		}
	}
}

func TestDataLayerGetHelmReleasesForEnv(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	releases, err := dl.GetHelmReleasesForEnv("foo-bar")
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if len(releases) == 0 {
		t.Fatalf("empty results")
	}
	for i, r := range releases {
		if r.EnvName != "foo-bar" {
			t.Fatalf("bad env name: %v", r.EnvName)
		}
		if r.Created.Equal(time.Time{}) {
			t.Fatalf("zero value for created (offset: %v): %v", i, r.Created)
		}
	}
}

func TestDataLayerUpdateHelmReleaseRevision(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	if err := dl.UpdateHelmReleaseRevision("foo-bar", "some-random-name", "9999"); err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	rel, err := dl.GetHelmReleasesForEnv("foo-bar")
	if err != nil {
		t.Fatalf("get should have succeeded: %v", err)
	}
	for _, r := range rel {
		if r.Release == "some-random-name" {
			if r.RevisionSHA != "9999" {
				t.Fatalf("bad revision: %v", r.RevisionSHA)
			}
		}
	}
}

func TestDataLayerCreateHelmReleasesForEnv(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	releases := []models.HelmRelease{
		models.HelmRelease{EnvName: "foo-bar-2", Release: "some-random-name", Name: "dollarshaveclub-foo-bar-2"},
		models.HelmRelease{EnvName: "foo-bar-2", Release: "some-random-name2", Name: "dollarshaveclub-foo-bar-3"},
		models.HelmRelease{EnvName: "foo-bar-2", Release: "some-random-name3", Name: "dollarshaveclub-foo-bar-4"},
	}
	if err := dl.CreateHelmReleasesForEnv(releases); err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	releases, err := dl.GetHelmReleasesForEnv("foo-bar-2")
	if err != nil {
		t.Fatalf("get should have succeeded: %v", err)
	}
	if len(releases) != 3 {
		t.Fatalf("bad result length: %v", len(releases))
	}
	if releases[0].EnvName != "foo-bar-2" {
		t.Fatalf("bad env name: %v", releases[0].EnvName)
	}
	releases = []models.HelmRelease{models.HelmRelease{EnvName: "asdf", Release: "some-random-name", Name: "dollarshaveclub-foo-bar-2"}}
	if err := dl.CreateHelmReleasesForEnv(releases); err == nil {
		t.Fatalf("should have failed with env not found")
	}
}

func TestDataLayerDeleteHelmReleasesForEnv(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	n, err := dl.DeleteHelmReleasesForEnv("foo-bar")
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if n != 2 {
		t.Fatalf("bad count: %v", n)
	}
	releases, err := dl.GetHelmReleasesForEnv("foo-bar")
	if err != nil {
		t.Fatalf("get should have succeeded: %v", err)
	}
	if len(releases) != 0 {
		t.Fatalf("bad result length: %v", len(releases))
	}
	n, err = dl.DeleteHelmReleasesForEnv("does-not-exist")
	if err != nil || n != 0 {
		t.Fatalf("should have succeeded: %v", err)
	}
}

func TestDataLayerGetK8sEnv(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	env, err := dl.GetK8sEnv("foo-bar")
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if env.EnvName != "foo-bar" {
		t.Fatalf("bad env name: %v", env.EnvName)
	}
	if env.TillerAddr != "10.10.10.10:1234" {
		t.Fatalf("bad tiller addr: %v", env.TillerAddr)
	}
	if env.Created.Equal(time.Time{}) {
		t.Fatalf("zero value for created: %v", env.Created)
	}
	env, err = dl.GetK8sEnv("does-not-exist")
	if err != nil {
		t.Fatalf("should have succeeded with empty results: %v", err)
	}
	if env != nil {
		t.Fatalf("should have returned a nil env: %v", env)
	}
}

func TestDataLayerGetK8sEnvByNamespace(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	envs, err := dl.GetK8sEnvsByNamespace("foo")
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if len(envs) != 1 {
		t.Fatalf("should have found k8senv")
	}
	env := envs[0]
	if env.EnvName != "foo-bar" {
		t.Fatalf("bad env name: %v", env.EnvName)
	}
	if env.TillerAddr != "10.10.10.10:1234" {
		t.Fatalf("bad tiller addr: %v", env.TillerAddr)
	}
	if env.Created.Equal(time.Time{}) {
		t.Fatalf("zero value for created: %v", env.Created)
	}
	envs, err = dl.GetK8sEnvsByNamespace("does-not-exist")
	if err != nil {
		t.Fatalf("should have succeeded with empty results: %v", err)
	}
	if len(envs) != 0 {
		t.Fatalf("should have returned a zero envs: %v", env)
	}
}

func TestDataLayerCreateK8sEnv(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	if err := dl.CreateK8sEnv(&models.KubernetesEnvironment{EnvName: "foo-bar-2"}); err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if err := dl.CreateK8sEnv(&models.KubernetesEnvironment{EnvName: "does-not-exist"}); err == nil {
		t.Fatalf("should have failed with unknown env")
	}
}

func TestDataLayerDeleteK8sEnv(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	if err := dl.DeleteK8sEnv("foo-bar"); err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if err := dl.DeleteK8sEnv("does-not-exist"); err != nil {
		t.Fatalf("delete of non-extant env should have succeeded: %v", err)
	}
}

func TestDataLayerUpdateK8sEnvTillerAddr(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	if err := dl.UpdateK8sEnvTillerAddr("foo-bar", "192.168.1.1:1234"); err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	env, err := dl.GetK8sEnv("foo-bar")
	if err != nil {
		t.Fatalf("get env should have succeeded: %v", err)
	}
	if env.TillerAddr != "192.168.1.1:1234" {
		t.Fatalf("bad tiller addr: %v", env.TillerAddr)
	}
}

func TestDataLayerGetEventLogByID(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	id, _ := uuid.Parse("db20d1e7-1e0d-45c6-bfe1-4ea24b7f4159")
	el, err := dl.GetEventLogByID(id)
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if el == nil {
		t.Fatalf("empty result")
	}
	if i := len(el.Log); i != 2 {
		t.Fatalf("expected log length of 2: %v", i)
	}
	if el.EnvName != "foo-bar" {
		t.Fatalf("unexpected env name: %v", el.EnvName)
	}
	if el.Created.Equal(time.Time{}) {
		t.Fatalf("zero value for created: %v", el.Created)
	}
}

func TestDataLayerGetEventLogsByEnvName(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	logs, err := dl.GetEventLogsByEnvName("foo-bar")
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if i := len(logs); i != 2 {
		t.Fatalf("expected length of 2: %v", i)
	}
}

func TestDataLayerGetEventLogsByRepoAndPR(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	logs, err := dl.GetEventLogsByRepoAndPR("acme/something", 23)
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if i := len(logs); i != 1 {
		t.Fatalf("expected length of 1: %v", i)
	}
}

func TestDataLayerCreateEventLog(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	el := models.EventLog{
		ID:      uuid.Must(uuid.NewRandom()),
		EnvName: "asdf",
		Log: []string{
			"foo",
		},
	}
	if err := dl.CreateEventLog(&el); err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	el2, err := dl.GetEventLogByID(el.ID)
	if err != nil {
		t.Fatalf("get should have succeeded: %v", err)
	}
	if el2.EnvName != el.EnvName {
		t.Fatalf("bad env name: %v", el2.EnvName)
	}
	if el2.Log[0] != el.Log[0] {
		t.Fatalf("bad log: %v", el2.Log)
	}
}

func TestDataLayerAppendToEventLog(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	id := uuid.Must(uuid.Parse("db20d1e7-1e0d-45c6-bfe1-4ea24b7f4159"))
	if err := dl.AppendToEventLog(id, "foo"); err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	el, err := dl.GetEventLogByID(id)
	if err != nil {
		t.Fatalf("get should have succeeded: %v", err)
	}
	if i := len(el.Log); i != 3 {
		t.Fatalf("bad length: %v", i)
	}
	if el.Log[2] != "foo" {
		t.Fatalf("bad log msg: %v", el.Log[2])
	}
}

func TestDataLayerSetEventLogEnvName(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	id := uuid.Must(uuid.Parse("db20d1e7-1e0d-45c6-bfe1-4ea24b7f4159"))
	if err := dl.SetEventLogEnvName(id, "1234"); err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	el, err := dl.GetEventLogByID(id)
	if err != nil {
		t.Fatalf("get should have succeeded: %v", err)
	}
	if el.EnvName != "1234" {
		t.Fatalf("bad env name: %v", el.EnvName)
	}
}

func TestDataLayerDeleteEventLog(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	id := uuid.Must(uuid.Parse("db20d1e7-1e0d-45c6-bfe1-4ea24b7f4159"))
	if err := dl.DeleteEventLog(id); err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	el, err := dl.GetEventLogByID(id)
	if err != nil {
		t.Fatalf("get should have succeeded: %v", err)
	}
	if el != nil {
		t.Fatalf("expected empty results: %v", el)
	}
}

func TestDataLayerDeleteEventLogsByEnvName(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	n, err := dl.DeleteEventLogsByEnvName("foo-bar")
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected delete count of 2: %v", n)
	}
	logs, err := dl.GetEventLogsByEnvName("foo-bar")
	if err != nil {
		t.Fatalf("get should have succeeded: %v", err)
	}
	if i := len(logs); i != 0 {
		t.Fatalf("expected empty results: len: %v: %v", i, logs)
	}
}

func TestDataLayerDeleteEventLogsByRepoAndPR(t *testing.T) {
	dl, tdl := NewTestDataLayer(t)
	if err := tdl.Setup(testDataPath); err != nil {
		t.Fatalf("error setting up test database: %v", err)
	}
	defer tdl.TearDown()
	n, err := dl.DeleteEventLogsByRepoAndPR("acme/something", 23)
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected delete count of 1: %v", n)
	}
	logs, err := dl.GetEventLogsByRepoAndPR("acme/something", 23)
	if err != nil {
		t.Fatalf("get should have succeeded: %v", err)
	}
	if i := len(logs); i != 0 {
		t.Fatalf("expected empty results: len: %v: %v", i, logs)
	}
}
