package env

import (
	"context"
	"testing"

	"github.com/dollarshaveclub/acyl/pkg/models"
	"github.com/dollarshaveclub/acyl/pkg/nitro/meta"
	"github.com/dollarshaveclub/acyl/pkg/persistence"
	"github.com/dollarshaveclub/acyl/pkg/spawner"
)

func TestNitroSpawnerExtantUsedNitro(t *testing.T) {
	env1 := &models.KubernetesEnvironment{EnvName: "foo-bar"}
	dl := persistence.NewFakeDataLayer()
	dl.CreateQAEnvironment(context.Background(), &models.QAEnvironment{Name: env1.EnvName})
	dl.CreateK8sEnv(context.Background(), env1)
	cb := NitroSpawner{DL: dl}
	ok, err := cb.extantUsedNitro(context.Background(), env1.EnvName)
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if !ok {
		t.Fatalf("expected true but got false")
	}
	ok, err = cb.extantUsedNitro(context.Background(), "something-else")
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if ok {
		t.Fatalf("expected false but got true")
	}
}

func TestNitroSpawnerExtantUsedNitroFromRDD(t *testing.T) {
	env1 := &models.QAEnvironment{Name: "foo-bar", Repo: "foo/bar", PullRequest: 1}
	env2 := &models.QAEnvironment{Name: "foo-bar2", Repo: "foo/bar2", PullRequest: 1, AminoEnvironmentID: 23}
	dl := persistence.NewFakeDataLayer()
	dl.CreateQAEnvironment(context.Background(), env1)
	dl.CreateQAEnvironment(context.Background(), env2)
	dl.CreateK8sEnv(context.Background(), &models.KubernetesEnvironment{EnvName: env1.Name})
	cb := NitroSpawner{DL: dl}
	ok, err := cb.extantUsedNitroFromRDD(context.Background(), *env1.RepoRevisionDataFromQA())
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if !ok {
		t.Fatalf("expected true but got false")
	}
	ok, err = cb.extantUsedNitroFromRDD(context.Background(), *env2.RepoRevisionDataFromQA())
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if ok {
		t.Fatalf("expected false but got true")
	}
}

func TestNitroSpawnerIsAcylYAMLV2(t *testing.T) {
	var v2 bool
	mg := &meta.FakeGetter{
		GetAcylYAMLFunc: func(ctx context.Context, rc *models.RepoConfig, repo, ref string) (err error) {
			if v2 {
				return nil
			}
			return meta.ErrUnsupportedVersion
		},
	}
	env1 := &models.QAEnvironment{Name: "foo-bar", Repo: "foo/bar", PullRequest: 1}
	cb := NitroSpawner{MG: mg}
	ok, err := cb.isAcylYMLV2(context.Background(), *env1.RepoRevisionDataFromQA())
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if ok {
		t.Fatalf("expected false but got true")
	}
	v2 = true
	ok, err = cb.isAcylYMLV2(context.Background(), *env1.RepoRevisionDataFromQA())
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if !ok {
		t.Fatalf("expected true but got false")
	}
}

func TestNitroSpawnerIsAcylYAMLV2FromName(t *testing.T) {
	var v2 bool
	mg := &meta.FakeGetter{
		GetAcylYAMLFunc: func(ctx context.Context, rc *models.RepoConfig, repo, ref string) (err error) {
			if v2 {
				return nil
			}
			return meta.ErrUnsupportedVersion
		},
	}
	env1 := &models.QAEnvironment{Name: "foo-bar", Repo: "foo/bar", PullRequest: 1}
	dl := persistence.NewFakeDataLayer()
	dl.CreateQAEnvironment(context.Background(), env1)
	cb := NitroSpawner{MG: mg, DL: dl}
	ok, err := cb.isAcylYMLV2FromName(context.Background(), env1.Name)
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if ok {
		t.Fatalf("expected false but got true")
	}
	v2 = true
	ok, err = cb.isAcylYMLV2FromName(context.Background(), env1.Name)
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if !ok {
		t.Fatalf("expected true but got false")
	}
}

