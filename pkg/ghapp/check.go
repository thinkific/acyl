package ghapp

import (
	"context"
	"net/http"

	"github.com/palantir/go-githubapp/githubapp"
)

type checksEventHandler struct {
	githubapp.ClientCreator
}

func (ch *checksEventHandler) Handles() []string {
	return []string{"check_run", "check_suite"}
}

func (ch *checksEventHandler) Handle(ctx context.Context, eventType, deliveryID string, payload []byte, w http.ResponseWriter) (int, []byte, error) {
	return 0, nil, nil
}
