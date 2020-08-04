package metahelm

import (
	"context"
	"fmt"
	"time"

	"github.com/dollarshaveclub/acyl/pkg/models"
	"github.com/dollarshaveclub/acyl/pkg/persistence"
	"github.com/dollarshaveclub/metahelm/pkg/metahelm"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// FakeInstaller satisfies the Installer interface but does nothing
type FakeInstaller struct {
	// ChartInstallFunc is called for each repo in chartLocations. Return an error to abort.
	ChartInstallFunc func(repo string, location ChartLocation) error
	ChartUpgradeFunc func(repo string, k8senv *models.KubernetesEnvironment, location ChartLocation) error
	DL               persistence.DataLayer
	KC               kubernetes.Interface
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
		ci := ChartInstaller{dl: fi.DL, kc: fi.KC}
		if err := ci.writeK8sEnvironment(ctx, newenv, "nitro-1234-"+newenv.Env.Name); err != nil {
			return err
		}
		if err := fi.DL.UpdateK8sEnvTillerAddr(ctx, newenv.Env.Name, "10.10.10.10:1234"); err != nil {
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

func (fi FakeInstaller) DeleteNamespace(ctx context.Context, k8senv *models.KubernetesEnvironment) error {
	if fi.DL != nil {
		return fi.DL.DeleteK8sEnv(ctx, k8senv.EnvName)
	}
	return nil
}

// FakeKubernetesReporter satisfies the kubernetes reporter interface but does nothing
type FakeKubernetesReporter struct {
	KC kubernetes.Interface
}

var _ KubernetesReporter = &FakeKubernetesReporter{}

func (fkr FakeKubernetesReporter) GetK8sEnvPodList(ctx context.Context, ns string) (out []K8sPod, err error) {
	pl := stubPodData(ns)
	for _, p := range pl.Items {
		age := time.Since(p.CreationTimestamp.Time)
		var nReady int
		nContainers := len(p.Spec.Containers)
		if string(p.Status.Phase) != "Completed" {
			if string(p.Status.Phase) != "Running" {
				for _, c := range p.Status.ContainerStatuses {
					if c.Ready {
						nReady += 1
					}
				}
			} else {
				nReady = nContainers
			}
		}
		out = append(out, K8sPod{
			Name:     p.Name,
			Ready:    fmt.Sprintf("%v/%v", nReady, nContainers),
			Status:   string(p.Status.Phase),
			Restarts: p.Status.ContainerStatuses[0].RestartCount,
			Age:      age,
		})
	}
	return out, nil
}

func stubPodData(ns string) *v1.PodList {
	podContainerStatus := v1.ContainerStatus{
		Name:         "foo-app",
		RestartCount: 0,
		Ready:        true,
	}
	pod := v1.Pod{
		Status: v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{podContainerStatus},
			Phase: "Running",
			PodIP: "10.0.0.1",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{Name: podContainerStatus.Name}},
		},
	}
	pod.Kind = "Pod"
	pod.Name = "foo-app-abc123"
	pod.Namespace = ns
	pod.Labels = map[string]string{"app": "foo-app"}
	pod.Status.Phase = "Running"
	pod.CreationTimestamp.Time = time.Now().UTC()

	jobContainerStatus := v1.ContainerStatus{
		Name: "foo-app-job",
		RestartCount: 0,
		Ready: true,
	}
	job := v1.Pod{
		Status: v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{jobContainerStatus},
			Phase: "Completed",
			PodIP: "10.0.0.2",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{Name: jobContainerStatus.Name}},
		},
	}
	job.Kind = "Job"
	job.Name = "foo-app-job-abc123"
	job.Namespace = ns
	job.Labels = map[string]string{"app": "foo-app-job"}
	job.CreationTimestamp.Time = time.Now().UTC()

	return &v1.PodList{
		Items: []v1.Pod{
			pod,
			job,
		},
	}
}