func TestNitroSpawnerCreate(t *testing.T) {
	var v2 bool
	mg := &meta.FakeGetter{
		GetAcylYAMLFunc: func(ctx context.Context, rc *models.RepoConfig, repo, ref string) (err error) {
			if v2 {
				return nil
			}
			return meta.ErrUnsupportedVersion
		},
	}
	var nitrocalled, aminocalled bool
	fknitro := &spawner.FakeEnvironmentSpawner{
		CreateFunc: func(ctx context.Context, rd models.RepoRevisionData) (string, error) {
			nitrocalled = true
			return "foo-bar", nil
		},
	}
	env1 := &models.QAEnvironment{Name: "foo-bar", Repo: "foo/bar", PullRequest: 1}
	cb := NitroSpawner{MG: mg, Nitro: fknitro}
	_, err := cb.Create(context.Background(), *env1.RepoRevisionDataFromQA())
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if nitrocalled || !aminocalled {
		t.Fatalf("expected amino and not nitro")
	}
	v2 = true
	nitrocalled, aminocalled = false, false
	_, err = cb.Create(context.Background(), *env1.RepoRevisionDataFromQA())
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if aminocalled || !nitrocalled {
		t.Fatalf("expected nitro and not amino")
	}
}

func TestNitroSpawnerUpdate(t *testing.T) {
	var v2 bool
	mg := &meta.FakeGetter{
		GetAcylYAMLFunc: func(ctx context.Context, rc *models.RepoConfig, repo, ref string) (err error) {
			if v2 {
				return nil
			}
			return meta.ErrUnsupportedVersion
		},
	}
	var nitrocalled, aminocalled bool
	fknitro := &spawner.FakeEnvironmentSpawner{
		CreateFunc: func(ctx context.Context, rd models.RepoRevisionData) (string, error) {
			nitrocalled = true
			return "foo-bar", nil
		},
	}
	env1 := &models.QAEnvironment{Name: "foo-bar", Repo: "foo/bar", PullRequest: 1}
	cb := NitroSpawner{MG: mg, Nitro: fknitro}
	_, err := cb.Update(context.Background(), *env1.RepoRevisionDataFromQA())
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if nitrocalled || !aminocalled {
		t.Fatalf("expected amino and not nitro")
	}
	v2 = true
	nitrocalled, aminocalled = false, false
	_, err = cb.Update(context.Background(), *env1.RepoRevisionDataFromQA())
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if aminocalled || !nitrocalled {
		t.Fatalf("expected nitro and not amino")
	}
}

func TestNitroSpawnerDestroy(t *testing.T) {
	env1 := &models.QAEnvironment{Name: "foo-bar", Repo: "foo/bar", PullRequest: 1}
	env2 := &models.QAEnvironment{Name: "foo-bar2", Repo: "foo/bar2", PullRequest: 1, AminoEnvironmentID: 23}
	dl := persistence.NewFakeDataLayer()
	dl.CreateQAEnvironment(context.Background(), env1)
	dl.CreateK8sEnv(context.Background(), &models.KubernetesEnvironment{EnvName: env1.Name})
	dl.CreateQAEnvironment(context.Background(), env2)
	var nitrocalled, aminocalled bool
	fknitro := &spawner.FakeEnvironmentSpawner{
		DestroyFunc: func(ctx context.Context, rd models.RepoRevisionData, reason models.QADestroyReason) error {
			nitrocalled = true
			return nil
		},
	}
	cb := NitroSpawner{DL: dl, Nitro: fknitro}
	if err := cb.Destroy(context.Background(), *env1.RepoRevisionDataFromQA(), models.DestroyApiRequest); err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if aminocalled || !nitrocalled {
		t.Fatalf("expected nitro and not amino")
	}
	nitrocalled, aminocalled = false, false
	if err := cb.Destroy(context.Background(), *env2.RepoRevisionDataFromQA(), models.DestroyApiRequest); err != nil {
		t.Fatalf("should have succeeded 2: %v", err)
	}
	if nitrocalled || !aminocalled {
		t.Fatalf("expected amino and not nitro")
	}
}

