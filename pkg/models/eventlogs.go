package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type EventLog struct {
	ID             uuid.UUID   `json:"id"`
	Created        time.Time   `json:"created"`
	Updated        pq.NullTime `json:"updated"`
	EnvName        string      `json:"env_name"`
	Repo           string      `json:"repo"`
	PullRequest    uint        `json:"pull_request"`
	WebhookPayload []byte      `json:"webhook_payload"`
	Log            []string    `json:"log"`
}

func (el EventLog) Columns() string {
	return strings.Join([]string{"id", "created", "updated", "env_name", "repo", "pull_request", "webhook_payload", "log"}, ",")
}

func (el EventLog) InsertColumns() string {
	return strings.Join([]string{"id", "env_name", "repo", "pull_request", "webhook_payload", "log"}, ",")
}

func (el *EventLog) ScanValues() []interface{} {
	return []interface{}{&el.ID, &el.Created, &el.Updated, &el.EnvName, &el.Repo, &el.PullRequest, &el.WebhookPayload, pq.Array(&el.Log)}
}

func (el *EventLog) InsertValues() []interface{} {
	return []interface{}{&el.ID, &el.EnvName, &el.Repo, &el.PullRequest, &el.WebhookPayload, pq.Array(&el.Log)}
}

func (el EventLog) InsertParams() string {
	params := []string{}
	for i := range strings.Split(el.InsertColumns(), ",") {
		params = append(params, fmt.Sprintf("$%v", i+1))
	}
	return strings.Join(params, ", ")
}
