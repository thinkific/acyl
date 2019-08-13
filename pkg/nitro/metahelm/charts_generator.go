package metahelm

import (
	"context"
	"fmt"

	"github.com/dollarshaveclub/acyl/pkg/models"
	"github.com/dollarshaveclub/metahelm/pkg/metahelm"
	"github.com/pkg/errors"
)

// QAEnvironmentChartsGenerator is an interface for generating charts for a QA environment that is ingestible by the packages in github.com/dollarshaveclub/metahelm.
type QAEnvironmentChartsGenerator interface {
	Generate(
		ctx context.Context,
		ns string, newenv *EnvInfo,
		cloc ChartLocations,
		genAppChart func(i int, rcd models.RepoConfigDependency) (_ metahelm.Chart, err error),
	) (out []metahelm.Chart, err error)
}

// MonorepoChartsGenerator manages the generation of charts from a monorepo.
type MonorepoChartsGenerator struct{}

func (mcg MonorepoChartsGenerator) Generate(
	ctx context.Context,
	ns string,
	newenv *EnvInfo,
	cloc ChartLocations,
	genAppChart func(i int, rcd models.RepoConfigDependency) (_ metahelm.Chart, err error),
) (out []metahelm.Chart, err error) {
	// Create slice of primary applications in order to maintain DAG.
	var primaryApps []models.RepoConfigDependency
	for _, app := range newenv.RC.Monorepo.Applications {
		rcd := models.RepoConfigDependency{Name: models.GetName(newenv.RC.Monorepo.Repo()), Repo: newenv.RC.Monorepo.Repo(), AppMetadata: *app, Requires: []string{}}
		primaryApps = append(primaryApps, rcd)
	}

	dmap := map[string]struct{}{}
	reqlist := []string{}
	for i, d := range newenv.RC.Dependencies.All() {
		dmap[d.Name] = struct{}{}
		reqlist = append(reqlist, d.Requires...)
		dc, err := genAppChart(i+1, d)
		if err != nil {
			return out, errors.Wrapf(err, "error generating chart: %v", d.Name)
		}
		out = append(out, dc)
		appendToPrimaryAppsRequires(primaryApps, d.Name)
	}
	for _, r := range reqlist { // verify that everything referenced in 'requires' exists
		if _, ok := dmap[r]; !ok {
			return out, fmt.Errorf("unknown requires on chart: %v", r)
		}
	}
	for _, app := range primaryApps {
		pc, err := genAppChart(0, app)
		if err != nil {
			return out, errors.Wrap(err, "error generating primary application chart")
		}
		out = append(out, pc)
	}
	return out, nil
}

func appendToPrimaryAppsRequires(primaryApps []models.RepoConfigDependency, depName string) {
	for _, app := range primaryApps {
		app.Requires = append(app.Requires, depName)
	}
}

// SingleAppRepoChartsGenerator manages the fetching of charts from a code repo that only has a single primary application.
type SingleAppRepoChartsGenerator struct{}

func (scg SingleAppRepoChartsGenerator) Generate(
	ctx context.Context,
	ns string,
	newenv *EnvInfo,
	cloc ChartLocations,
	genAppChart func(i int, rcd models.RepoConfigDependency) (_ metahelm.Chart, err error),
) (out []metahelm.Chart, err error) {
	prc := models.RepoConfigDependency{Name: models.GetName(newenv.RC.Application.Repo), Repo: newenv.RC.Application.Repo, AppMetadata: newenv.RC.Application, Requires: []string{}}
	dmap := map[string]struct{}{}
	reqlist := []string{}
	for i, d := range newenv.RC.Dependencies.All() {
		dmap[d.Name] = struct{}{}
		reqlist = append(reqlist, d.Requires...)
		dc, err := genAppChart(i+1, d)
		if err != nil {
			return out, errors.Wrapf(err, "error generating chart: %v", d.Name)
		}
		out = append(out, dc)
		prc.Requires = append(prc.Requires, d.Name)
	}
	for _, r := range reqlist { // verify that everything referenced in 'requires' exists
		if _, ok := dmap[r]; !ok {
			return out, fmt.Errorf("unknown requires on chart: %v", r)
		}
	}
	pc, err := genAppChart(0, prc)
	if err != nil {
		return out, errors.Wrap(err, "error generating primary application chart")
	}
	out = append(out, pc)
	return out, nil
}
