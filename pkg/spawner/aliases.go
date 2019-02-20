package spawner

import (
  "github.com/dollarshaveclub/acyl/pkg/ghclient"
  "github.com/dollarshaveclub/acyl/pkg/ghevent"
  "github.com/dollarshaveclub/acyl/pkg/models"
  "github.com/dollarshaveclub/acyl/pkg/config"
  "github.com/dollarshaveclub/acyl/pkg/metrics"
  "github.com/dollarshaveclub/acyl/pkg/persistence"
  "github.com/dollarshaveclub/acyl/pkg/namegen"
  "github.com/dollarshaveclub/acyl/pkg/slacknotifier"
  "github.com/dollarshaveclub/acyl/pkg/locker"
)

type QAType = models.QAType
type QAEnvironment = models.QAEnvironment
type QAEnvironments = models.QAEnvironments
type EnvironmentStatus = models.EnvironmentStatus
type RepoRevisionData = models.RepoRevisionData
type RefMap = models.RefMap
type QADestroyReason = models.QADestroyReason
type QAEnvironmentEvent = models.QAEnvironmentEvent

type MigrateConfig = config.MigrateConfig
type ServerConfig = config.ServerConfig
type AWSCreds = config.AWSCreds
type AWSConfig = config.AWSConfig
type GithubConfig = config.GithubConfig
type SlackConfig = config.SlackConfig
type AminoConfig = config.AminoConfig
type BackendConfig = config.BackendConfig
type VaultConfig = config.VaultConfig
type SecretsConfig = config.SecretsConfig
type ConsulConfig = config.ConsulConfig

type RepoClient = ghclient.RepoClient
type BranchInfo = ghclient.BranchInfo
type CommitStatus = ghclient.CommitStatus

type GitHubEventWebhook = ghevent.GitHubEventWebhook
type BadSignature = ghevent.BadSignature

type MetricsCollector = metrics.Collector

type DataLayer = persistence.DataLayer

type NameGenerator = namegen.NameGenerator

type ChatNotifier = slacknotifier.ChatNotifier

type PreemptiveLocker = locker.PreemptiveLocker
type LockProvider = locker.LockProvider
type PreemptiveLockProvider = locker.PreemptiveLockProvider
var NewPreemptiveLocker = locker.NewPreemptiveLocker

const (
  Destroyed = models.Destroyed
  Spawned = models.Spawned
  Failure = models.Failure
  Updating = models.Updating
  Success = models.Success
  DestroyApiRequest = models.DestroyApiRequest
  EnvironmentLimitExceeded = models.EnvironmentLimitExceeded
  ReapPrClosed = models.ReapPrClosed
  ReapAgeSpawned = models.ReapAgeSpawned
  ReapAgeFailure = models.ReapAgeFailure
  ReapEnvironmentLimitExceeded = models.ReapEnvironmentLimitExceeded
  Update = ghevent.Update
  CreateNew = ghevent.CreateNew
  Destroy = ghevent.Destroy
  NotRelevant = ghevent.NotRelevant
)
