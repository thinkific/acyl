package env

import (
	"context"

	"github.com/dollarshaveclub/acyl/pkg/eventlogger"
	"github.com/dollarshaveclub/acyl/pkg/models"
	"github.com/dollarshaveclub/acyl/pkg/nitro/meta"
	"github.com/dollarshaveclub/acyl/pkg/persistence"
	"github.com/dollarshaveclub/acyl/pkg/spawner"
	"github.com/pkg/errors"
)

// CombinedSpawner is an object that uses a Nitro backend for acyl.yml versions > 1,
// and legacy Spawner for acyl.yml <= 1
type CombinedSpawner struct {
	DL      persistence.DataLayer
	MG      meta.Getter
	Nitro   spawner.EnvironmentSpawner
	Spawner spawner.EnvironmentSpawner
}

var _ spawner.EnvironmentSpawner = &CombinedSpawner{}

func (cb *CombinedSpawner) log(ctx context.Context, msg string, args ...interface{}) {
	eventlogger.GetLogger(ctx).Printf(msg, args...)
}

// extantUsedNitro returns whether the extant environment was created using Nitro (otherwise assume Amino)
func (cb *CombinedSpawner) extantUsedNitro(name string) (bool, error) {
	k8senv, err := cb.DL.GetK8sEnv(name)
	if err != nil {
		return false, errors.Wrap(err, "error getting k8s environment")
	}
	return k8senv != nil, nil
}

var errNoExtants = errors.New("no extant environments")

// extantUsedNitroFromRDD determines if the extant environment was created using Nitro from the RDD payload
func (cb *CombinedSpawner) extantUsedNitroFromRDD(ctx context.Context, rd models.RepoRevisionData) (bool, error) {
	envs, err := cb.DL.GetExtantQAEnvironments(rd.Repo, rd.PullRequest)
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
	return cb.extantUsedNitro(envs[0].Name)
}

func (cb *CombinedSpawner) isAcylYMLV2(ctx context.Context, rd models.RepoRevisionData) (bool, error) {
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

func (cb *CombinedSpawner) isAcylYMLV2FromName(ctx context.Context, name string) (bool, error) {
	qa, err := cb.DL.GetQAEnvironment(name)
	if err != nil {
		return false, errors.Wrap(err, "error getting QA")
	}
	return cb.isAcylYMLV2(ctx, *qa.RepoRevisionDataFromQA())
}

func (cb *CombinedSpawner) Create(ctx context.Context, rd models.RepoRevisionData) (string, error) {
	ok, err := cb.isAcylYMLV2(ctx, rd)
	if err != nil {
		return "", errors.Wrap(err, "error checking acyl.yml version")
	}
	if ok {
		cb.log(ctx, "acyl v2: using Nitro\n")
		return cb.Nitro.Create(ctx, rd)
	}
	cb.log(ctx, "acyl v1: falling back to Amino\n")
	return cb.Spawner.Create(ctx, rd)
}

func (cb *CombinedSpawner) Update(ctx context.Context, rd models.RepoRevisionData) (string, error) {
	// if acyl v2 && extant used nitro (or no extants):
	// 		-> use nitro to update
	// if acyl v2 && extant used amino:
	//		-> destroy existing using amino, create with nitro
	// if acyl v1 && extant used amino:
	//		-> update using amino (destroy & create under the hood)
	// if acyl v1 && extant used nitro:
	//	  -> destroy existing using nitro, create with amino
	v2, err := cb.isAcylYMLV2(ctx, rd)
	if err != nil {
		return "", errors.Wrap(err, "error checking acyl.yml version")
	}
	nitro, err := cb.extantUsedNitroFromRDD(ctx, rd)
	if err != nil && err != errNoExtants {
		return "", errors.Wrap(err, "error checking if extant environment used nitro")
	}
	if v2 {
		if nitro || err == errNoExtants {
			cb.log(ctx, "acyl v2 and extant used Nitro (or no extants): using Nitro to update\n")
			return cb.Nitro.Update(ctx, rd)
		}
		cb.log(ctx, "acyl v2 and extant used Amino: using Amino to destroy & Nitro to create\n")
		if err := cb.Spawner.Destroy(ctx, rd, models.DestroyApiRequest); err != nil {
			return "", errors.Wrap(err, "error destroying amino environment")
		}
		return cb.Nitro.Create(ctx, rd)
	}
	// v1
	if nitro {
		cb.log(ctx, "acyl v1 and extant used Nitro: using Nitro to destroy and Amino to create\n")
		if err := cb.Nitro.Destroy(ctx, rd, models.DestroyApiRequest); err != nil {
			return "", errors.Wrap(err, "error destroying nitro environment")
		}
		return cb.Spawner.Create(ctx, rd)
	}
	cb.log(ctx, "acyl v1 and extant used Amino: using Amino\n")
	return cb.Spawner.Update(ctx, rd)
}

func (cb *CombinedSpawner) Destroy(ctx context.Context, rd models.RepoRevisionData, reason models.QADestroyReason) error {
	ok, err := cb.extantUsedNitroFromRDD(ctx, rd)
	if err != nil {
		return errors.Wrap(err, "error checking if extant environment used nitro")
	}
	if ok {
		cb.log(ctx, "using Nitro to destroy\n")
		return cb.Nitro.Destroy(ctx, rd, reason)
	}
	cb.log(ctx, "using Amino to destroy\n")
	return cb.Spawner.Destroy(ctx, rd, reason)
}

func (cb *CombinedSpawner) DestroyExplicitly(ctx context.Context, qa *models.QAEnvironment, reason models.QADestroyReason) error {
	ok, err := cb.extantUsedNitro(qa.Name)
	if err != nil {
		return errors.Wrap(err, "error checking if extant environment used nitro")
	}
	if ok {
		cb.log(ctx, "using Nitro to destroy\n")
		return cb.Nitro.DestroyExplicitly(ctx, qa, reason)
	}
	cb.log(ctx, "using Amino to destroy\n")
	return cb.Spawner.DestroyExplicitly(ctx, qa, reason)
}

func (cb *CombinedSpawner) Success(ctx context.Context, name string) error {
	ok, err := cb.extantUsedNitro(name)
	if err != nil {
		return errors.Wrap(err, "error checking if extant environment used nitro")
	}
	if ok {
		return cb.Nitro.Success(ctx, name)
	}
	return cb.Spawner.Success(ctx, name)
}

func (cb *CombinedSpawner) Failure(ctx context.Context, name string, msg string) error {
	ok, err := cb.extantUsedNitro(name)
	if err != nil {
		return errors.Wrap(err, "error checking if extant environment used nitro")
	}
	if ok {
		return cb.Nitro.Failure(ctx, name, msg)
	}
	return cb.Spawner.Failure(ctx, name, msg)
}