func TestNitroSpawnerDestroyExplicitly(t *testing.T) {
	env1 := &models.QAEnvironment{Name: "foo-bar", Repo: "foo/bar", PullRequest: 1}
	env2 := &models.QAEnvironment{Name: "foo-bar2", Repo: "foo/bar2", PullRequest: 1, AminoEnvironmentID: 23}
	dl := persistence.NewFakeDataLayer()
	dl.CreateQAEnvironment(context.Background(), env1)
	dl.CreateK8sEnv(context.Background(), &models.KubernetesEnvironment{EnvName: env1.Name})
	dl.CreateQAEnvironment(context.Background(), env2)
	var nitrocalled, aminocalled bool
	fknitro := &spawner.FakeEnvironmentSpawner{
		DestroyExplicitlyFunc: func(ctx context.Context, env *models.QAEnvironment, reason models.QADestroyReason) error {
			nitrocalled = true
			return nil
		},
	}
	cb := NitroSpawner{DL: dl, Nitro: fknitro}
	if err := cb.DestroyExplicitly(context.Background(), env1, models.DestroyApiRequest); err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if aminocalled || !nitrocalled {
		t.Fatalf("expected nitro and not amino")
	}
	nitrocalled, aminocalled = false, false
	if err := cb.DestroyExplicitly(context.Background(), env2, models.DestroyApiRequest); err != nil {
		t.Fatalf("should have succeeded 2: %v", err)
	}
	if nitrocalled || !aminocalled {
		t.Fatalf("expected amino and not nitro")
	}
}

func TestNitroSpawnerSuccess(t *testing.T) {
	env1 := &models.QAEnvironment{Name: "foo-bar", Repo: "foo/bar", PullRequest: 1}
	env2 := &models.QAEnvironment{Name: "foo-bar2", Repo: "foo/bar2", PullRequest: 1, AminoEnvironmentID: 23}
	dl := persistence.NewFakeDataLayer()
	dl.CreateQAEnvironment(context.Background(), env1)
	dl.CreateK8sEnv(context.Background(), &models.KubernetesEnvironment{EnvName: env1.Name})
	dl.CreateQAEnvironment(context.Background(), env2)
	var nitrocalled, aminocalled bool
	fknitro := &spawner.FakeEnvironmentSpawner{
		SuccessFunc: func(ctx context.Context, name string) error {
			nitrocalled = true
			return nil
		},
	}
	cb := NitroSpawner{DL: dl, Nitro: fknitro}
	if err := cb.Success(context.Background(), env1.Name); err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if aminocalled || !nitrocalled {
		t.Fatalf("expected nitro and not amino")
	}
	nitrocalled, aminocalled = false, false
	if err := cb.Success(context.Background(), env2.Name); err != nil {
		t.Fatalf("should have succeeded 2: %v", err)
	}
	if nitrocalled || !aminocalled {
		t.Fatalf("expected amino and not nitro")
	}
}

func TestNitroSpawnerFailure(t *testing.T) {
	env1 := &models.QAEnvironment{Name: "foo-bar", Repo: "foo/bar", PullRequest: 1}
	env2 := &models.QAEnvironment{Name: "foo-bar2", Repo: "foo/bar2", PullRequest: 1, AminoEnvironmentID: 23}
	dl := persistence.NewFakeDataLayer()
	dl.CreateQAEnvironment(context.Background(), env1)
	dl.CreateK8sEnv(context.Background(), &models.KubernetesEnvironment{EnvName: env1.Name})
	dl.CreateQAEnvironment(context.Background(), env2)
	var nitrocalled, aminocalled bool
	fknitro := &spawner.FakeEnvironmentSpawner{
		FailureFunc: func(ctx context.Context, name, msg string) error {
			nitrocalled = true
			return nil
		},
	}
	cb := NitroSpawner{DL: dl, Nitro: fknitro}
	if err := cb.Failure(context.Background(), env1.Name, ""); err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if aminocalled || !nitrocalled {
		t.Fatalf("expected nitro and not amino")
	}
	nitrocalled, aminocalled = false, false
	if err := cb.Failure(context.Background(), env2.Name, ""); err != nil {
		t.Fatalf("should have succeeded 2: %v", err)
	}
	if nitrocalled || !aminocalled {
		t.Fatalf("expected amino and not nitro")
	}
}
