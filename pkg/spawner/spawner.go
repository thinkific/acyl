package spawner

import (
	"golang.org/x/net/context"
)

// EnvironmentSpawner describes an object capable of managing environments
type EnvironmentSpawner interface {
	Create(context.Context, RepoRevisionData) (string, error)
	Update(context.Context, RepoRevisionData) (string, error)
	Destroy(context.Context, RepoRevisionData, QADestroyReason) error
	DestroyExplicitly(context.Context, *QAEnvironment, QADestroyReason) error
	Success(context.Context, string) error
	Failure(context.Context, string, string) error
}
