package api

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/gorilla/sessions"

	"github.com/dollarshaveclub/acyl/pkg/models"
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

// globals are a bad idea but this is how all the other middlewares work, so...
var sessionAuthMiddleware = &sessionAuthenticator{}

// these are used to store the session ID in the request context
const sessionIDContextKeyVal = "ctx_acyl_session_id"

type sessionIDContextKey string

func withSession(ctx context.Context, uis models.UISession) context.Context {
	return context.WithValue(ctx, sessionIDContextKey(sessionIDContextKeyVal), uis)
}

func getSessionFromContext(ctx context.Context) (models.UISession, error) {
	uis, ok := ctx.Value(sessionIDContextKey(sessionIDContextKeyVal)).(models.UISession)
	if !ok {
		return models.UISession{}, fmt.Errorf("session missing from context")
	}
	if !uis.Authenticated || uis.GitHubUser == "" {
		return models.UISession{}, fmt.Errorf("unauthenticated session or empty GitHub user")
	}
	return uis, nil
}

// sessionAuthenticator is a middleware that authenticates UI API calls with session cookies
type sessionAuthenticator struct {
	Enforce     bool
	CookieStore sessions.Store
	DL          persistence.UISessionsDataLayer
}

func (sa *sessionAuthenticator) sessionAuth(f http.HandlerFunc) http.HandlerFunc {
	if sa.CookieStore == nil || sa.DL == nil {
		return f
	}
	if !sa.Enforce {
		return f
	}
	return func(w http.ResponseWriter, r *http.Request) {
		accessDenied := func() {
			w.WriteHeader(http.StatusForbidden)
		}
		// Get returns the session from the cookie or creates a new session if missing
		sess, err := sa.CookieStore.Get(r, uiSessionName)
		if err != nil {
			// invalid cookie or failure to authenticate/decrypt
			log.Printf("sessionAuth: error getting session, access denied: %v", err)
			accessDenied()
			return
		}
		if sess.IsNew {
			log.Printf("sessionAuth: session missing from request, access denied")
			accessDenied()
			return
		}
		id, ok := sess.Values[cookieIDkey].(int)
		if !ok {
			// missing id
			log.Printf("sessionAuth: session id is missing from cookie")
			accessDenied()
			return
		}
		uis, err := sa.DL.GetUISession(id)
		if err != nil {
			log.Printf("sessionAuth: error getting session by id: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if uis == nil {
			// not found in db
			log.Printf("sessionAuth: session %v not found in db, access denied", id)
			accessDenied()
			return
		}
		if !uis.IsValid() {
			if err := sa.DL.DeleteUISession(id); err != nil {
				log.Printf("sessionAuth: error deleting session: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			log.Printf("sessionAuth: session %v isn't valid, access denied", id)
			accessDenied()
			return
		}
		f(w, r.Clone(withSession(r.Context(), *uis)))
	}
}
