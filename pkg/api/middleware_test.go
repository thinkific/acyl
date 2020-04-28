package api

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const (
	testValidAPIKey   = "foo"
	testInvalidAPIKey = "bar"
	testCIDR          = "10.10.0.0/16"
)

func TestMiddlewareIPWhitelistSuccess(t *testing.T) {
	_, cn, err := net.ParseCIDR(testCIDR)
	if err != nil {
		t.Fatalf("error parsing CIDR: %v", err)
	}
	ipWhitelistMiddleware.ipwl = []*net.IPNet{cn}
	rc := httptest.NewRecorder()
	h := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }
	mh := middlewareChain(h, ipWhitelistMiddleware.checkIPWhitelist)
	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.10.100.1:34567"
	mh(rc, req)
	if rc.Code != http.StatusNoContent {
		t.Fatalf("should have succeeded: %v", rc.Code)
	}
}

func TestMiddlewareIPWhitelistFailure(t *testing.T) {
	_, cn, err := net.ParseCIDR(testCIDR)
	if err != nil {
		t.Fatalf("error parsing CIDR: %v", err)
	}
	ipWhitelistMiddleware.ipwl = []*net.IPNet{cn}
	rc := httptest.NewRecorder()
	h := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }
	mh := middlewareChain(h, ipWhitelistMiddleware.checkIPWhitelist)
	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.20.10"
	mh(rc, req)
	if rc.Code != http.StatusForbidden {
		t.Fatalf("should have succeeded")
	}
}

func TestMiddlewareAuthSuccess(t *testing.T) {
	authMiddleware.apiKeys = []string{testValidAPIKey}
	rc := httptest.NewRecorder()
	h := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }
	mh := middlewareChain(h, authMiddleware.authRequest)
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Add(apiKeyHeader, testValidAPIKey)
	mh(rc, req)
	if rc.Code != http.StatusNoContent {
		t.Fatalf("should have succeeded")
	}
}

func TestMiddlewareAuthFailure(t *testing.T) {
	authMiddleware.apiKeys = []string{testValidAPIKey}
	rc := httptest.NewRecorder()
	h := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }
	mh := middlewareChain(h, authMiddleware.authRequest)
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Add(apiKeyHeader, testInvalidAPIKey)
	mh(rc, req)
	if rc.Code != http.StatusUnauthorized {
		t.Fatalf("should have succeeded")
	}
}

func TestMiddlewareWaitGroupDelay(t *testing.T) {
	authMiddleware.apiKeys = []string{testValidAPIKey}
	rc := httptest.NewRecorder()
	h := func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(250 * time.Millisecond)
		w.WriteHeader(http.StatusNoContent)
	}
	mh := middlewareChain(h, waitMiddleware.waitOnRequest)
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Add(apiKeyHeader, testValidAPIKey)

	start := time.Now()
	go mh(rc, req)
	time.Sleep(5 * time.Millisecond)
	waitMiddleware.wg.Wait()
	elapsed := time.Since(start)

	if rc.Code != http.StatusNoContent {
		t.Fatalf("should have succeeded")
	}
	if elapsed.Seconds() < 0.2499999 {
		t.Fatalf("should have waited")
	}
}
