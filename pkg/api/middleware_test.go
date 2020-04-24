package api

import (
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/securecookie"
	"github.com/pkg/errors"

	"github.com/dollarshaveclub/acyl/pkg/persistence"
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

func randomCookieKey() [32]byte {
	rand.Seed(time.Now().UTC().UnixNano())
	out := [32]byte{}
	rand.Read(out[:])
	return out
}

func Test_uiAuthRequest_uiAuth(t *testing.T) {
	type fields struct {
		cookiekeys [2][32]byte
	}
	var sessID int64
	authKey := randomCookieKey()
	encKey := randomCookieKey()
	expectStatus := func(rc *httptest.ResponseRecorder, status int) error {
		if rc.Result().StatusCode != status {
			return fmt.Errorf("unexpected status code: %v (wanted %v)", rc.Result().StatusCode, status)
		}
		return nil
	}
	expectOK := func(rc *httptest.ResponseRecorder, dl persistence.UISessionsDataLayer) error {
		return expectStatus(rc, 200)
	}
	expectAuthRedirect := func(rc *httptest.ResponseRecorder, dl persistence.UISessionsDataLayer) error {
		return expectStatus(rc, 302)
	}
	tests := []struct {
		name        string
		fields      fields
		wantErr     bool
		dbSetupFunc func(dl persistence.UISessionsDataLayer)
		requestFunc func() *http.Request
		verifyfunc  func(*httptest.ResponseRecorder, persistence.UISessionsDataLayer) error
	}{
		{
			name: "authenticated",
			fields: fields{
				cookiekeys: [2][32]byte{authKey, encKey},
			},
			dbSetupFunc: func(dl persistence.UISessionsDataLayer) {
				id, _ := dl.CreateUISession("/some/other/path", []byte("asdf"), net.ParseIP("10.0.0.0"), "some agent", time.Now().UTC().Add(24*time.Hour))
				dl.UpdateUISession(id, "johndoe", true)
				atomic.StoreInt64(&sessID, int64(id))
			},
			requestFunc: func() *http.Request {
				s := securecookie.New(authKey[:], encKey[:])
				enc, _ := s.Encode(uiSessionName, map[interface{}]interface{}{cookieIDkey: int(atomic.LoadInt64(&sessID))})
				req, _ := http.NewRequest("GET", "https://example.com/some/path", nil)
				req.AddCookie(&http.Cookie{
					Name:  uiSessionName,
					Value: enc,
					Path:  "/",
				})
				return req
			},
			verifyfunc: expectOK,
		},
		{
			name: "unauthenticated",
			fields: fields{
				cookiekeys: [2][32]byte{authKey, encKey},
			},
			dbSetupFunc: func(dl persistence.UISessionsDataLayer) {},
			requestFunc: func() *http.Request {
				req, _ := http.NewRequest("GET", "https://example.com/some/path", nil)
				return req
			},
			verifyfunc: expectAuthRedirect,
		},
		{
			name: "bad keys",
			fields: fields{
				cookiekeys: [2][32]byte{authKey, encKey},
			},
			dbSetupFunc: func(dl persistence.UISessionsDataLayer) {},
			requestFunc: func() *http.Request {
				badAuthKey, badEncKey := randomCookieKey(), randomCookieKey()
				s := securecookie.New(badAuthKey[:], badEncKey[:])
				enc, _ := s.Encode(uiSessionName, map[interface{}]interface{}{cookieIDkey: int(atomic.LoadInt64(&sessID))})
				req, _ := http.NewRequest("GET", "https://example.com/some/path", nil)
				req.AddCookie(&http.Cookie{
					Name:  uiSessionName,
					Value: enc,
					Path:  "/",
				})
				return req
			},
			verifyfunc: expectAuthRedirect,
		},
		{
			name: "missing id",
			fields: fields{
				cookiekeys: [2][32]byte{authKey, encKey},
			},
			dbSetupFunc: func(dl persistence.UISessionsDataLayer) {},
			requestFunc: func() *http.Request {
				s := securecookie.New(authKey[:], encKey[:])
				cookieKey := "invalid"
				enc, _ := s.Encode(uiSessionName, map[interface{}]interface{}{cookieKey: 0})
				req, _ := http.NewRequest("GET", "https://example.com/some/path", nil)
				req.AddCookie(&http.Cookie{
					Name:  uiSessionName,
					Value: enc,
					Path:  "/",
				})
				return req
			},
			verifyfunc: expectAuthRedirect,
		},
		{
			name: "missing session in db",
			fields: fields{
				cookiekeys: [2][32]byte{authKey, encKey},
			},
			dbSetupFunc: func(dl persistence.UISessionsDataLayer) {},
			requestFunc: func() *http.Request {
				s := securecookie.New(authKey[:], encKey[:])
				enc, _ := s.Encode(uiSessionName, map[interface{}]interface{}{cookieIDkey: int(atomic.LoadInt64(&sessID))})
				req, _ := http.NewRequest("GET", "https://example.com/some/path", nil)
				req.AddCookie(&http.Cookie{
					Name:  uiSessionName,
					Value: enc,
					Path:  "/",
				})
				return req
			},
			verifyfunc: expectAuthRedirect,
		},
		{
			name: "expired session",
			fields: fields{
				cookiekeys: [2][32]byte{authKey, encKey},
			},
			dbSetupFunc: func(dl persistence.UISessionsDataLayer) {
				id, _ := dl.CreateUISession("/some/other/path", []byte("asdf"), net.ParseIP("10.0.0.0"), "some agent", time.Now().UTC().Add(1*time.Millisecond))
				dl.UpdateUISession(id, "johndoe", true)
				atomic.StoreInt64(&sessID, int64(id))
			},
			requestFunc: func() *http.Request {
				s := securecookie.New(authKey[:], encKey[:])
				enc, _ := s.Encode(uiSessionName, map[interface{}]interface{}{cookieIDkey: int(atomic.LoadInt64(&sessID))})
				req, _ := http.NewRequest("GET", "https://example.com/some/path", nil)
				req.AddCookie(&http.Cookie{
					Name:  uiSessionName,
					Value: enc,
					Path:  "/",
				})
				time.Sleep(2 * time.Millisecond)
				return req
			},
			verifyfunc: expectAuthRedirect,
		},
		{
			// verifies that the redirect is correct and proper session state is set up
			name: "redirect",
			fields: fields{
				cookiekeys: [2][32]byte{authKey, encKey},
			},
			dbSetupFunc: func(dl persistence.UISessionsDataLayer) {},
			requestFunc: func() *http.Request {
				req, _ := http.NewRequest("GET", "https://example.com/some/protected/path", nil)
				req.RemoteAddr = "10.0.0.0:2000"
				req.Header.Add("User-Agent", "some useragent")
				return req
			},
			verifyfunc: func(rc *httptest.ResponseRecorder, dl persistence.UISessionsDataLayer) error {
				if err := expectStatus(rc, 302); err != nil {
					return err
				}
				uis, err := dl.GetUISession(1)
				if err != nil {
					return errors.Wrap(err, "error getting session")
				}
				if uis == nil {
					return errors.New("session missing from db")
				}
				if uis.Authenticated {
					return errors.New("session should not be authenticated yet")
				}
				if time.Now().UTC().After(uis.Expires) {
					return fmt.Errorf("session should not be expired: %v", uis.Expires)
				}
				if uis.GitHubUser != "" {
					return fmt.Errorf("github user shouldn't be set: %v", uis.GitHubUser)
				}
				if uis.ClientIP != "10.0.0.0" {
					return fmt.Errorf("bad client ip: %v", uis.ClientIP)
				}
				if uis.UserAgent != "some useragent" {
					return fmt.Errorf("bad user agent: %v", uis.UserAgent)
				}
				if uis.TargetRoute != "/some/protected/path" {
					return fmt.Errorf("bad target route: %v", uis.TargetRoute)
				}
				rurl, err := url.Parse(rc.Result().Header.Get("Location"))
				if err != nil {
					return errors.Wrap(err, "error parsing redirect location")
				}
				if rurl.Scheme != "https" {
					return fmt.Errorf("bad redirect scheme: %v", rurl.Scheme)
				}
				if rurl.Host != "github.com" {
					return fmt.Errorf("bad redirect host: %v", rurl.Host)
				}
				if rurl.Path != "/some/path" {
					return fmt.Errorf("bad redirect path: %v", rurl.Path)
				}
				if rs := rurl.Query().Get("state"); rs != string(uis.State) {
					return fmt.Errorf("bad redirect state: %v", rs)
				}
				return nil
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dl := persistence.NewFakeDataLayer()
			tt.dbSetupFunc(dl)
			uai, err := newUIAuthRequest(dl, "https://github.com/some/path", tt.fields.cookiekeys[0], tt.fields.cookiekeys[1])
			if err != nil {
				t.Fatalf("error creating auth request: %v", err)
			}
			rc := httptest.NewRecorder()
			h := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }
			mh := middlewareChain(h, uai.uiAuth)
			mh(rc, tt.requestFunc())
			if err := tt.verifyfunc(rc, dl); err != nil {
				if !tt.wantErr {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}
