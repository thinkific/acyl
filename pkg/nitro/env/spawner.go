package env

import (
	"context"

	"github.com/dollarshaveclub/acyl/pkg/eventlogger"
	"github.com/dollarshaveclub/acyl/pkg/models"
	"github.com/dollarshaveclub/acyl/pkg/nitro/meta"
	"github.com/dollarshaveclub/acyl/pkg/persistence"
	"github.com/pkg/errors"
)

// Temporary workaround to support decoupling amino from prior Nitro CombinedSpawner
// EnvironmentSpawner describes an object capable of managing environments
type EnvironmentSpawner interface {
	Create(context.Context, models.RepoRevisionData) (string, error)
	Update(context.Context, models.RepoRevisionData) (string, error)
	Destroy(context.Context, models.RepoRevisionData, models.QADestroyReason) error
	DestroyExplicitly(context.Context, *models.QAEnvironment, models.QADestroyReason) error
	Success(context.Context, string) error
	Failure(context.Context, string, string) error
}

// NitroSpawner is an object that uses a Nitro backend for acyl.yml versions > 1,
// return error for legacy AminoSpawner for acyl.yml <= 1
type NitroSpawner struct {
	DL           persistence.DataLayer
	MG           meta.Getter
	Nitro        EnvironmentSpawner
}

var _ EnvironmentSpawner = &NitroSpawner{}

func (cb *NitroSpawner) log(ctx context.Context, msg string, args ...interface{}) {
	eventlogger.GetLogger(ctx).Printf(msg, args...)
}

// extantUsedNitro returns whether the extant environment was created using Nitro (otherwise assume Amino)
func (cb *NitroSpawner) extantUsedNitro(ctx context.Context, name string) (bool, error) {
	k8senv, err := cb.DL.GetK8sEnv(ctx, name)
	if err != nil {
		return false, errors.Wrap(err, "error getting k8s environment")
	}
	return k8senv != nil, nil
}

var errNoExtants = errors.New("no extant environments")

// extantUsedNitroFromRDD determines if the extant environment was created using Nitro from the RDD payload
func (cb *NitroSpawner) extantUsedNitroFromRDD(ctx context.Context, rd models.RepoRevisionData) (bool, error) {
	envs, err := cb.DL.GetExtantQAEnvironments(ctx, rd.Repo, rd.PullRequest)
	if err != nil {
		return false, errors.Wrap(err, "error getting extant environments")
	}
	if len(envs) != 1 {
		cb.log(ctx, "cannot determine if Nitro was used for extant env: expected exactly one extant environment, got %v", len(envs))
		if len(envs) == 0 {
			return false, errNoExtants
		}
		return false, nil
	}
	return cb.extantUsedNitro(ctx, envs[0].Name)
}

func (cb *NitroSpawner) isAcylYMLV2(ctx context.Context, rd models.RepoRevisionData) (bool, error) {
	err := cb.MG.GetAcylYAML(ctx, &models.RepoConfig{}, rd.Repo, rd.SourceSHA)
	if err != nil {
		if err == meta.ErrUnsupportedVersion {
			cb.log(ctx, "acyl.yml for %v@%v is less than 2", rd.Repo, rd.SourceSHA)
			return false, nil
		}
		return false, errors.Wrap(err, "error getting acyl.yml")
	}
	cb.log(ctx, "acyl.yml for %v@%v is >= 2", rd.Repo, rd.SourceSHA)
	return true, nil
}

func (cb *NitroSpawner) isAcylYMLV2FromName(ctx context.Context, name string) (bool, error) {
	qa, err := cb.DL.GetQAEnvironment(ctx, name)
	if err != nil {
		return false, errors.Wrap(err, "error getting QA")
	}
	return cb.isAcylYMLV2(ctx, *qa.RepoRevisionDataFromQA())
}

func (cb *NitroSpawner) Create(ctx context.Context, rd models.RepoRevisionData) (string, error) {
	ok, err := cb.isAcylYMLV2(ctx, rd)
	if err != nil {
		return "", errors.Wrap(err, "error checking acyl.yml version")
	}
	if !ok {
		cb.log(ctx, "acyl v1: not supported, please use acyl v2\n")
		return "", errors.Wrap(err, "error acyl version 1 not supported")
	}
	cb.log(ctx, "acyl v2: using Nitro to create\n")
	return cb.Nitro.Create(ctx, rd)
}

func (cb *NitroSpawner) Update(ctx context.Context, rd models.RepoRevisionData) (string, error) {
	ok, err := cb.isAcylYMLV2(ctx, rd)
	if err != nil {
		return "", errors.Wrap(err, "error checking acyl.yml version")
	}
	if !ok {
		cb.log(ctx, "acyl v1: not supported, please use acyl v2\n")
		return "", errors.Wrap(err, "error acyl version 1 not supported")
	}
	cb.log(ctx, "acyl v2: using Nitro to update\n")
	return cb.Nitro.Update(ctx, rd)
}

func (cb *NitroSpawner) Destroy(ctx context.Context, rd models.RepoRevisionData, reason models.QADestroyReason) error {
	ok, err := cb.isAcylYMLV2(ctx, rd)
	if err != nil {
		return errors.Wrap(err, "error checking acyl.yml version")
	}
	if !ok {
		cb.log(ctx, "acyl v1: not supported, please use acyl v2\n")
		return errors.Wrap(err, "error acyl version 1 not supported")
	}
	cb.log(ctx, "using Nitro to destroy\n")
	return cb.Nitro.Destroy(ctx, rd, reason)
}

func (cb *NitroSpawner) DestroyExplicitly(ctx context.Context, qa *models.QAEnvironment, reason models.QADestroyReason) error {
	ok, err := cb.extantUsedNitro(ctx, qa.Name)
	if err != nil {
		return errors.Wrap(err, "error checking if extant environment used nitro")
	}
	if !ok {
		cb.log(ctx, "acyl v1: not supported, please use acyl v2\n")
		return errors.Wrap(err, "error acyl version 1 not supported")
	}
	cb.log(ctx, "using Nitro to destroy explicitly\n")
	return cb.DestroyExplicitly(ctx, qa, reason)
}

func (cb *NitroSpawner) Success(ctx context.Context, name string) error {
	ok, err := cb.extantUsedNitro(ctx, name)
	if err != nil {
		return errors.Wrap(err, "error checking if extant environment used nitro")
	}
	if !ok {
		cb.log(ctx, "acyl v1: not supported, please use acyl v2\n")
		return errors.Wrap(err, "error acyl version 1 not supported")
	}
	return cb.Nitro.Success(ctx, name)
}

func (cb *NitroSpawner) Failure(ctx context.Context, name string, msg string) error {
	ok, err := cb.extantUsedNitro(ctx, name)
	if err != nil {
		return errors.Wrap(err, "error checking if extant environment used nitro")
	}
	if !ok {
		cb.log(ctx, "acyl v1: not supported, please use acyl v2\n")
		return errors.Wrap(err, "error acyl version 1 not supported")
	}
	return cb.Nitro.Failure(ctx, name, msg)
}
