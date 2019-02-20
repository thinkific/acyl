package persistence

import (
	"time"

	"github.com/dollarshaveclub/acyl/pkg/models"
	"github.com/google/uuid"
)

// LogFunc is a function that logs a formatted string somewhere
type LogFunc func(string, ...interface{})

// DataLayer describes an object that interacts with the persistant data store
type DataLayer interface {
	CreateQAEnvironment(*QAEnvironment) error
	GetQAEnvironment(string) (*QAEnvironment, error)
	GetQAEnvironmentConsistently(string) (*QAEnvironment, error)
	GetQAEnvironments() ([]QAEnvironment, error)
	DeleteQAEnvironment(string) error
	GetQAEnvironmentsByStatus(status string) ([]QAEnvironment, error)
	GetRunningQAEnvironments() ([]QAEnvironment, error)
	GetQAEnvironmentsByRepoAndPR(string, uint) ([]QAEnvironment, error)
	GetQAEnvironmentsByRepo(repo string) ([]QAEnvironment, error)
	GetQAEnvironmentBySourceSHA(sourceSHA string) (*QAEnvironment, error)
	GetQAEnvironmentsBySourceBranch(sourceBranch string) ([]QAEnvironment, error)
	GetQAEnvironmentsByUser(user string) ([]QAEnvironment, error)
	SetQAEnvironmentStatus(string, EnvironmentStatus) error
	SetQAEnvironmentRepoData(string, *RepoRevisionData) error
	SetQAEnvironmentRefMap(string, RefMap) error
	SetQAEnvironmentCommitSHAMap(string, RefMap) error
	SetQAEnvironmentCreated(string, time.Time) error
	GetExtantQAEnvironments(string, uint) ([]QAEnvironment, error)
	SetAminoEnvironmentID(name string, did int) error
	SetAminoServiceToPort(name string, serviceToPort map[string]int64) error
	SetAminoKubernetesNamespace(name, namespace string) error
	AddEvent(string, string) error
	Search(opts models.EnvSearchParameters) ([]QAEnvironment, error)
	GetMostRecent(n uint) ([]QAEnvironment, error)
	Close() error
	HelmDataLayer
	K8sEnvDataLayer
	EventLoggerDataLayer
}

// HelmDataLayer describes an object that stores data about Helm
type HelmDataLayer interface {
	GetHelmReleasesForEnv(name string) ([]models.HelmRelease, error)
	UpdateHelmReleaseRevision(envname, release, revision string) error
	CreateHelmReleasesForEnv(releases []models.HelmRelease) error
	DeleteHelmReleasesForEnv(name string) (uint, error)
}

// K8sEnvDataLayer describes an object that stores data about the K8s environment details
type K8sEnvDataLayer interface {
	GetK8sEnv(name string) (*models.KubernetesEnvironment, error)
	GetK8sEnvsByNamespace(ns string) ([]models.KubernetesEnvironment, error)
	CreateK8sEnv(env *models.KubernetesEnvironment) error
	DeleteK8sEnv(name string) error
	UpdateK8sEnvTillerAddr(envname, taddr string) error
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
