package meta

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dollarshaveclub/acyl/pkg/models"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

// QAEnvironmentChartsFetcher is an interface that, given a function for fetching a single chart, can manage fetching charts for an entire QAEnvironment.
type QAEnvironmentChartsFetcher interface {
	Fetch(
		ctx context.Context,
		rc *models.RepoConfig,
		basePath string,
		fetch func(i int, rc models.RepoConfigDependency) (*ChartLocation, error),
	) (ChartLocations, error)
}

// MonorepoChartsFetcher manages the fetching of charts from a monorepo.
type MonorepoChartsFetcher struct{}

func (mcf MonorepoChartsFetcher) Fetch(
	ctx context.Context,
	rc *models.RepoConfig,
	basePath string,
	fetch func(i int, rc models.RepoConfigDependency) (*ChartLocation, error),
) (ChartLocations, error) {
	// Fetch charts for all applications
	name := models.GetName(rc.Monorepo.Repo())
	out := map[string]ChartLocation{}
	eg, _ := errgroup.WithContext(ctx)
	chartCount := 0
	for _, app := range rc.Monorepo.Applications {
		eg.Go(func() error {
			loc, err := fetch(chartCount, models.RepoConfigDependency{Name: name, AppMetadata: *app})
			if err != nil || loc == nil {
				return errors.Wrapf(err, "error getting primary application chart: %v, %v", name, app.ChartRepoPath)
			}
			return nil
		})
		chartCount++
	}

	// Wait for primary application charts to be fetched
	if err := eg.Wait(); err != nil {
		return nil, errors.Wrap(err, "error fetching charts")
	}

	// Fetch Depedency Charts
	offsetmap := make(map[int]*models.RepoConfigDependency, rc.Dependencies.Count())
	couts := make([]ChartLocation, rc.Dependencies.Count())
	for i, d := range rc.Dependencies.All() {
		offsetmap[i] = &d
		eg.Go(func() error {
			loc, err := fetch(i+chartCount, d)
			if err != nil || loc == nil {
				return errors.Wrapf(err, "error getting dependency chart: %v", d.Name)
			}
			couts[i] = *loc
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, errors.Wrap(err, "error fetching charts")
	}
	for i := 0; i < rc.Dependencies.Count(); i++ {
		loc := couts[i]
		d := offsetmap[i]
		for _, v := range d.ValueOverrides { // Dependency value_overrides override anything in the application metadata
			vsl := strings.Split(v, "=")
			if len(vsl) != 2 {
				return nil, fmt.Errorf("malformed value override: %v", v)
			}
			loc.ValueOverrides[vsl[0]] = vsl[1]
		}
		out[d.Name] = loc
	}
	return out, nil
}

// SingleAppRepoChartsFetcher manages the fetching of charts from a code repo that only has a single primary application.
type SingleAppRepoChartsFetcher struct{}

func (scf SingleAppRepoChartsFetcher) Fetch(
	ctx context.Context,
	rc *models.RepoConfig,
	basePath string,
	fetch func(i int, rc models.RepoConfigDependency) (*ChartLocation, error),
) (ChartLocations, error) {
	name := models.GetName(rc.Application.Repo)
	loc, err := fetch(0, models.RepoConfigDependency{Name: name, AppMetadata: rc.Application})
	if err != nil || loc == nil {
		return nil, errors.Wrap(err, "error getting primary repo chart")
	}
	out := map[string]ChartLocation{name: *loc}
	ctx, cf := context.WithTimeout(ctx, 2*time.Minute)
	defer cf()
	couts := make([]ChartLocation, rc.Dependencies.Count())
	offsetmap := make(map[int]*models.RepoConfigDependency, rc.Dependencies.Count())
	eg, _ := errgroup.WithContext(ctx)
	for i, d := range rc.Dependencies.All() {
		d := d
		i := i
		offsetmap[i] = &d
		eg.Go(func() error {
			loc, err = fetch(i+1, d)
			if err != nil || loc == nil {
				return errors.Wrapf(err, "error getting dependency chart: %v", d.Name)
			}
			couts[i] = *loc
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, errors.Wrap(err, "error fetching charts")
	}
	for i := 0; i < rc.Dependencies.Count(); i++ {
		loc := couts[i]
		d := offsetmap[i]
		for _, v := range d.ValueOverrides { // Dependency value_overrides override anything in the application metadata
			vsl := strings.Split(v, "=")
			if len(vsl) != 2 {
				return nil, fmt.Errorf("malformed value override: %v", v)
			}
			loc.ValueOverrides[vsl[0]] = vsl[1]
		}
		out[d.Name] = loc
	}
	return out, nil
}
