package spawner

import (
	"context"
)

var _ EnvironmentSpawner = &FakeEnvironmentSpawner{}

type FakeEnvironmentSpawner struct {
	CreateFunc            func(ctx context.Context, rd RepoRevisionData) (string, error)
	UpdateFunc            func(ctx context.Context, rd RepoRevisionData) (string, error)
	DestroyFunc           func(ctx context.Context, rd RepoRevisionData, reason QADestroyReason) error
	DestroyExplicitlyFunc func(ctx context.Context, env *QAEnvironment, reason QADestroyReason) error
	SuccessFunc           func(ctx context.Context, name string) error
	FailureFunc           func(ctx context.Context, name, msg string) error
}

func (fes *FakeEnvironmentSpawner) Create(ctx context.Context, rd RepoRevisionData) (string, error) {
	return fes.CreateFunc(ctx, rd)
}
func (fes *FakeEnvironmentSpawner) Update(ctx context.Context, rd RepoRevisionData) (string, error) {
	return fes.UpdateFunc(ctx, rd)
}
func (fes *FakeEnvironmentSpawner) Destroy(ctx context.Context, rd RepoRevisionData, reason QADestroyReason) error {
	return fes.DestroyFunc(ctx, rd, reason)
}
func (fes *FakeEnvironmentSpawner) DestroyExplicitly(ctx context.Context, env *QAEnvironment, reason QADestroyReason) error {
	return fes.DestroyExplicitlyFunc(ctx, env, reason)
}
func (fes *FakeEnvironmentSpawner) Success(ctx context.Context, name string) error {
	return fes.SuccessFunc(ctx, name)
}
func (fes *FakeEnvironmentSpawner) Failure(ctx context.Context, name, msg string) error {
	return fes.FailureFunc(ctx, name, msg)
}
