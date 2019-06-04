package models

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRenderCommitStatusTemplates(t *testing.T) {
	cases := []struct {
		name           string
		inputTemplate  CommitStatusTemplate
		inputData      CommitStatusData
		expectedOutput *RenderedCommitStatus
	}{
		{
			name: "No template variables",
			inputTemplate: CommitStatusTemplate{
				Description: "Hello world",
				TargetURL:   "https://google.com",
			},
			inputData: CommitStatusData{
				EnvName: "test",
			},
			expectedOutput: &RenderedCommitStatus{
				Description: "Hello world",
				TargetURL:   "https://google.com",
			},
		},
		{
			name: "One template variable",
			inputTemplate: CommitStatusTemplate{
				Description: "Hello {{.EnvName}}",
				TargetURL:   "https://{{.EnvName}}-google.com",
			},
			inputData: CommitStatusData{
				EnvName: "world",
			},
			expectedOutput: &RenderedCommitStatus{
				Description: "Hello world",
				TargetURL:   "https://world-google.com",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, err := c.inputTemplate.Render(c.inputData)
			if err != nil {
				t.Errorf("Didn't expect error: %v", err)
			}
			if diff := cmp.Diff(out, c.expectedOutput); diff != "" {
				t.Errorf("Render() mismatch (-expected +got):\n%s", diff)
			}
		})
	}

}
