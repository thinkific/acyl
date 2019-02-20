package spawner

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/dollarshaveclub/acyl/pkg/config"
	"github.com/dollarshaveclub/acyl/pkg/persistence"
	"github.com/dollarshaveclub/acyl/pkg/spawner/pbaminoapi"
	newrelic "github.com/newrelic/go-agent"
)

const (
	createEnvironmentTimeout = 40 * time.Minute
	aminoStatusRefreshDelay  = 5 * time.Second
)

// AminoBackend provides a Acyl backend that uses Amino.
type AminoBackend struct {
	aminoClient pbaminoapi.AminoAPIClient
	dataLayer   persistence.DataLayer
	logger      *log.Logger
	spawner     EnvironmentSpawner
	metrics     MetricsCollector
	aminoConfig *config.AminoConfig
}

var _ AcylBackend = &AminoBackend{}

// CreateEnvironment creates a QA environment using the Amino API.
func (a *AminoBackend) CreateEnvironment(ctx context.Context, qa *QAEnvironment, qat *QAType) (string, error) {
	if qa == nil {
		return "", fmt.Errorf("qa is nil")
	}

	qaName := qa.Name

	txn, ok := GetNRTxnFromContext(ctx)
	if !ok {
		return "", fmt.Errorf("context is missing New Relic transaction")
	}

	nrseg := newrelic.StartSegment(txn, "AminoBackend.CreateEnvironment.Setup")

	// Map the Github refs that we want deployed to a mapping of
	// Kubernetes resources to Docker image tags.
	deploymentTags := make(map[string]string)
	jobTags := make(map[string]string)
	var chartVariables []*pbaminoapi.ChartVariableOverride
	if a.aminoConfig.AminoDeploymentToRepo != nil {
		for deployment, repo := range a.aminoConfig.AminoDeploymentToRepo {
			if ref, ok := qa.CommitSHAMap[repo]; ok {
				deploymentTags[deployment] = ref
			}
		}
	}
	if a.aminoConfig.AminoJobToRepo != nil {
		for job, repo := range a.aminoConfig.AminoJobToRepo {
			if ref, ok := qa.CommitSHAMap[repo]; ok {
				jobTags[job] = ref
			}
		}
	}
	if a.aminoConfig.HelmChartToRepo != nil {
		for chart, repo := range a.aminoConfig.HelmChartToRepo {
			if ref, ok := qa.CommitSHAMap[repo]; ok {
				chartVariables = append(chartVariables, &pbaminoapi.ChartVariableOverride{
					ChartName:    chart,
					VariableName: "image.tag",
					Value:        ref,
				})
			}
		}
	}

	nrseg.End()
	nrseg = newrelic.StartSegment(txn, "AminoBackend.CreateEnvironment.CreateRPC")

	a.logger.Printf("creating Amino environment %s with deployment tags: %v", qaName, deploymentTags)

	// Queue up the deployment of the environment.
	createCtx := context.Background() // CreateEnvironment gets its own context so it won't be cancelled
	response, err := a.aminoClient.CreateEnvironment(createCtx, &pbaminoapi.CreateEnvironmentRequest{
		Name:                           qaName,
		DeploymentTags:                 deploymentTags,
		JobTags:                        jobTags,
		ChartVariableOverrides:         chartVariables,
		GithubProjectDirectoryOverride: qat.EnvType,

		// TODO: Refactor this to be configured in each
		// project's codebase.
		GithubProjectName: "amino-configs",
		GithubProjectRef:  "master",
	})
	if err != nil {
		return "", fmt.Errorf("error creating environment with Amino: %s", err)
	}
	if err := a.dataLayer.SetAminoEnvironmentID(qaName, int(response.Id)); err != nil {
		return "", fmt.Errorf("error setting Amino environment ID: %s", err)
	}
	envID := response.Id

	if ctx.Err() == context.Canceled {
		if err := a.dataLayer.AddEvent(qa.Name, "spawn preempted by new action"); err != nil {
			a.logger.Printf("error storing Amino event: %s", err)
		}
		return "", ctx.Err()
	}

	nrseg.End()
	nrseg = newrelic.StartSegment(txn, "AminoBackend.CreateEnvironment.WaitForEnv")
	defer nrseg.End()

	ctxw, cancelFuncWait := context.WithDeadline(
		ctx,
		time.Now().Add(createEnvironmentTimeout),
	)
	defer cancelFuncWait()

	if err := a.waitForEnvironment(ctxw, int(envID), qa.Name); err != nil {
		ctxs, cancelFuncStatus := context.WithDeadline(
			ctx,
			time.Now().Add(1*time.Minute),
		)
		defer cancelFuncStatus()

		if grpc.Code(err) == codes.Canceled || err == context.Canceled {
			if err := a.dataLayer.AddEvent(qa.Name, "spawn preempted by new action"); err != nil {
				a.logger.Printf("error storing Amino event: %s", err)
			}

			a.logger.Printf("Amino wait for environment canceled")

			return "", err
		}

		response, err := a.aminoClient.EnvironmentStatus(ctxs, &pbaminoapi.EnvironmentStatusRequest{
			Id: int64(envID),
		})
		if err != nil {

			if grpc.Code(err) == codes.Canceled || err == context.Canceled {
				if err := a.dataLayer.AddEvent(qa.Name, "spawn preempted by new action"); err != nil {
					a.logger.Printf("error storing Amino event: %s", err)
				}

				a.logger.Printf("Amino environment status canceled")

				return "", err
			}

			err = fmt.Errorf("error getting Amino environment status: %s", err)
			txn.NoticeError(err)
			a.logger.Printf(err.Error())
		}

		// The environment was destroyed through
		// external means.
		if response.GetState() == "destroyed" {
			return strconv.Itoa(int(envID)), nil
		}

		messageParts := []string{}
		if ns := response.GetKubernetesNamespace(); ns != "" {
			messageParts = append(messageParts, fmt.Sprintf("Kubernetes namespace: %s", ns))
		}
		if err := response.GetError(); err != "" {
			messageParts = append(messageParts, fmt.Sprintf("aminoClient.EnvironmentStatus response Error: %s", err))
		}
		if ud := response.GetUnavailableDeployments(); len(ud) > 0 {
			messageParts = append(messageParts, fmt.Sprintf("Unavailable deployments: %v", ud))
		}
		if aj := response.GetActiveJobs(); len(aj) > 0 {
			messageParts = append(messageParts, fmt.Sprintf("Unfinished jobs: %v", aj))
		}
		if fj := response.GetFailedJobs(); len(fj) > 0 {
			messageParts = append(messageParts, fmt.Sprintf("Failed jobs: %v", fj))
		}
		if len(messageParts) == 0 {
			messageParts = []string{"Unknown reason"}
		}

		go a.metrics.AminoDeployTimedOut(qaName, qa.Repo, qa.SourceBranch)

		txn.NoticeError(fmt.Errorf("amino timeout: %v", strings.Join(messageParts, ": ")))

		if err := a.spawner.Failure(context.Background(), qaName, strings.Join(messageParts, "\n")); err != nil {
			err = fmt.Errorf("error failing Amino environment: %s", err)
			txn.NoticeError(err)
			a.logger.Printf(err.Error())
		}
		return strconv.Itoa(int(envID)), nil
	}

	if err := a.spawner.Success(context.Background(), qaName); err != nil {
		err = fmt.Errorf("error marking Amino environment as successful: %s", err)
		txn.NoticeError(err)
		a.logger.Printf(err.Error())
	}

	return strconv.Itoa(int(envID)), nil
}

