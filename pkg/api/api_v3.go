package api

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"

	muxtrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/gorilla/mux"
)

type WebhookHandlerFactory interface {
	Handler(wg *sync.WaitGroup) http.Handler
}

type v3api struct {
	apiBase
	hf WebhookHandlerFactory
}

func newV3API(hf WebhookHandlerFactory, logger *log.Logger) (*v3api, error) {
	if hf == nil {
		return nil, errors.New("handlerFactory is required")
	}
	return &v3api{
		apiBase: apiBase{
			logger: logger,
		},
		hf: hf,
	}, nil
}

func (api *v3api) register(r *muxtrace.Router) error {
	if r == nil {
		return fmt.Errorf("router is nil")
	}
	// GitHub app webhook handler
	r.HandleFunc("/v3/github/webhook", middlewareChain(http.HandlerFunc(api.hf.Handler(&api.wg).ServeHTTP), waitMiddleware.waitOnRequest)).Methods("POST")
	return nil
}
