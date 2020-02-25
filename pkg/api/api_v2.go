package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/dollarshaveclub/acyl/pkg/config"
	"github.com/dollarshaveclub/acyl/pkg/ghevent"
	"github.com/dollarshaveclub/acyl/pkg/models"
	"github.com/dollarshaveclub/acyl/pkg/persistence"
	"github.com/dollarshaveclub/acyl/pkg/spawner"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	muxtrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/gorilla/mux"
)

// API output schema
type v2QAEnvironment struct {
	Name                     string                      `json:"name"`
	Created                  time.Time                   `json:"created"`
	RawEvents                []string                    `json:"-"`
	Events                   []models.QAEnvironmentEvent `json:"events"`
	Hostname                 string                      `json:"hostname"`
	QAType                   string                      `json:"qa_type"`
	User                     string                      `json:"user"`
	Repo                     string                      `json:"repo"`
	PullRequest              uint                        `json:"pull_request"`
	SourceSHA                string                      `json:"source_sha"`
	BaseSHA                  string                      `json:"base_sha"`
	SourceBranch             string                      `json:"source_branch"`
	BaseBranch               string                      `json:"base_branch"`
	SourceRef                string                      `json:"source_ref"`
	RawStatus                string                      `json:"status"`
	RefMap                   models.RefMap               `json:"ref_map"`
	CommitSHAMap             models.RefMap               `json:"commit_sha_map"`
	AminoServiceToPort       map[string]int64            `json:"amino_service_to_port"`
	AminoKubernetesNamespace string                      `json:"amino_kubernetes_namespace"`
	AminoEnvironmentID       int                         `json:"amino_environment_id"`
}

func v2QAEnvironmentFromQAEnvironment(qae *models.QAEnvironment) *v2QAEnvironment {
	return &v2QAEnvironment{
		Name:                     qae.Name,
		Created:                  qae.Created,
		RawEvents:                qae.RawEvents,
		Events:                   qae.Events,
		Hostname:                 qae.Hostname,
		QAType:                   qae.QAType,
		User:                     qae.User,
		Repo:                     qae.Repo,
		PullRequest:              qae.PullRequest,
		SourceSHA:                qae.SourceSHA,
		BaseSHA:                  qae.BaseSHA,
		SourceBranch:             qae.SourceBranch,
		BaseBranch:               qae.BaseBranch,
		SourceRef:                qae.SourceRef,
		RawStatus:                qae.RawStatus,
		RefMap:                   qae.RefMap,
		CommitSHAMap:             qae.CommitSHAMap,
		AminoServiceToPort:       qae.AminoServiceToPort,
		AminoKubernetesNamespace: qae.AminoKubernetesNamespace,
		AminoEnvironmentID:       qae.AminoEnvironmentID,
	}
}

type v2EventLog struct {
	ID             uuid.UUID   `json:"id"`
	Created        time.Time   `json:"created"`
	Updated        pq.NullTime `json:"updated"`
	EnvName        string      `json:"env_name"`
	Repo           string      `json:"repo"`
	PullRequest    uint        `json:"pull_request"`
	WebhookPayload []byte      `json:"webhook_payload"`
	Log            []string    `json:"log"`
}

func v2EventLogFromEventLog(el *models.EventLog) *v2EventLog {
	return &v2EventLog{
		ID:             el.ID,
		Created:        el.Created,
		Updated:        el.Updated,
		EnvName:        el.EnvName,
		Repo:           el.Repo,
		PullRequest:    el.PullRequest,
		WebhookPayload: el.WebhookPayload,
		Log:            el.Log,
	}
}

