package api

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/gorilla/sessions"

	"github.com/dollarshaveclub/acyl/pkg/config"
	"github.com/dollarshaveclub/acyl/pkg/ghclient"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/dollarshaveclub/acyl/pkg/persistence"

	muxtrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/gorilla/mux"
)

type uiBranding struct {
	config.UIBrandingConfig
	FaviconType string
}

// OAuthConfig models the configuration needed to support a GitHub OAuth authn/authz flow
type OAuthConfig struct {
	Enforce                     bool                                                    // Enforce OAuth authn/authz for protected routes
	AppInstallationID           int64                                                   // GitHub App installation ID
	ClientID, ClientSecret      string                                                  // GitHub App ClientID/secret
	ValidateURL                 url.URL                                                 // URL to validate callback code and exchange for bearer token
	AuthURL                     url.URL                                                 // URL to initiate OAuth flow
	AppGHClientFactoryFunc      func(token string) ghclient.GitHubAppInstallationClient // Function that produces a token-scoped GitHubAppInstallationClient
	CookieAuthKey, CookieEncKey [32]byte                                                // Cookie authentication & encryption keys (AES-256)
	cookiestore                 sessions.Store
	errorPage                   []byte
	hc                          http.Client
}

func (oac *OAuthConfig) SetValidateURL(vurl string) error {
	pvurl, err := url.Parse(vurl)
	if err != nil || pvurl == nil {
		return errors.Wrap(err, "error parsing validate url")
	}
	oac.ValidateURL = *pvurl
	return nil
}

func (oac *OAuthConfig) SetAuthURL(aurl string) error {
	paurl, err := url.Parse(aurl)
	if err != nil || paurl == nil {
		return errors.Wrap(err, "error parsing auth url")
	}
	oac.AuthURL = *paurl
	return nil
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
	oauth       OAuthConfig
	stop        chan struct{}
}

var viewPaths = map[string]string{
	"status":     path.Join("views", "status.html"),
	"auth_error": path.Join("views", "autherror.html"),
}

func newSessionsCookieStore(oauthCfg OAuthConfig) sessions.Store {
	cstore := sessions.NewCookieStore(oauthCfg.CookieAuthKey[:], oauthCfg.CookieEncKey[:])
	cstore.Options.SameSite = http.SameSiteLaxMode // Lax mode is required so callback request contains the session cookie
	cstore.Options.Secure = true
	cstore.MaxAge(int(cookieMaxAge.Seconds()))
	return cstore
}

func newUIAPI(baseURL, assetsPath, routePrefix string, reload bool, branding config.UIBrandingConfig, dl persistence.DataLayer, oauthCfg OAuthConfig, logger *log.Logger) (*uiapi, error) {
	if assetsPath == "" || routePrefix == "" ||
		dl == nil {
		return nil, errors.New("all dependencies required")
	}
	oauthCfg.cookiestore = newSessionsCookieStore(oauthCfg)
	api := &uiapi{
		apiBase: apiBase{
			logger: logger,
		},
		apiBaseURL:  baseURL,
		assetsPath:  assetsPath,
		routePrefix: routePrefix,
		dl:          dl,
		oauth:       oauthCfg,
		reload:      reload,
		views:       make(map[string]*template.Template, len(viewPaths)),
		stop:        make(chan struct{}),
	}
	for k := range viewPaths {
		if err := api.loadTemplate(k); err != nil {
			return nil, errors.Wrap(err, "error reading view template")
		}
	}
	if branding.LogoURL == "" && branding.Title == "" {
		branding = config.DefaultUIBranding
	}
	err := api.processBranding(branding)
	if err != nil {
		return nil, errors.Wrap(err, "error processing branding")
	}
	api.cacheRenderedAuthErrorPage()
	return api, nil
}

// StartSessionsCleanup begins periodic async cleanup of expired UI sessions
func (api *uiapi) StartSessionsCleanup() {
	rand.Seed(time.Now().UTC().UnixNano())
	d := time.Duration(18+rand.Intn(7)) * time.Hour // random interval between 18-24 hours
	log.Printf("ui api: starting periodic ui sessions cleanup: interval: %v", d)
	ticker := time.NewTicker(d)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				n, err := api.dl.DeleteExpiredUISessions()
				if err != nil {
					log.Printf("ui api: error deleting expired sessions: %v", err)
					continue
				}
				log.Printf("ui api: deleted %v expired ui sessions", n)
			case <-api.stop:
				log.Printf("ui api: close signalled, stopping sessions cleanup")
				return
			}
		}
	}()
}

