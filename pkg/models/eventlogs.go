package models

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type EventLog struct {
	ID             uuid.UUID          `json:"id"`
	Created        time.Time          `json:"created"`
	Updated        pq.NullTime        `json:"updated"`
	EnvName        string             `json:"env_name"`
	Repo           string             `json:"repo"`
	PullRequest    uint               `json:"pull_request"`
	WebhookPayload []byte             `json:"webhook_payload"`
	Log            []string           `json:"log"`
	Status         EventStatusSummary `json:"status"`
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

//go:generate stringer -type=NodeChartStatus
//go:generate stringer -type=EventStatus

type NodeChartStatus int

const (
	UnknownChartStatus NodeChartStatus = iota
	WaitingChartStatus
	ReadyChartStatus
	InstallingChartStatus
	DoneChartStatus
	FailedChartStatus
)

type EventStatus int

const (
	UnknownEventStatus EventStatus = iota
	CreateNewStatus
	UpdateTearDownStatus
	UpdateInPlaceStatus
	DestroyStatus
	DoneStatus
	FailedStatus
)

type EventStatusSummaryConfig struct {
	Status         EventStatus       `json:"status"`
	ProcessingTime time.Duration     `json:"processing_time"`
	Started        time.Time         `json:"started"`
	Completed      time.Time         `json:"completed"`
	RefMap         map[string]string `json:"ref_map"`
}

type EventStatusTreeNodeImage struct {
	Name      string    `json:"name"`
	Error     bool      `json:"error"`
	Completed time.Time `json:"completed"`
	Started   time.Time `json:"started"`
}

type EventStatusTreeNodeChart struct {
	Status    NodeChartStatus `json:"status"`
	Started   time.Time       `json:"started"`
	Completed time.Time       `json:"completed"`
}

type EventStatusTreeNode struct {
	Parent string                   `json:"parent"`
	Image  EventStatusTreeNodeImage `json:"image"`
	Chart  EventStatusTreeNodeChart `json:"chart"`
}

type EventStatusSummary struct {
	Config EventStatusSummaryConfig       `json:"config"`
	Tree   map[string]EventStatusTreeNode `json:"tree"`
}

// Value implements database/sql/driver Valuer interface.
func (es EventStatusSummary) Value() (driver.Value, error) {
	return json.Marshal(es)
}

// Scan implements database/sql Scanner interface.
func (es *EventStatusSummary) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("unexpected type for value: %T (wanted []byte)", value)
	}
	return json.Unmarshal(b, &es)
}

// check interfaces
var (
	_ driver.Valuer = EventStatusSummary{}
	_ sql.Scanner   = &EventStatusSummary{}
)
