package ghapp

import (
	"context"

	"github.com/palantir/go-githubapp/githubapp"
)

// checksEventHandler handles check events, stubbed out for now
type checksEventHandler struct {
	githubapp.ClientCreator
}

func (ch *checksEventHandler) Handles() []string {
	return []string{"check_run", "check_suite"}
}

func (ch *checksEventHandler) Handle(ctx context.Context, eventType, deliveryID string, payload []byte) error {
	return nil
}
