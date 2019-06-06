package models

import (
	"bytes"
	"text/template"

	"github.com/pkg/errors"
)

type NitroCommitStatus int

// Valid commit statuses from Nitro's perspective
const (
	// CommitStatusSuccess occurs when a Nitro environment has been created.
	CommitStatusSuccess NitroCommitStatus = iota
	// CommitStatusPending occurs when Nitro is first creating an environment,
	// or updating an existing environment
	CommitStatusPending
	// CommitStatusFailure occurs when an unrecoverable error has occured during
	// the creation or update of an environment
	CommitStatusFailure
)

func (ncs NitroCommitStatus) Key() string {
	switch ncs {
	case CommitStatusSuccess:
		return "success"
	case CommitStatusPending:
		return "pending"
	case CommitStatusFailure:
		return "failure"
	default:
		return "failure: unknown status"
	}
}

// CommitStatuses models the configuration that Nitro supports for setting
// commit statuses. Users can specify templates for each valid commit status.
type CommitStatuses struct {
	Templates map[string]CommitStatusTemplate `yaml:"templates" json:"templates"`
}

type CommitStatusTemplate struct {
	Description string `yaml:"description" json:"description"`
	TargetURL   string `yaml:"target_url" json:"target_url"`
}

type RenderedCommitStatus struct {
	Description, TargetURL string
}

// CommitStatusData models the data available to commit status templates.
type CommitStatusData struct {
	EnvName, ErrorMessage, K8sNamespace string
}

// Render renders the commit status template using the supplied data.
func (cs CommitStatusTemplate) Render(d CommitStatusData) (*RenderedCommitStatus, error) {
	desc, err := renderTemplate("description", cs.Description, d)
	if err != nil {
		return nil, errors.Wrap(err, "error rendering commit status description template")
	}
	url, err := renderTemplate("target_url", cs.TargetURL, d)
	if err != nil {
		return nil, errors.Wrap(err, "error rendering commit status target url")
	}
	return &RenderedCommitStatus{
		Description: desc,
		TargetURL:   url,
	}, nil
}

func renderTemplate(name, ts string, d CommitStatusData) (string, error) {
	tmpl, err := template.New(name).Parse(ts)
	if err != nil {
		return "", errors.Wrap(err, "error parsing template")
	}
	res := &bytes.Buffer{}
	if err := tmpl.Execute(res, d); err != nil {
		return "", errors.Wrap(err, "error executing template")
	}
	return res.String(), nil
}

// DefaultCommitStatusTemplates are the templates that are used if the user
// does not specify a template in the acyl.yml configuration file.
var DefaultCommitStatusTemplates = map[string]CommitStatusTemplate{
	"success": CommitStatusTemplate{
		Description: "The Acyl environment {{ .EnvName }} was created successfully.",
		TargetURL:   "https://media.giphy.com/media/SRO0ZwmImic0/giphy.gif",
	},
	"pending": CommitStatusTemplate{
		Description: "The Acyl environment {{ .EnvName }} is being created.",
		TargetURL:   "https://media.giphy.com/media/oiymhxu13VYEo/giphy.gif",
	},
	"failure": CommitStatusTemplate{
		Description: "The Acyl environment {{ .EnvName }} failed. Reason: {{ .ErrorMessage }}. Check the Acyl event log for more details.",
		TargetURL:   "https://media.giphy.com/media/pyFsc5uv5WPXN9Ocki/giphy.gif",
	},
}
