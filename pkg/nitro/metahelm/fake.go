package metahelm

import (
	"context"

	"github.com/dollarshaveclub/acyl/pkg/models"
	"github.com/dollarshaveclub/acyl/pkg/persistence"
	"github.com/dollarshaveclub/metahelm/pkg/metahelm"
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/helm/pkg/helm"
	rls "k8s.io/helm/pkg/proto/hapi/release"
)

// FakeInstaller satisfies the Installer interface but does nothing
type FakeInstaller struct {
	// ChartInstallFunc is called for each repo in chartLocations. Return an error to abort.
	ChartInstallFunc func(repo string, location ChartLocation) error
	ChartUpgradeFunc func(repo string, k8senv *models.KubernetesEnvironment, location ChartLocation) error
	DL               persistence.DataLayer
	HelmReleases     []string
}

var _ Installer = &FakeInstaller{}

func getReleases(cl ChartLocations) metahelm.ReleaseMap {
	releases := metahelm.ReleaseMap{} // chart title to release name
	for k := range cl {
		releases[k] = k
	}
	return releases
}

func (fi *FakeInstaller) BuildAndInstallCharts(ctx context.Context, newenv *EnvInfo, chartsLocation ChartLocations) error {
	for k, v := range chartsLocation {
		if err := fi.ChartInstallFunc(k, v); err != nil {
			return errors.Wrap(err, "install aborted")
		}
	}
	if fi.DL != nil {
		ci := ChartInstaller{dl: fi.DL}
		if err := ci.writeK8sEnvironment(ctx, newenv, "nitro-1234-"+newenv.Env.Name); err != nil {
			return err
		}
		if err := fi.DL.UpdateK8sEnvTillerAddr(newenv.Env.Name, "10.10.10.10:1234"); err != nil {
			return err
		}
		releases := getReleases(chartsLocation)
		return ci.writeReleaseNames(ctx, releases, "fake-namespace", newenv)
	}
	return nil
}

func (fi FakeInstaller) BuildAndUpgradeCharts(ctx context.Context, env *EnvInfo, k8senv *models.KubernetesEnvironment, cl ChartLocations) error {
	for k, v := range cl {
		if err := fi.ChartUpgradeFunc(k, k8senv, v); err != nil {
			return errors.Wrap(err, "upgrade aborted")
		}
	}
	if fi.DL != nil {
		ci := ChartInstaller{dl: fi.DL}
		return ci.updateReleaseRevisions(ctx, env)
	}
	return nil
}

func (fi FakeInstaller) BuildAndInstallChartsIntoExisting(ctx context.Context, newenv *EnvInfo, k8senv *models.KubernetesEnvironment, cl ChartLocations) error {
	for k, v := range cl {
		if err := fi.ChartInstallFunc(k, v); err != nil {
			return errors.Wrap(err, "install aborted")
		}
	}
	if fi.DL != nil {
		ci := ChartInstaller{dl: fi.DL}
		releases := getReleases(cl)
		return ci.writeReleaseNames(ctx, releases, "fake-namespace", newenv)
	}
	return nil
}

func (fi FakeInstaller) DeleteReleases(ctx context.Context, k8senv *models.KubernetesEnvironment) error {
	if fi.DL != nil {
		ci := ChartInstaller{dl: fi.DL}
		rels := []*rls.Release{}
		for _, r := range fi.HelmReleases {
			rels = append(rels, &rls.Release{Name: r})
		}
		ci.hcf = func(tillerNS, tillerAddr string, rcfg *rest.Config, kc kubernetes.Interface) (helm.Interface, error) {
			return &helm.FakeClient{Rels: rels}, nil
		}
		if err := ci.DeleteReleases(context.Background(), k8senv); err != nil {
			return err
		}
		return fi.DL.DeleteK8sEnv(k8senv.EnvName)
	}
	return nil
}
func (fi FakeInstaller) DeleteNamespace(ctx context.Context, k8senv *models.KubernetesEnvironment) error {
	return nil
}