// waitForEnvironment periodically polls Amino to determine the status
// of the deploy, stopping once the deploy has succeeded or timed out.
func (a *AminoBackend) waitForEnvironment(ctx context.Context, envID int, name string) error {
	successfulDeploys := make(map[string]bool)
	successfulJobs := make(map[string]bool)
	var lastResponse *pbaminoapi.EnvironmentStatusResponse

	for {
		if ctx.Err() == context.Canceled {
			return ctx.Err()
		}

		a.logger.Printf("retrieving status for Amino(%d) environment: %s", envID, name)

		// If we meet a deadline, then log the pending events.
		select {
		case <-ctx.Done():
			if err := a.dataLayer.AddEvent(name, "environment deploy timed out"); err != nil {
				a.logger.Printf("error storing Amino event: %s", err)
			}
			if lastResponse != nil {
				if err := a.dataLayer.AddEvent(name, fmt.Sprintf("pending deployments: %v", lastResponse.UnavailableDeployments)); err != nil {
					a.logger.Printf("error storing Amino event: %s", err)
				}
				if err := a.dataLayer.AddEvent(name, fmt.Sprintf("pending jobs: %v", lastResponse.ActiveJobs)); err != nil {
					a.logger.Printf("error storing Amino event: %s", err)
				}
			}
			return errors.New("Amino environment deploy timed out")
		default:
		}

		response, err := a.aminoClient.EnvironmentStatus(ctx, &pbaminoapi.EnvironmentStatusRequest{
			Id: int64(envID),
		})
		if err != nil {
			if grpc.Code(err) == codes.Canceled {
				return err
			}

			a.logger.Printf("error getting Amino environment status: %s", err)
			time.Sleep(aminoStatusRefreshDelay)
			continue
		}

		switch response.GetState() {
		case "spawned":
			// Environment is still spawning
			time.Sleep(aminoStatusRefreshDelay)
			continue
		case "destroyed":
			return errors.New("error Amino environment destroyed, expected spawning or created")
		case "errored":
			return fmt.Errorf("encountered Amino error while spawning: %s", response.GetError())
		case "created":
			// Environment is ready
		default:
			return fmt.Errorf("invalid Amino environment state: %s", response.GetState())
		}
		lastResponse = response

		// Save namespace. It's immediately available.
		namespace := response.GetKubernetesNamespace()
		if err := a.dataLayer.SetAminoKubernetesNamespace(name, namespace); err != nil {
			a.logger.Printf("error setting Amino Kubernetes namespace: %s", err)
		}

		// Log events for all new successful deploys and jobs.
		for _, deploy := range response.AvailableDeployments {
			if _, ok := successfulDeploys[deploy]; !ok {
				if err := a.dataLayer.AddEvent(name, fmt.Sprintf("deploy completed: %s", deploy)); err != nil {
					a.logger.Printf("error storing Amino event: %s", err)
				}
			}
			successfulDeploys[deploy] = true
		}
		for _, job := range response.CompletedJobs {
			if _, ok := successfulJobs[job]; !ok {
				if err := a.dataLayer.AddEvent(name, fmt.Sprintf("job completed: %s", job)); err != nil {
					a.logger.Printf("error storing Amino event: %s", err)
				}
			}
			successfulJobs[job] = true
		}

		// The environment is up and running once all jobs and
		// deployments are available.
		if len(response.AllDeployments) == len(response.AvailableDeployments) &&
			len(response.AllJobs) == len(response.CompletedJobs) {
			if err := a.dataLayer.AddEvent(name, "environment deploy completed"); err != nil {
				a.logger.Printf("error storing Amino event: %s", err)
			}

			serviceToExposedPorts := response.GetServiceToExposedPort()
			if err := a.dataLayer.SetAminoServiceToPort(name, serviceToExposedPorts); err != nil {
				a.logger.Printf("error setting Amino service metadata: %s", err)
			}

			return nil
		}

		time.Sleep(aminoStatusRefreshDelay)
	}
}

// DestroyEnvironment destroys a Amino environment.
func (a *AminoBackend) DestroyEnvironment(ctx context.Context, qae *QAEnvironment, dns bool) error {
	_, err := a.aminoClient.DestroyEnvironment(
		context.TODO(),
		&pbaminoapi.DestroyEnvironmentRequest{
			Id:    int64(qae.AminoEnvironmentID),
			RmDns: dns,
		},
	)
	return err
}

// ManagesDNS returns whether Amino manages DNS.
func (a *AminoBackend) ManagesDNS() bool {
	return true
}
