package api

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/sessions"
	"github.com/pkg/errors"

	"github.com/dollarshaveclub/acyl/pkg/persistence"
)

type middleware func(http.HandlerFunc) http.HandlerFunc

// middlewareChain is used to chain middlewares on a request handler.
// Usage: router.HandleFunc("/foo", middlewareChain(myhandler, authMiddleware.authRequest, waitMiddleware.waitOnRequest))
func middlewareChain(f http.HandlerFunc, m ...middleware) http.HandlerFunc {
	if len(m) < 1 {
		return f
	}
	return m[0](middlewareChain(f, m[1:cap(m)]...))
}

const (
	apiKeyHeader = "API-Key"
)

// authMiddleware checks for correct API key header or aborts with Unauthorized
var authMiddleware = reqAuthorizor{}

type reqAuthorizor struct {
	apiKeys []string
}

func (ra reqAuthorizor) authRequest(f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get(apiKeyHeader)
		if h != "" {
			for _, k := range ra.apiKeys {
				if h == k {
					f(w, r)
					return
				}
			}
		}
		w.WriteHeader(http.StatusUnauthorized)
	}
}

// waitMiddleware increments a waitgroup for the duration of the request
var waitMiddleware = waitOnRequests{}

type waitOnRequests struct {
	wg sync.WaitGroup
}

func (wr *waitOnRequests) waitOnRequest(f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wr.wg.Add(1)
		f(w, r)
		wr.wg.Done()
	}
}

var ipWhitelistMiddleware = ipWhitelistChecker{}

type ipWhitelistChecker struct {
	ipwl []*net.IPNet
}

func (iwc *ipWhitelistChecker) checkIPWhitelist(f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		addr := net.ParseIP(host)
		for _, cidr := range iwc.ipwl {
			if cidr.Contains(addr) {
				f(w, r)
				return
			}
		}
		w.WriteHeader(http.StatusForbidden)
	}
}

const (
	uiSessionName  = "user-session"
	cookieIDkey    = "id"
	stateSizeBytes = 32
	cookieMaxAge   = 30 * 24 * time.Hour
)

type uiAuthRequest struct {
	dl          persistence.UISessionsDataLayer
	authURL     url.URL
	cookiestore sessions.Store
}

func newUIAuthRequest(dl persistence.UISessionsDataLayer, authURL string, authKey, encKey [32]byte) (*uiAuthRequest, error) {
	if dl == nil {
		return nil, fmt.Errorf("UISessionsDataLayer is required")
	}
	aurl, err := url.Parse(authURL)
	if err != nil {
		return nil, errors.Wrap(err, "error parsing auth url")
	}
	cstore := sessions.NewCookieStore(authKey[:], encKey[:])
	cstore.Options.SameSite = http.SameSiteStrictMode
	cstore.Options.Secure = true
	cstore.MaxAge(int(cookieMaxAge.Seconds()))
	return &uiAuthRequest{
		dl:          dl,
		authURL:     *aurl,
		cookiestore: cstore,
	}, nil
}

// uiAuth is a middleware that ensures a request to a protected UI path has an authenticated session
// if not, it creates a new session and redirects to the provided oauth URL
func (uia *uiAuthRequest) uiAuth(f http.HandlerFunc) http.HandlerFunc {
	if uia.dl == nil || uia.cookiestore == nil {
		log.Fatalf("uiAuthRequest: missing params: %+v", uia)
	}
	rr := rand.New(rand.NewSource(time.Now().UTC().UnixNano()))
	// for convenience we use only alphanumerics for the state, so we don't have to worry about URL encoding or base64
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
		redirectToAuth := func(s *sessions.Session) {
			if s == nil {
				var err error
				s, err = uia.cookiestore.New(r, uiSessionName)
				if err != nil {
					log.Printf("error creating new cookie session: %v", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}
			state := randstate(stateSizeBytes)
			ip, _, _ := net.SplitHostPort(r.RemoteAddr) // ignore the error if the remote addr can't be parsed
			id, err := uia.dl.CreateUISession(r.URL.Path, state, net.ParseIP(ip), r.UserAgent(), time.Now().UTC().Add(cookieMaxAge))
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
			aurl := uia.authURL
			qvals := make(url.Values, 1)
			qvals.Add("state", string(state))
			aurl.RawQuery = qvals.Encode()
			w.Header().Add("Location", aurl.String())
			w.WriteHeader(http.StatusFound)
		}
		// Get returns the session from the cookie or creates a new session if missing
		sess, err := uia.cookiestore.Get(r, uiSessionName)
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
		uis, err := uia.dl.GetUISession(id)
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
			if err := uia.dl.DeleteUISession(id); err != nil {
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
