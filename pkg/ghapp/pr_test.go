package ghapp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/dollarshaveclub/acyl/pkg/ghclient"

	"github.com/dollarshaveclub/acyl/pkg/nitro/env"
	"github.com/dollarshaveclub/acyl/pkg/persistence"
	"github.com/palantir/go-githubapp/githubapp"
)

// fake testing key
var key = `-----BEGIN RSA PRIVATE KEY-----
MIIEpQIBAAKCAQEA0BUezcR7uycgZsfVLlAf4jXP7uFpVh4geSTY39RvYrAll0yh
q7uiQypP2hjQJ1eQXZvkAZx0v9lBYJmX7e0HiJckBr8+/O2kARL+GTCJDJZECpjy
97yylbzGBNl3s76fZ4CJ+4f11fCh7GJ3BJkMf9NFhe8g1TYS0BtSd/sauUQEuG/A
3fOJxKTNmICZr76xavOQ8agA4yW9V5hKcrbHzkfecg/sQsPMmrXixPNxMsqyOMmg
jdJ1aKr7ckEhd48ft4bPMO4DtVL/XFdK2wJZZ0gXJxWiT1Ny41LVql97Odm+OQyx
tcayMkGtMb1nwTcVVl+RG2U5E1lzOYpcQpyYFQIDAQABAoIBAAfUY55WgFlgdYWo
i0r81NZMNBDHBpGo/IvSaR6y/aX2/tMcnRC7NLXWR77rJBn234XGMeQloPb/E8iw
vtjDDH+FQGPImnQl9P/dWRZVjzKcDN9hNfNAdG/R9JmGHUz0JUddvNNsIEH2lgEx
C01u/Ntqdbk+cDvVlwuhm47MMgs6hJmZtS1KDPgYJu4IaB9oaZFN+pUyy8a1w0j9
RAhHpZrsulT5ThgCra4kKGDNnk2yfI91N9lkP5cnhgUmdZESDgrAJURLS8PgInM4
YPV9L68tJCO4g6k+hFiui4h/4cNXYkXnaZSBUoz28ICA6e7I3eJ6Y1ko4ou+Xf0V
csM8VFkCgYEA7y21JfECCfEsTHwwDg0fq2nld4o6FkIWAVQoIh6I6o6tYREmuZ/1
s81FPz/lvQpAvQUXGZlOPB9eW6bZZFytcuKYVNE/EVkuGQtpRXRT630CQiqvUYDZ
4FpqdBQUISt8KWpIofndrPSx6JzI80NSygShQsScWFw2wBIQAnV3TpsCgYEA3reL
L7AwlxCacsPvkazyYwyFfponblBX/OvrYUPPaEwGvSZmE5A/E4bdYTAixDdn4XvE
ChwpmRAWT/9C6jVJ/o1IK25dwnwg68gFDHlaOE+B5/9yNuDvVmg34PWngmpucFb/
6R/kIrF38lEfY0pRb05koW93uj1fj7Uiv+GWRw8CgYEAn1d3IIDQl+kJVydBKItL
tvoEur/m9N8wI9B6MEjhdEp7bXhssSvFF/VAFeQu3OMQwBy9B/vfaCSJy0t79uXb
U/dr/s2sU5VzJZI5nuDh67fLomMni4fpHxN9ajnaM0LyI/E/1FFPgqM+Rzb0lUQb
yqSM/ptXgXJls04VRl4VjtMCgYEAprO/bLx2QjxdPpXGFcXbz6OpsC92YC2nDlsP
3cfB0RFG4gGB2hbX/6eswHglLbVC/hWDkQWvZTATY2FvFps4fV4GrOt5Jn9+rL0U
elfC3e81Dw+2z7jhrE1ptepprUY4z8Fu33HNcuJfI3LxCYKxHZ0R2Xvzo+UYSBqO
ng0eTKUCgYEAxW9G4FjXQH0bjajntjoVQGLRVGWnteoOaQr/cy6oVii954yNMKSP
rezRkSNbJ8cqt9XQS+NNJ6Xwzl3EbuAt6r8f8VO1TIdRgFOgiUXRVNZ3ZyW8Hegd
kGTL0A6/0yAu9qQZlFbaD5bWhQo7eyx63u4hZGppBhkTSPikOYUPCH8=
-----END RSA PRIVATE KEY-----`

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
			c.App.PrivateKey = key
			cc, _ := githubapp.NewDefaultCachingClientCreator(c)
			prh := &prEventHandler{
				ClientCreator: cc,
				wg:            &sync.WaitGroup{},
				es:            &env.FakeManager{},
				dl:            persistence.NewFakeDataLayer(),
				rc: &ghclient.FakeRepoClient{
					GetFileContentsFunc: func(ctx context.Context, repo, path, ref string) ([]byte, error) {
						return []byte("target_branches:\n  - tyui\n"), nil
					},
				},
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
