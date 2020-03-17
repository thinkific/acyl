package api

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path"
	"text/template"

	"github.com/dollarshaveclub/acyl/pkg/persistence"
	"github.com/google/uuid"
	"github.com/pkg/errors"

	muxtrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/gorilla/mux"
)

type uiapi struct {
	apiBase
	dl          persistence.DataLayer
	apiBaseURL  string
	assetsPath  string
	routePrefix string
	views       map[string]*template.Template
}

var viewPaths = map[string]string{
	"status": path.Join("views", "status.html"),
}

func newUIAPI(baseURL, assetsPath, routePrefix string, dl persistence.DataLayer, logger *log.Logger) (*uiapi, error) {
	if baseURL == "" || assetsPath == "" || routePrefix == "" ||
		dl == nil {
		return nil, errors.New("all deps required")
	}
	api := &uiapi{
		apiBase: apiBase{
			logger: logger,
		},
		apiBaseURL:  baseURL,
		assetsPath:  assetsPath,
		routePrefix: routePrefix,
		dl:          dl,
		views:       make(map[string]*template.Template, len(viewPaths)),
	}
	for k, v := range viewPaths {
		p := path.Join(api.assetsPath, v)
		d, err := ioutil.ReadFile(p)
		if err != nil {
			return nil, errors.Wrapf(err, "error reading asset: %v", p)
		}
		tmpl, err := template.New(k).Parse(string(d))
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing asset template: %v", p)
		}
		api.views[k] = tmpl
	}
	return api, nil
}

func (api *uiapi) register(r *muxtrace.Router) error {
	if r == nil {
		return fmt.Errorf("router is nil")
	}
	urlPath := func(route string) string {
		return api.routePrefix + route
	}

	// UI routes
	r.HandleFunc(urlPath("/event/status"), middlewareChain(api.statusHandler)).Methods("GET")

	// static assets
	r.PathPrefix(urlPath("/static/")).Handler(http.StripPrefix(urlPath("/static/"), http.FileServer(http.Dir(path.Join(api.assetsPath, "assets")))))

	return nil
}

func (api *uiapi) statusHandler(w http.ResponseWriter, r *http.Request) {
	ids := r.URL.Query()["id"]
	if len(ids) != 1 {
		api.logger.Printf("error serving status page: missing event id")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	id, err := uuid.Parse(ids[0])
	if err != nil {
		api.logger.Printf("error serving status page: invalid event id: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	elog, err := api.dl.GetEventLogByID(id)
	if err != nil {
		api.logger.Printf("error serving status page: error getting event log: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if elog == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	tmpldata := struct {
		APIBaseURL string
		LogKey     string
	}{
		APIBaseURL: api.apiBaseURL,
		LogKey:     elog.LogKey.String(),
	}
	w.Header().Add("Content-Type", "text/html")
	if err := api.views["status"].Execute(w, &tmpldata); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		api.logger.Printf("error serving ui template: status: %v", err)
	}
}
