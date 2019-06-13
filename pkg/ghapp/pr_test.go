package ghapp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dollarshaveclub/acyl/pkg/nitro/env"
	"github.com/dollarshaveclub/acyl/pkg/persistence"
	"github.com/palantir/go-githubapp/githubapp"
)

func Test_prEventHandler_Handle(t *testing.T) {
	rc := httptest.NewRecorder()
	type args struct {
		eventType  string
		deliveryID string
		payload    []byte
		w          http.ResponseWriter
	}
	tests := []struct {
		name       string
		args       args
		wantStatus int
		wantErr    bool
	}{
		{
			name: "opened",
			args: args{
				eventType:  "opened",
				deliveryID: "asdf",
				payload:    []byte(`{"action":"opened","repo":{"owner":"foo","name":"bar"},"pull_request":{"number":1,"head":{"label":"asdf","ref":"zxcv","sha":"1234"},"base":{"label":"qwer","ref":"tyui","sha":"5678"}}}`),
				w:          rc,
			},
			wantStatus: http.StatusAccepted,
		},
		{
			name: "reopened",
			args: args{
				eventType:  "reopened",
				deliveryID: "asdf",
				payload:    []byte(`{"action":"reopened","repo":{"owner":"foo","name":"bar"},"pull_request":{"number":1,"head":{"label":"asdf","ref":"zxcv","sha":"1234"},"base":{"label":"qwer","ref":"tyui","sha":"5678"}}}`),
				w:          rc,
			},
			wantStatus: http.StatusAccepted,
		},
		{
			name: "update",
			args: args{
				eventType:  "synchronize",
				deliveryID: "asdf",
				payload:    []byte(`{"action":"synchronize","repo":{"owner":"foo","name":"bar"},"pull_request":{"number":1,"head":{"label":"asdf","ref":"zxcv","sha":"1234"},"base":{"label":"qwer","ref":"tyui","sha":"5678"}}}`),
				w:          rc,
			},
			wantStatus: http.StatusAccepted,
		},
		{
			name: "closed",
			args: args{
				eventType:  "closed",
				deliveryID: "asdf",
				payload:    []byte(`{"action":"closed","repo":{"owner":"foo","name":"bar"},"pull_request":{"number":1,"head":{"label":"asdf","ref":"zxcv","sha":"1234"},"base":{"label":"qwer","ref":"tyui","sha":"5678"}}}`),
				w:          rc,
			},
			wantStatus: http.StatusAccepted,
		},
		{
			name: "unsupported action",
			args: args{
				eventType:  "someotherthing",
				deliveryID: "asdf",
				payload:    []byte(`{"action":"someotherthing","repo":{"owner":"foo","name":"bar"},"pull_request":{"number":1,"head":{"label":"asdf","ref":"zxcv","sha":"1234"},"base":{"label":"qwer","ref":"tyui","sha":"5678"}}}`),
				w:          rc,
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "invalid payload",
			args: args{
				eventType:  "opened",
				deliveryID: "asdf",
				payload:    []byte(`invalid`),
				w:          rc,
			},
			wantErr:    true,
			wantStatus: http.StatusBadRequest,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := githubapp.Config{}
			c.App.IntegrationID = 10
			cc, _ := githubapp.NewDefaultCachingClientCreator(c)
			prh := &prEventHandler{
				ClientCreator: cc,
				es:            &env.FakeManager{},
				dl:            persistence.NewFakeDataLayer(),
			}
			gotStatus, _, err := prh.Handle(context.Background(), tt.args.eventType, tt.args.deliveryID, tt.args.payload, tt.args.w)
			if (err != nil) != tt.wantErr {
				t.Errorf("prEventHandler.Handle() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotStatus != tt.wantStatus {
				t.Errorf("prEventHandler.Handle() gotStatus = %v, want %v", gotStatus, tt.wantStatus)
			}
			rc = httptest.NewRecorder() // reset the recorder
		})
	}
}

// func Test_prEventHandler_Handles(t *testing.T) {
// 	type fields struct {
// 		ClientCreator githubapp.ClientCreator
// 		wg            sync.WaitGroup
// 		es            spawner.EnvironmentSpawner
// 		dl            persistence.DataLayer
// 	}
// 	tests := []struct {
// 		name   string
// 		fields fields
// 		want   []string
// 	}{
// 		// TODO: Add test cases.
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			prh := &prEventHandler{
// 				ClientCreator: tt.fields.ClientCreator,
// 				wg:            tt.fields.wg,
// 				es:            tt.fields.es,
// 				dl:            tt.fields.dl,
// 			}
// 		})
// 	}
// }
