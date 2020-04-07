package ghapp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/dollarshaveclub/acyl/pkg/persistence"

	"github.com/dollarshaveclub/acyl/pkg/eventlogger"
	"github.com/dollarshaveclub/acyl/pkg/models"
	"github.com/google/go-github/github"
	"github.com/google/uuid"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
)

// prEventHandler is a ClientCreator that handles PR webhook events
type prEventHandler struct {
	githubapp.ClientCreator
	dl                 persistence.DataLayer
	installationID     int64
	supportedPRActions map[string]struct{}
	RRDCallback        PRCallback
}

// Handles specifies the type of events handled
func (prh *prEventHandler) Handles() []string {
	return []string{"pull_request"}
}

// Handle is called by the handler when an event is received
// The PR event handler validates the webhook, sets up the context with appropriate eventlogger and GH client factory
// and then executes the callback
func (prh *prEventHandler) Handle(ctx context.Context, eventType, deliveryID string, payload []byte) error {
	if eventType != "pull_request" {
		return errors.New("not a pull request event")
	}

	var event github.PullRequestEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return errors.Wrap(err, "error unmarshaling event")
	}

	did, err := uuid.Parse(deliveryID)
	if err != nil {
		return errors.Wrap(err, "malformed delivery id")
	}

	rrd := models.RepoRevisionData{
		BaseBranch:   event.GetPullRequest().GetBase().GetRef(),
		BaseSHA:      event.GetPullRequest().GetBase().GetSHA(),
		PullRequest:  uint(event.GetPullRequest().GetNumber()),
		Repo:         event.GetPullRequest().GetBase().GetRepo().GetFullName(),
		SourceBranch: event.GetPullRequest().GetHead().GetRef(),
		SourceRef:    event.GetPullRequest().GetHead().GetRef(),
		SourceSHA:    event.GetPullRequest().GetHead().GetSHA(),
		User:         event.GetPullRequest().GetUser().GetLogin(),
		IsFork:       event.GetPullRequest().GetHead().GetRepo().GetFork(),
	}
	action := event.GetAction()

	_, ok := prh.supportedPRActions[action]
	if !ok {
		githubapp.SetResponder(ctx, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("action not relevant: " + action))
		})
		return nil
	}

	elogger, err := prh.getlogger(payload, did, rrd.Repo, rrd.PullRequest)
	if err != nil {
		return errors.Wrap(err, "error getting event logger")
	}

	// Create a new independent context for the async action
	ctx = eventlogger.NewEventLoggerContext(ctx, elogger)

	// Add the GitHub app client factory to the context
	ctx = NewGitHubClientContext(ctx, prh.installationID, prh)

	err = prh.RRDCallback(ctx, action, rrd)
	githubapp.SetResponder(ctx, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		if err == nil {
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte(fmt.Sprintf(`{"event_log_id": "%v"}`, eventlogger.GetLogger(ctx).ID.String())))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf(`{"error_details":"%v"}`, err)))
	})
	return err
}

func (prh *prEventHandler) getlogger(body []byte, deliveryID uuid.UUID, repo string, pr uint) (*eventlogger.Logger, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, errors.Wrap(err, "error getting random UUID")
	}
	logger := &eventlogger.Logger{
		ID:         id,
		DeliveryID: deliveryID,
		DL:         prh.dl,
		Sink:       os.Stdout,
	}
	if err := logger.Init(body, repo, pr); err != nil {
		return nil, errors.Wrap(err, "error initializing event logger")
	}
	return logger, nil
}
