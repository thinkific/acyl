package persistence

import (
	"context"
	"time"

	"github.com/dollarshaveclub/acyl/pkg/models"
	"github.com/google/uuid"
)

// LogFunc is a function that logs a formatted string somewhere
type LogFunc func(string, ...interface{})

// DataLayer describes an object that interacts with the persistant data store
type DataLayer interface {
	CreateQAEnvironment(context.Context, *QAEnvironment) error
	GetQAEnvironment(context.Context, string) (*QAEnvironment, error)
	GetQAEnvironmentConsistently(context.Context, string) (*QAEnvironment, error)
	GetQAEnvironments(context.Context) ([]QAEnvironment, error)
	DeleteQAEnvironment(context.Context, string) error
	GetQAEnvironmentsByStatus(ctx context.Context, status string) ([]QAEnvironment, error)
	GetRunningQAEnvironments(context.Context) ([]QAEnvironment, error)
	GetQAEnvironmentsByRepoAndPR(context.Context, string, uint) ([]QAEnvironment, error)
	GetQAEnvironmentsByRepo(ctx context.Context, repo string) ([]QAEnvironment, error)
	GetQAEnvironmentBySourceSHA(ctx context.Context, sourceSHA string) (*QAEnvironment, error)
	GetQAEnvironmentsBySourceBranch(ctx context.Context, sourceBranch string) ([]QAEnvironment, error)
	GetQAEnvironmentsByUser(ctx context.Context, user string) ([]QAEnvironment, error)
	SetQAEnvironmentStatus(context.Context, string, EnvironmentStatus) error
	SetQAEnvironmentRepoData(context.Context, string, *RepoRevisionData) error
	SetQAEnvironmentRefMap(context.Context, string, RefMap) error
	SetQAEnvironmentCommitSHAMap(context.Context, string, RefMap) error
	SetQAEnvironmentCreated(context.Context, string, time.Time) error
	GetExtantQAEnvironments(context.Context, string, uint) ([]QAEnvironment, error)
	SetAminoEnvironmentID(ctx context.Context, name string, did int) error
	SetAminoServiceToPort(ctx context.Context, name string, serviceToPort map[string]int64) error
	SetAminoKubernetesNamespace(ctx context.Context, name, namespace string) error
	AddEvent(context.Context, string, string) error
	Search(ctx context.Context, opts models.EnvSearchParameters) ([]QAEnvironment, error)
	GetMostRecent(ctx context.Context, n uint) ([]QAEnvironment, error)
	Close() error
	HelmDataLayer
	K8sEnvDataLayer
	EventLoggerDataLayer
}

// HelmDataLayer describes an object that stores data about Helm
type HelmDataLayer interface {
	GetHelmReleasesForEnv(ctx context.Context, name string) ([]models.HelmRelease, error)
	UpdateHelmReleaseRevision(ctx context.Context, envname, release, revision string) error
	CreateHelmReleasesForEnv(ctx context.Context, releases []models.HelmRelease) error
	DeleteHelmReleasesForEnv(ctx context.Context, name string) (uint, error)
}

// K8sEnvDataLayer describes an object that stores data about the K8s environment details
type K8sEnvDataLayer interface {
	GetK8sEnv(ctx context.Context, name string) (*models.KubernetesEnvironment, error)
	GetK8sEnvsByNamespace(ctx context.Context, ns string) ([]models.KubernetesEnvironment, error)
	CreateK8sEnv(ctx context.Context, env *models.KubernetesEnvironment) error
	DeleteK8sEnv(ctx context.Context, name string) error
	UpdateK8sEnvTillerAddr(ctx context.Context, envname, taddr string) error
}

// EventLoggerDataLayer desribes an object that stores event log data
type EventLoggerDataLayer interface {
	GetEventLogByID(id uuid.UUID) (*models.EventLog, error)
	GetEventLogsByEnvName(name string) ([]models.EventLog, error)
	GetEventLogsByRepoAndPR(repo string, pr uint) ([]models.EventLog, error)
	CreateEventLog(elog *models.EventLog) error
	SetEventLogEnvName(id uuid.UUID, name string) error
	AppendToEventLog(id uuid.UUID, msg string) error
	DeleteEventLog(id uuid.UUID) error
	DeleteEventLogsByEnvName(name string) (uint, error)
	DeleteEventLogsByRepoAndPR(repo string, pr uint) (uint, error)
}
