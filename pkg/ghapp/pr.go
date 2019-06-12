package ghapp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/dollarshaveclub/acyl/pkg/persistence"

	"github.com/dollarshaveclub/acyl/pkg/models"

	"github.com/dollarshaveclub/acyl/pkg/eventlogger"
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
	wg sync.WaitGroup
	es spawner.EnvironmentSpawner
	dl persistence.DataLayer
}

// Handles specifies the type of events handled
func (prh *prEventHandler) Handles() []string {
	return []string{"pull_request"}
}

// Handle is called by the handler when an event is received
// The PR event handler validates the incoming webhook and
func (prh *prEventHandler) Handle(syncctx context.Context, eventType, deliveryID string, payload []byte, w http.ResponseWriter) (status int, body []byte, err error) {
	w.Header().Add("Content-Type", "application/json")

	rootSpan := tracer.StartSpan("github_app_pr_event_handler")

	errbody := func(msg string) []byte { return []byte(`{"error":"` + msg + `"}`) }

	// We need to set this sampling priority tag in order to allow distributed tracing to work.
	// This only needs to be done for the root level span as the value is propagated down.
	// https://docs.datadoghq.com/tracing/getting_further/trace_sampling_and_storage/#priority-sampling-for-distributed-tracing
	rootSpan.SetTag(ext.SamplingPriority, ext.PriorityUserKeep)

	var event github.PullRequestEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return http.StatusBadRequest, errbody("could not unmarshal payload into pull request event"), errors.Wrap(err, "error unmarshaling event")
	}

	elogger, err := prh.getlogger(payload, event.GetRepo().GetName(), uint(event.GetPullRequest().GetNumber()))
	if err != nil {
		return http.StatusInternalServerError, errbody("could not get eventlogger"), errors.Wrap(err, "error getting event logger")
	}

	// Create a new context for the async action
	log := elogger.Printf
	ctx := eventlogger.NewEventLoggerContext(context.Background(), elogger)

	rrd := models.RepoRevisionData{
		BaseBranch:   event.GetPullRequest().GetBase().GetLabel(),
		BaseSHA:      event.GetPullRequest().GetBase().GetSHA(),
		PullRequest:  uint(event.GetPullRequest().GetNumber()),
		Repo:         event.GetRepo().GetName(),
		SourceBranch: event.GetPullRequest().GetHead().GetLabel(),
		SourceRef:    event.GetPullRequest().GetHead().GetRef(),
		SourceSHA:    event.GetPullRequest().GetHead().GetSHA(),
		User:         event.GetPullRequest().GetUser().GetName(),
	}

	rootSpan.SetTag("base_branch", rrd.BaseBranch)
	rootSpan.SetTag("base_sha", rrd.BaseSHA)
	rootSpan.SetTag("pull_request", rrd.PullRequest)
	rootSpan.SetTag("repo", rrd.Repo)
	rootSpan.SetTag("source_branch", rrd.SourceBranch)
	rootSpan.SetTag("source_ref", rrd.SourceRef)
	rootSpan.SetTag("source_sha", rrd.SourceSHA)
	rootSpan.SetTag("user", rrd.User)

	ctx = tracer.ContextWithSpan(ctx, rootSpan)

	log("starting async processing for %v", event.GetAction())

	async := func(f func(ctx context.Context) error) {
		defer prh.wg.Done()
		ctx, cf := context.WithTimeout(ctx, MaxAsyncActionTimeout)
		defer cf() // guarantee that any goroutines created with the ctx are cancelled
		err := f(ctx)
		if err != nil {
			log("finished processing %v with error: %v", event.GetAction(), err)
			rootSpan.Finish(tracer.WithError(err))
			return
		}
		rootSpan.Finish()
		log("success processing %v event; done", event.GetAction())
	}

	switch event.GetAction() {
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
		rootSpan.Finish()
		return http.StatusOK, []byte(`event not supported (ignored)`), nil
	}

	rootSpan.Finish()
	w.Header().Add("Content-Type", "application/json")
	return http.StatusAccepted, []byte(fmt.Sprintf(`{"event_log_id": "%v"}`, elogger.ID.String())), nil
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
