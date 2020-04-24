package api

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"sync"
	"text/template"

	"github.com/dollarshaveclub/acyl/pkg/config"

	"github.com/dollarshaveclub/acyl/pkg/persistence"
	"github.com/google/uuid"
	"github.com/pkg/errors"

	muxtrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/gorilla/mux"
)

type uiBranding struct {
	config.UIBrandingConfig
	FaviconType string
}

type uiapi struct {
	apiBase
	dl          persistence.DataLayer
	apiBaseURL  string
	assetsPath  string
	routePrefix string
	reload      bool
	views       map[string]*template.Template
	viewmtx     sync.RWMutex
	branding    uiBranding
}

var viewPaths = map[string]string{
	"status": path.Join("views", "status.html"),
}

func newUIAPI(baseURL, assetsPath, routePrefix string, reload bool, branding config.UIBrandingConfig, dl persistence.DataLayer, logger *log.Logger) (*uiapi, error) {
	if assetsPath == "" || routePrefix == "" ||
		dl == nil {
		return nil, errors.New("all dependencies required")
	}
	api := &uiapi{
		apiBase: apiBase{
			logger: logger,
		},
		apiBaseURL:  baseURL,
		assetsPath:  assetsPath,
		routePrefix: routePrefix,
		dl:          dl,
		reload:      reload,
		views:       make(map[string]*template.Template, len(viewPaths)),
	}
	for k := range viewPaths {
		if err := api.loadTemplate(k); err != nil {
			return nil, errors.Wrap(err, "error reading view template")
		}
	}
	if branding.LogoURL == "" && branding.Title == "" {
		branding = config.DefaultUIBranding
	}
	return api, api.processBranding(branding)
}

func (api *uiapi) processBranding(b config.UIBrandingConfig) error {
	api.branding.UIBrandingConfig = b
	fiurl, err := url.Parse(b.FaviconURL)
	if err != nil {
		return errors.Wrap(err, "error in favicon url")
	}
	switch filepath.Ext(fiurl.Path) {
	case ".ico":
		api.branding.FaviconType = "image/x-icon"
	case ".gif":
		api.branding.FaviconType = "image/gif"
	case ".png":
		api.branding.FaviconType = "image/png"
	}
	if _, err := url.Parse(b.LogoURL); err != nil {
		return errors.Wrap(err, "error in logo url")
	}
	return nil
}

func (api *uiapi) loadTemplate(name string) error {
	v := viewPaths[name]
	if v == "" {
		return fmt.Errorf("view not found: %v", name)
	}
	p := path.Join(api.assetsPath, v)
	d, err := ioutil.ReadFile(p)
	if err != nil {
		return errors.Wrapf(err, "error reading asset: %v", p)
	}
	tmpl, err := template.New(name).Parse(string(d))
	if err != nil {
		return errors.Wrapf(err, "error parsing asset template: %v", p)
	}
	api.viewmtx.Lock()
	api.views[name] = tmpl
	api.viewmtx.Unlock()
	return nil
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

type StatusTemplateData struct {
	APIBaseURL string
	LogKey     string
	Branding   uiBranding
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
		api.logger.Printf("error serving status page: event log not found: %v", id)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	tmpldata := StatusTemplateData{
		Branding:   api.branding,
		APIBaseURL: api.apiBaseURL,
		LogKey:     elog.LogKey.String(),
	}
	w.Header().Add("Content-Type", "text/html")
	if api.reload {
		if err := api.loadTemplate("status"); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			api.logger.Printf("error serving ui template: %v", err)
			return
		}
	}
	api.viewmtx.RLock()
	defer api.viewmtx.RUnlock()
	if err := api.views["status"].Execute(w, &tmpldata); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		api.logger.Printf("error serving ui template: status: %v", err)
	}
}

/*
UI auth flow:

client request to a protected UI route without session cookie or invalid/expired session:

- server detects missing/invalid session, goes into auth flow
- creates new session:
	* authenticated = false
	* target_route = the route they attempted to access
	* state = random data
- session cookie is the session ID (encrypted)
- server response is 302 to github auth endpoint w/ state param

client successfully auths w/ github and is redirected to acyl callback URL:

- request contains session cookie set above (session ID)
- server validates code and state parameters supplied by GitHub in the callback request
- server validates that the user is authorized (part of the org, etc)
- server updates session:
	* authenticated = true
	* target_route is unset (but remembered for redirect below)
	* github_user is set
- server returns a 302 redirect to target_route, with same session cookie

client requests the protected UI route again with valid session cookie:

- server decrypts and validates the session, response is the content for the protected route

###############

Error flow:

- callback URL detects invalid state or code, or the authenticated user fails permission check:
	* server returns 302 to an unauthenticated "login error"/"access denied" page

*/
