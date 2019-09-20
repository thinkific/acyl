package ghapp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/dollarshaveclub/acyl/pkg/eventlogger"
	ghactx "github.com/dollarshaveclub/acyl/pkg/ghapp/context"
	"github.com/dollarshaveclub/acyl/pkg/ghclient"
	"github.com/dollarshaveclub/acyl/pkg/models"
	"github.com/dollarshaveclub/acyl/pkg/persistence"
	"github.com/dollarshaveclub/acyl/pkg/spawner"

	"github.com/google/go-github/github"
	"github.com/google/uuid"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/ext"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

// MaxAsyncActionTimeout is the maximum amount of time an asynchronous action can take before it's forcibly cancelled
var MaxAsyncActionTimeout = 30 * time.Minute

// prEventHandler is a ClientCreator that handles PR webhook events
type prEventHandler struct {
	githubapp.ClientCreator
	typePath string
	wg       *sync.WaitGroup
	es       spawner.EnvironmentSpawner
	dl       persistence.DataLayer
	rc       ghclient.RepoClient
}

// Handles specifies the type of events handled
func (prh *prEventHandler) Handles() []string {
	return []string{"pull_request"}
}

// ProcessEvent processes an incoming payload and returns the context loaded with an eventlogger and GitHup app client, the parsed event data and action string or error
func (prh *prEventHandler) ProcessEvent(ctx context.Context, payload []byte) (_ context.Context, rrd models.RepoRevisionData, action string, err error) {
	var event github.PullRequestEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return ctx, models.RepoRevisionData{}, "", errors.Wrap(err, "error unmarshaling event")
	}

	rrd = models.RepoRevisionData{
		BaseBranch:   event.GetPullRequest().GetBase().GetRef(),
		BaseSHA:      event.GetPullRequest().GetBase().GetSHA(),
		PullRequest:  uint(event.GetPullRequest().GetNumber()),
		Repo:         event.GetRepo().GetFullName(),
		SourceBranch: event.GetPullRequest().GetHead().GetRef(),
		SourceRef:    event.GetPullRequest().GetHead().GetRef(),
		SourceSHA:    event.GetPullRequest().GetHead().GetSHA(),
		User:         event.GetPullRequest().GetUser().GetLogin(),
	}
	action = event.GetAction()

	elogger, err := prh.getlogger(payload, rrd.Repo, rrd.PullRequest)
	if err != nil {
		return ctx, rrd, action, errors.Wrap(err, "error getting event logger")
	}

	// Create a new independent context for the async action
	ctx = eventlogger.NewEventLoggerContext(ctx, elogger)

	// Add the GitHub app client factory to the context
	ctx, err = ghactx.NewGitHubClientContext(ctx, prh)
	if err != nil {
		return ctx, rrd, action, errors.Wrap(err, "error getting GitHub app client")
	}
	return ctx, rrd, action, nil
}

// Handle is called by the handler when an event is received
// The PR event handler validates the incoming webhook and
func (prh *prEventHandler) Handle(syncctx context.Context, eventType, deliveryID string, payload []byte, w http.ResponseWriter) (status int, body []byte, err error) {
	rootSpan := tracer.StartSpan("github_app_pr_event_handler")

	// We need to set this sampling priority tag in order to allow distributed tracing to work.
	// This only needs to be done for the root level span as the value is propagated down.
	// https://docs.datadoghq.com/tracing/getting_further/trace_sampling_and_storage/#priority-sampling-for-distributed-tracing
	rootSpan.SetTag(ext.SamplingPriority, ext.PriorityUserKeep)

	ctx, rrd, action, err := prh.ProcessEvent(syncctx, payload)
	if err != nil {
		if strings.HasPrefix(err.Error(), "error unmarshaling event") {
			return http.StatusBadRequest, []byte{}, errors.Wrap(err, "error processing event")
		}
		return http.StatusInternalServerError, []byte{}, errors.Wrap(err, "error processing event")
	}
	log := eventlogger.GetLogger(ctx).Printf

	rootSpan.SetTag("base_branch", rrd.BaseBranch)
	rootSpan.SetTag("base_sha", rrd.BaseSHA)
	rootSpan.SetTag("pull_request", rrd.PullRequest)
	rootSpan.SetTag("repo", rrd.Repo)
	rootSpan.SetTag("source_branch", rrd.SourceBranch)
	rootSpan.SetTag("source_ref", rrd.SourceRef)
	rootSpan.SetTag("source_sha", rrd.SourceSHA)
	rootSpan.SetTag("user", rrd.User)

	ctx = tracer.ContextWithSpan(ctx, rootSpan)

	checkRelevancy := func(ctx context.Context) bool {
		cfg, err := prh.rc.GetFileContents(ctx, rrd.Repo, prh.typePath, rrd.SourceRef)
		if err != nil {
			log("error getting acyl.yml: %v", err)
			return false
		}
		qat := models.QAType{}
		if err := qat.FromYAML(cfg); err != nil {
			log("error unmarshaling acyl.yml: %v", err)
			return false
		}
		if qat.TargetBranch != "" {
			return qat.TargetBranch == rrd.BaseBranch
		}
		for _, tb := range qat.TargetBranches {
			if tb == rrd.BaseBranch {
				log("PR base branch found in target branches: %v", tb)
				return true
			}
		}
		log("PR base branch not found in target branches: %v", rrd.BaseBranch)
		return false
	}

	async := func(f func(ctx context.Context) error) {
		defer prh.wg.Done()
		log("starting async processing for %v", action)
		ctx, cf := context.WithTimeout(ctx, MaxAsyncActionTimeout)
		defer cf() // guarantee that any goroutines created with the ctx are cancelled
		// make sure that this is a relevant event
		if !checkRelevancy(ctx) {
			log("event not relevant; ending processing")
			return
		}
		err := f(ctx)
		if err != nil {
			log("finished processing %v with error: %v", action, err)
			rootSpan.Finish(tracer.WithError(err))
			return
		}
		rootSpan.Finish()
		log("success processing %v event; done", action)
	}

	switch action {
	case "opened":
		fallthrough
	case "reopened":
		prh.wg.Add(1)
		go async(func(ctx context.Context) error { _, err := prh.es.Create(ctx, rrd); return err })
	case "synchronize":
		prh.wg.Add(1)
		go async(func(ctx context.Context) error { _, err := prh.es.Update(ctx, rrd); return err })
	case "closed":
		prh.wg.Add(1)
		go async(func(ctx context.Context) error { return prh.es.Destroy(ctx, rrd, models.DestroyApiRequest) })
	default:
		log("unsupported action: %v", action)
		rootSpan.Finish()
		return http.StatusOK, []byte(`event not supported (` + action + `); ignored`), nil
	}

	rootSpan.Finish()
	w.Header().Add("Content-Type", "application/json")
	return http.StatusAccepted, []byte(fmt.Sprintf(`{"event_log_id": "%v"}`, eventlogger.GetLogger(ctx).ID.String())), nil
}

func (prh *prEventHandler) getlogger(body []byte, repo string, pr uint) (*eventlogger.Logger, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, errors.Wrap(err, "error getting random UUID")
	}
	logger := &eventlogger.Logger{
		ID:   id,
		DL:   prh.dl,
		Sink: os.Stdout,
	}
	if err := logger.Init(body, repo, pr); err != nil {
		return nil, errors.Wrap(err, "error initializing event logger")
	}
	return logger, nil
}