func timeOrNil(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func statusSummaryType(t models.EventStatusType) string {
	switch t {
	case models.UnknownEventStatusType:
		return "unknown"
	case models.CreateEvent:
		return "create"
	case models.UpdateEvent:
		return "update"
	case models.DestroyEvent:
		return "destroy"
	default:
		return "default"
	}
}

func statusSummaryStatus(s models.EventStatus) string {
	switch s {
	case models.UnknownEventStatus:
		return "unknown"
	case models.PendingStatus:
		return "pending"
	case models.DoneStatus:
		return "done"
	case models.FailedStatus:
		return "failed"
	default:
		return "default"
	}
}

func statusNodeChartStatus(n models.NodeChartStatus) string {
	switch n {
	case models.UnknownChartStatus:
		return "unknown"
	case models.WaitingChartStatus:
		return "waiting"
	case models.InstallingChartStatus:
		return "installing"
	case models.UpgradingChartStatus:
		return "upgrading"
	case models.DoneChartStatus:
		return "done"
	case models.FailedChartStatus:
		return "failed"
	default:
		return "default"
	}
}

type v2EventStatusSummaryConfig struct {
	Type           string            `json:"type"`
	Status         string            `json:"status"`
	EnvName        string            `json:"env_name"`
	TriggeringRepo string            `json:"triggering_repo"`
	PullRequest    uint              `json:"pull_request"`
	GitHubUser     string            `json:"github_user"`
	Branch         string            `json:"branch"`
	Revision       string            `json:"revision"`
	ProcessingTime string            `json:"processing_time"`
	Started        *time.Time        `json:"started"`
	Completed      *time.Time        `json:"completed"`
	RefMap         map[string]string `json:"ref_map"`
}

type v2EventStatusTreeNodeImage struct {
	Name      string     `json:"name"`
	Error     bool       `json:"error"`
	Completed *time.Time `json:"completed"`
	Started   *time.Time `json:"started"`
}

type v2EventStatusTreeNodeChart struct {
	Status    string     `json:"status"`
	Started   *time.Time `json:"started"`
	Completed *time.Time `json:"completed"`
}

func statusImageOrNil(image models.EventStatusTreeNodeImage) *v2EventStatusTreeNodeImage {
	if image.Name == "" {
		return nil
	}
	return &v2EventStatusTreeNodeImage{
		Name:      image.Name,
		Error:     image.Error,
		Completed: timeOrNil(image.Completed),
		Started:   timeOrNil(image.Started),
	}
}

type v2EventStatusTreeNode struct {
	Parent string                      `json:"parent"`
	Image  *v2EventStatusTreeNodeImage `json:"image"`
	Chart  v2EventStatusTreeNodeChart  `json:"chart"`
}

type v2EventStatusSummary struct {
	Config v2EventStatusSummaryConfig       `json:"config"`
	Tree   map[string]v2EventStatusTreeNode `json:"tree"`
}

func v2EventStatusTreeFromTree(tree map[string]models.EventStatusTreeNode) map[string]v2EventStatusTreeNode {
	out := make(map[string]v2EventStatusTreeNode, len(tree))
	for k, v := range tree {
		out[k] = v2EventStatusTreeNode{
			Parent: v.Parent,
			Image:  statusImageOrNil(v.Image),
			Chart: v2EventStatusTreeNodeChart{
				Status:    statusNodeChartStatus(v.Chart.Status),
				Completed: timeOrNil(v.Chart.Completed),
				Started:   timeOrNil(v.Chart.Started),
			},
		}
	}
	return out
}

func v2EventStatusSummaryFromEventStatusSummary(sum *models.EventStatusSummary) *v2EventStatusSummary {
	return &v2EventStatusSummary{
		Config: v2EventStatusSummaryConfig{
			Type:           statusSummaryType(sum.Config.Type),
			Status:         statusSummaryStatus(sum.Config.Status),
			EnvName:        sum.Config.EnvName,
			TriggeringRepo: sum.Config.TriggeringRepo,
			PullRequest:    sum.Config.PullRequest,
			GitHubUser:     sum.Config.GitHubUser,
			Branch:         sum.Config.Branch,
			Revision:       sum.Config.Revision,
			ProcessingTime: sum.Config.ProcessingTime.String(),
			Started:        timeOrNil(sum.Config.Started),
			Completed:      timeOrNil(sum.Config.Completed),
			RefMap:         sum.Config.RefMap,
		},
		Tree: v2EventStatusTreeFromTree(sum.Tree),
	}
}

type v2api struct {
	apiBase
	dl persistence.DataLayer
	ge *ghevent.GitHubEventWebhook
	es spawner.EnvironmentSpawner
	sc config.ServerConfig
}

func newV2API(dl persistence.DataLayer, ge *ghevent.GitHubEventWebhook, es spawner.EnvironmentSpawner, sc config.ServerConfig, logger *log.Logger) (*v2api, error) {
	return &v2api{
		apiBase: apiBase{
			logger: logger,
		},
		dl: dl,
		ge: ge,
		es: es,
		sc: sc,
	}, nil
}

func (api *v2api) register(r *muxtrace.Router) error {
	if r == nil {
		return fmt.Errorf("router is nil")
	}
	// v2 routes
	r.HandleFunc("/v2/envs/_search", middlewareChain(api.envSearchHandler, authMiddleware.authRequest)).Methods("GET")
	r.HandleFunc("/v2/envs/{name}", middlewareChain(api.envDetailHandler, authMiddleware.authRequest)).Methods("GET")
	r.HandleFunc("/v2/eventlog/{id}", middlewareChain(api.eventLogHandler, authMiddleware.authRequest)).Methods("GET")
	r.HandleFunc("/v2/event/{id}/status", middlewareChain(api.eventStatusHandler)).Methods("GET")
	r.HandleFunc("/v2/health-check", middlewareChain(api.healthCheck)).Methods("GET")
	return nil
}

func (api *v2api) marshalQAEnvironments(qas []models.QAEnvironment, w http.ResponseWriter) {
	w.Header().Add("Content-Type", "application/json")
	output := []v2QAEnvironment{}
	for _, e := range qas {
		output = append(output, *v2QAEnvironmentFromQAEnvironment(&e))
	}
	j, err := json.Marshal(output)
	if err != nil {
		api.internalError(w, fmt.Errorf("error marshaling environments: %v", err))
		return
	}
	w.Write(j)
}

func (api *v2api) envDetailHandler(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	qa, err := api.dl.GetQAEnvironmentConsistently(r.Context(), name)
	if err != nil {
		api.internalError(w, fmt.Errorf("error getting environment: %v", err))
		return
	}
	if qa == nil {
		api.notfoundError(w)
		return
	}

	output := v2QAEnvironmentFromQAEnvironment(qa)
	j, err := json.Marshal(output)
	if err != nil {
		api.internalError(w, fmt.Errorf("error marshaling environment: %v", err))
		return
	}
	w.Header().Add("Content-Type", "application/json")
	w.Write(j)
}

func (api *v2api) envSearchHandler(w http.ResponseWriter, r *http.Request) {
	qvars := r.URL.Query()
	if _, ok := qvars["pr"]; ok {
		if _, ok := qvars["repo"]; !ok {
			api.badRequestError(w, fmt.Errorf("search by PR requires repo name"))
			return
		}
	}
	if status, ok := qvars["status"]; ok {
		if status[0] == "destroyed" && len(qvars) == 1 {
			api.badRequestError(w, fmt.Errorf("'destroyed' status searches require at least one other search parameter"))
			return
		}
	}
	if _, ok := qvars["tracking_ref"]; ok {
		if _, ok := qvars["repo"]; !ok {
			api.badRequestError(w, fmt.Errorf("search by tracking_ref requires repo name"))
			return
		}
	}
	if len(qvars) == 0 {
		api.badRequestError(w, fmt.Errorf("at least one search parameter is required"))
		return
	}
	ops := models.EnvSearchParameters{}

	for k, vs := range qvars {
		if len(vs) != 1 {
			api.badRequestError(w, fmt.Errorf("unexpected value for %v: %v", k, vs))
			return
		}
		v := vs[0]
		switch k {
		case "repo":
			ops.Repo = v
		case "pr":
			pr, err := strconv.Atoi(v)
			if err != nil || pr < 1 {
				api.badRequestError(w, fmt.Errorf("bad PR number"))
				return
			}
			ops.Pr = uint(pr)
		case "source_sha":
			ops.SourceSHA = v
		case "source_branch":
			ops.SourceBranch = v
		case "user":
			ops.User = v
		case "status":
			s, err := models.EnvironmentStatusFromString(v)
			if err != nil {
				api.badRequestError(w, fmt.Errorf("unknown status"))
				return
			}
			ops.Status = s
		case "tracking_ref":
			ops.TrackingRef = v
		}
	}
	qas, err := api.dl.Search(r.Context(), ops)
	if err != nil {
		api.internalError(w, fmt.Errorf("error searching in DB: %v", err))
	}
	api.marshalQAEnvironments(qas, w)
}

func (api *v2api) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	w.Write([]byte(`{ "message" : "Todo es bueno!" }`))
}

