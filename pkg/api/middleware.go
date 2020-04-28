package api

import (
	"net"
	"net/http"
	"sync"
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
