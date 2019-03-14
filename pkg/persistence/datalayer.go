package persistence

import (
	"time"

	"github.com/dollarshaveclub/acyl/pkg/models"
	"github.com/google/uuid"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

// LogFunc is a function that logs a formatted string somewhere
type LogFunc func(string, ...interface{})

// DataLayer describes an object that interacts with the persistant data store
type DataLayer interface {
	CreateQAEnvironment(tracer.Span, *QAEnvironment) error
	GetQAEnvironment(tracer.Span, string) (*QAEnvironment, error)
	GetQAEnvironmentConsistently(tracer.Span, string) (*QAEnvironment, error)
	GetQAEnvironments(tracer.Span) ([]QAEnvironment, error)
	DeleteQAEnvironment(tracer.Span, string) error
	GetQAEnvironmentsByStatus(span tracer.Span, status string) ([]QAEnvironment, error)
	GetRunningQAEnvironments(tracer.Span) ([]QAEnvironment, error)
	GetQAEnvironmentsByRepoAndPR(tracer.Span, string, uint) ([]QAEnvironment, error)
	GetQAEnvironmentsByRepo(span tracer.Span, repo string) ([]QAEnvironment, error)
	GetQAEnvironmentBySourceSHA(span tracer.Span, sourceSHA string) (*QAEnvironment, error)
	GetQAEnvironmentsBySourceBranch(span tracer.Span, sourceBranch string) ([]QAEnvironment, error)
	GetQAEnvironmentsByUser(span tracer.Span, user string) ([]QAEnvironment, error)
	SetQAEnvironmentStatus(tracer.Span, string, EnvironmentStatus) error
	SetQAEnvironmentRepoData(tracer.Span, string, *RepoRevisionData) error
	SetQAEnvironmentRefMap(tracer.Span, string, RefMap) error
	SetQAEnvironmentCommitSHAMap(tracer.Span, string, RefMap) error
	SetQAEnvironmentCreated(tracer.Span, string, time.Time) error
	GetExtantQAEnvironments(tracer.Span, string, uint) ([]QAEnvironment, error)
	SetAminoEnvironmentID(span tracer.Span, name string, did int) error
	SetAminoServiceToPort(span tracer.Span, name string, serviceToPort map[string]int64) error
	SetAminoKubernetesNamespace(span tracer.Span, name, namespace string) error
	AddEvent(tracer.Span, string, string) error
	Search(span tracer.Span, opts models.EnvSearchParameters) ([]QAEnvironment, error)
	GetMostRecent(span tracer.Span, n uint) ([]QAEnvironment, error)
	Close() error
	HelmDataLayer
	K8sEnvDataLayer
	EventLoggerDataLayer
}

// HelmDataLayer describes an object that stores data about Helm
type HelmDataLayer interface {
	GetHelmReleasesForEnv(span tracer.Span, name string) ([]models.HelmRelease, error)
	UpdateHelmReleaseRevision(span tracer.Span, envname, release, revision string) error
	CreateHelmReleasesForEnv(span tracer.Span, releases []models.HelmRelease) error
	DeleteHelmReleasesForEnv(span tracer.Span, name string) (uint, error)
}

// K8sEnvDataLayer describes an object that stores data about the K8s environment details
type K8sEnvDataLayer interface {
	GetK8sEnv(span tracer.Span, name string) (*models.KubernetesEnvironment, error)
	GetK8sEnvsByNamespace(span tracer.Span, ns string) ([]models.KubernetesEnvironment, error)
	CreateK8sEnv(span tracer.Span, env *models.KubernetesEnvironment) error
	DeleteK8sEnv(span tracer.Span, name string) error
	UpdateK8sEnvTillerAddr(span tracer.Span, envname, taddr string) error
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