// Close stops any async processes such as sessions cleanup
func (api *uiapi) Close() {
	close(api.stop)
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

const baseErrorRoute = "/oauth/accessdenied"

func (api *uiapi) register(r *muxtrace.Router) error {
	if r == nil {
		return fmt.Errorf("router is nil")
	}
	urlPath := func(route string) string {
		return api.routePrefix + route
	}

	// UI routes
	r.HandleFunc(urlPath("/event/status"), middlewareChain(api.statusHandler, api.authenticate)).Methods("GET")
	r.HandleFunc(urlPath(baseErrorRoute), middlewareChain(api.authErrorHandler)).Methods("GET")
	r.HandleFunc(urlPath("/oauth/callback"), middlewareChain(api.authCallbackHandler)).Methods("GET")

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
UI authn/authz flow:

client requests a protected UI route without session cookie or invalid/expired session:

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
	* this authenticates client as a valid GitHub user
- server then validates that the user is authorized:
	* GitHub user must have permissions to this installation of the GitHub app
	* GitHub user must have permissions to at least one repo that this installation has access to
- if these conditions are true, server updates session:
	* authenticated = true
	* github_user is set
- server returns a 302 redirect to target_route, with same session cookie
- client requests the protected UI route again with valid session cookie
- server decrypts and validates the session id in the database, response is the content for the protected route

Error flow:

- callback URL detects invalid state or code, or the authenticated user fails authz:
	* server returns 302 to the unauthenticated error page (/ui/oauth/accessdenied)
*/

const (
	uiSessionName  = "user-session"
	cookieIDkey    = "id"
	stateSizeBytes = 32
	cookieMaxAge   = 30 * 24 * time.Hour
)

// authenticate is a middleware that ensures a request to a protected UI path has an authenticated session
// if not, it creates a new session and redirects to the provided oauth URL
func (ui *uiapi) authenticate(f http.HandlerFunc) http.HandlerFunc {
	rr := rand.New(rand.NewSource(time.Now().UTC().UnixNano()))
	// for convenience we use only alphanumerics for the state, so we don't have to base64-encode
	rset := []rune(`abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789`)
	// randstate generates a random state for a session.
	randstate := func(n uint) []byte {
		out := make([]rune, n)
		for i := range out {
			out[i] = rset[rr.Intn(len(rset))]
		}
		return []byte(string(out))
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if !ui.oauth.Enforce {
			f(w, r)
			return
		}
		redirectToAuth := func(s *sessions.Session) {
			if s == nil {
				var err error
				s, err = ui.oauth.cookiestore.New(r, uiSessionName)
				if err != nil {
					log.Printf("error creating new cookie session: %v", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}
			state := randstate(stateSizeBytes)
			ip, _, _ := net.SplitHostPort(r.RemoteAddr) // ignore the error if the remote addr can't be parsed
			troute := r.URL.Path
			if r.URL.RawQuery != "" {
				troute += "?" + r.URL.RawQuery
			}
			id, err := ui.dl.CreateUISession(troute, state, net.ParseIP(ip), r.UserAgent(), time.Now().UTC().Add(cookieMaxAge))
			if err != nil {
				log.Printf("error creating session in db: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			s.Values[cookieIDkey] = id
			if err := s.Save(r, w); err != nil {
				log.Printf("error saving session in cookie store: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			aurl := ui.oauth.AuthURL
			qvals := make(url.Values, 2)
			qvals.Add("state", string(state))
			qvals.Add("client_id", ui.oauth.ClientID)
			aurl.RawQuery = qvals.Encode()
			w.Header().Add("Location", aurl.String())
			w.WriteHeader(http.StatusFound)
		}
		// Get returns the session from the cookie or creates a new session if missing
		sess, err := ui.oauth.cookiestore.Get(r, uiSessionName)
		if err != nil {
			// invalid cookie or failure to authenticate/decrypt
			log.Printf("error getting session, redirecting to auth: %v", err)
			redirectToAuth(sess) // Get returns a new session on error
			return
		}
		if sess.IsNew {
			log.Printf("session missing from request, redirecting to auth")
			redirectToAuth(sess)
			return
		}
		id, ok := sess.Values[cookieIDkey].(int)
		if !ok {
			// missing id
			log.Printf("session id is missing from cookie")
			redirectToAuth(nil)
			return
		}
		uis, err := ui.dl.GetUISession(id)
		if err != nil {
			log.Printf("error getting session by id: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if uis == nil {
			// not found in db
			log.Printf("session %v not found in db, redirecting to auth", id)
			redirectToAuth(nil)
			return
		}
		if !uis.IsValid() {
			if err := ui.dl.DeleteUISession(id); err != nil {
				log.Printf("error deleting session: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			log.Printf("session %v isn't valid, redirecting to auth", id)
			redirectToAuth(nil)
			return
		}
		// we have a valid, authenticated session
		if err := sess.Save(r, w); err != nil {
			log.Printf("error saving valid session in cookie store: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		f(w, r)
	}
}

// authCallbackHandler handles the OAuth GitHub callback, verifies that the authenticated GitHub
// user account is authorized to access UI resources and updates the session accordingly
func (api *uiapi) authCallbackHandler(w http.ResponseWriter, r *http.Request) {
	redirectToError := func() {
		w.Header().Add("Location", api.apiBaseURL+api.routePrefix+baseErrorRoute)
		w.WriteHeader(http.StatusFound)
	}
	sess, err := api.oauth.cookiestore.Get(r, uiSessionName)
	if err != nil || sess.IsNew {
		// invalid cookie or failure to authenticate/decrypt
		log.Printf("error getting session or new session, redirecting to error: %v", err)
		redirectToError() // Get returns a new session on error
		return
	}
	id, ok := sess.Values[cookieIDkey].(int)
	if !ok {
		// missing id
		log.Printf("session id is missing from cookie")
		redirectToError()
		return
	}
	uis, err := api.dl.GetUISession(id)
	if err != nil {
		log.Printf("error getting session by id: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if uis == nil {
		// not found in db
		log.Printf("session %v not found in db, redirecting to auth", id)
		redirectToError()
		return
	}
	state := r.URL.Query().Get("state")
	if state != string(uis.State) {
		log.Printf("missing or invalid state: %v", state)
		redirectToError()
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		log.Printf("missing code: %v", code)
		redirectToError()
		return
	}
	ghuser, err := api.getAndAuthorizeGitHubUser(r.Context(), code, state)
	if err != nil {
		log.Printf("error authorizing github user: %v", err)
		redirectToError()
		return
	}
	if err := api.dl.UpdateUISession(id, ghuser, true); err != nil {
		log.Printf("error updating session: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if err := sess.Save(r, w); err != nil {
		log.Printf("error saving session to cookie store: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Add("Location", api.apiBaseURL+uis.TargetRoute)
	w.WriteHeader(http.StatusFound)
}

// getAndAuthorizeGitHubUser verifies that the code supplied to the redirect is valid, and that the GitHub user is authorized to view UI resources
// A GitHub user is considered authorized if and only if all the following are true:
// - the user has explicit permissions for this installation of this GitHub app
// - the user has at least one repository accessible via this installation
func (api *uiapi) getAndAuthorizeGitHubUser(ctx context.Context, code, state string) (string, error) {
	form := make(url.Values, 4)
	form.Add("client_id", api.oauth.ClientID)
	form.Add("client_secret", api.oauth.ClientSecret)
	form.Add("code", code)
	form.Add("state", state)
	req, err := http.NewRequest("POST", api.oauth.ValidateURL.String(), strings.NewReader(form.Encode()))
	if err != nil {
		return "", errors.Wrap(err, "error creating validate request")
	}
	resp, err := api.oauth.hc.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "error validating code")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("validation response indicates error: %v", resp.StatusCode)
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "error reading response body")
	}
	vals, err := url.ParseQuery(string(b))
	if err != nil {
		return "", errors.Wrap(err, "error parsing response")
	}
	tkn := vals.Get("access_token")
	if tkn == "" || vals.Get("token_type") != "bearer" {
		return "", fmt.Errorf("invalid validation reponse or missing values: %v", string(b))
	}
	rc := api.oauth.AppGHClientFactoryFunc(tkn)
	ghuser, err := rc.GetUser(ctx)
	if err != nil {
		return "", errors.Wrap(err, "error getting github user from token")
	}
	insts, err := rc.GetUserAppInstallations(ctx)
	if err != nil {
		return "", errors.Wrap(err, "error getting user app installations")
	}
	if !insts.IDPresent(api.oauth.AppInstallationID) {
		return "", fmt.Errorf("app installation id missing from user installations")
	}
	repos, err := rc.GetUserAppRepos(ctx, api.oauth.AppInstallationID)
	if err != nil {
		return "", fmt.Errorf("error getting user app installation repos")
	}
	if len(repos) == 0 {
		return "", fmt.Errorf("user %v has zero app installation repos", ghuser)
	}
	return ghuser, nil
}

// cacheRenderedAuthErrorPage renders the auth error page and saves the output for future use
// it must be called via uiapi constructor only as no locking is performed
func (api *uiapi) cacheRenderedAuthErrorPage() {
	tmpldata := struct {
		Branding uiBranding
	}{
		Branding: api.branding,
	}
	if err := api.loadTemplate("auth_error"); err != nil {
		api.logger.Printf("error loading auth_error template: %v", err)
		return
	}
	b := bytes.Buffer{}
	if err := api.views["auth_error"].Execute(&b, &tmpldata); err != nil {
		api.logger.Printf("error rendering auth_error template: %v", err)
		return
	}
	api.oauth.errorPage = b.Bytes()
}

func (api *uiapi) authErrorHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	if api.reload || len(api.oauth.errorPage) == 0 {
		tmpldata := struct {
			Branding uiBranding
		}{
			Branding: api.branding,
		}
		if err := api.loadTemplate("auth_error"); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			api.logger.Printf("error serving ui template: %v", err)
			return
		}
		b := bytes.Buffer{}
		api.viewmtx.RLock()
		defer api.viewmtx.RUnlock()
		if err := api.views["auth_error"].Execute(&b, &tmpldata); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			api.logger.Printf("error serving ui template: status: %v", err)
		}
		w.WriteHeader(http.StatusUnauthorized)
		w.Write(b.Bytes())
		return
	}
	w.WriteHeader(http.StatusUnauthorized)
	w.Write(api.oauth.errorPage)
}