func (api *v2api) eventLogHandler(w http.ResponseWriter, r *http.Request) {
	idstr := mux.Vars(r)["id"]
	id, err := uuid.Parse(idstr)
	if err != nil {
		api.badRequestError(w, errors.Wrap(err, "error parsing id"))
		return
	}
	el, err := api.dl.GetEventLogByID(id)
	if err != nil {
		api.internalError(w, errors.Wrap(err, "error fetching event logs"))
		return
	}
	if el == nil {
		api.notfoundError(w)
		return
	}
	j, err := json.Marshal(v2EventLogFromEventLog(el))
	if err != nil {
		api.internalError(w, errors.Wrap(err, "error marshaling event log"))
	}
	w.Header().Add("Content-Type", "application/json")
	w.Write(j)
}

func (api *v2api) eventStatusHandler(w http.ResponseWriter, r *http.Request) {
	idstr := mux.Vars(r)["id"]
	id, err := uuid.Parse(idstr)
	if err != nil {
		api.badRequestError(w, errors.Wrap(err, "error parsing id"))
		return
	}
	es, err := api.dl.GetEventStatus(id)
	if err != nil {
		api.internalError(w, errors.Wrap(err, "error fetching event status"))
		return
	}
	if es == nil {
		api.notfoundError(w)
		return
	}
	j, err := json.Marshal(v2EventStatusSummaryFromEventStatusSummary(es))
	if err != nil {
		api.internalError(w, errors.Wrap(err, "error marshaling event status"))
	}
	w.Header().Add("Content-Type", "application/json")
	w.Write(j)
}
