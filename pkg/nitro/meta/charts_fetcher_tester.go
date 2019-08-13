package meta

import (
	"context"
	"path"
	"strconv"
	"testing"

	"github.com/dollarshaveclub/acyl/pkg/models"
)

func fakeFetch(ctx context.Context, rc *models.RepoConfig, basePath string) func(i int, rc models.RepoConfigDependency) (*ChartLocation, error) {
	return func(i int, d models.RepoConfigDependency) (*ChartLocation, error) {
		cd := path.Join(basePath, strconv.Itoa(i), d.Name)
		return &ChartLocation{
			ChartPath:   cd,
			VarFilePath: path.Join(cd, "vars.yaml"),
		}, nil
	}
}

// TestFetchChartsMonorepo ensures that the primary applications are included in the ChartLocations result for a Monorepo.
func TestFetchChartsMonorepo(t *testing.T) {
	testCases := []struct {
		rc       *models.RepoConfig
		basePath string
		name     string
	}{{
		rc: &models.RepoConfig{
			Monorepo: models.MonorepoConfigAppMetadata{
				Enabled: true,
				Applications: []*models.RepoConfigAppMetadata{
					&models.RepoConfigAppMetadata{
						DockerfilePath: "app1",
						Image:          "app1-image",
					},
				},
			},
		},
	},
	}
	for _, tc := range testCases {
		fetcher := MonorepoChartsFetcher{}
		locs, err := fetcher.Fetch(context.Background(), tc.rc, tc.basePath, fakeFetch)
		if err != nil {
			t.Fatalf("Received an unexpected error: %v", err)
		}
		// TODO(mk): We need to be able to associate the names of the applications with something other than the repo name now!
	}
}

func TestFetchChartsStandardRepo(t *testing.T) {

}
