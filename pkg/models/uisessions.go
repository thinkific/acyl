package models

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/pkg/errors"
	"golang.org/x/crypto/nacl/secretbox"
)

type UISession struct {
	ID                 int         `json:"id"`
	Created            time.Time   `json:"created"`
	Updated            pq.NullTime `json:"updated"`
	Expires            time.Time   `json:"expires"`
	State              []byte      `json:"state"`
	GitHubUser         string      `json:"github_user"`
	TargetRoute        string      `json:"target_route"`
	ClientIP           string      `json:"client_ip"`
	UserAgent          string      `json:"user_agent"`
	Authenticated      bool        `json:"authenticated"`
	EncryptedUserToken []byte      `json:"-"` // GitHub API user-scoped token encrypted w/ NaCl secretbox
}

func (uis UISession) Columns() string {
	return strings.Join([]string{"id", "created", "updated", "expires", "state", "github_user", "target_route", "authenticated", "client_ip", "user_agent", "encrypted_user_token"}, ",")
}

func (uis UISession) InsertColumns() string {
	return strings.Join([]string{"expires", "state", "github_user", "target_route", "authenticated", "client_ip", "user_agent"}, ",")
}

func (uis *UISession) ScanValues() []interface{} {
	return []interface{}{&uis.ID, &uis.Created, &uis.Updated, &uis.Expires, &uis.State, &uis.GitHubUser, &uis.TargetRoute, &uis.Authenticated, &uis.ClientIP, &uis.UserAgent, &uis.EncryptedUserToken}
}

func (uis *UISession) InsertValues() []interface{} {
	return []interface{}{&uis.Expires, &uis.State, &uis.GitHubUser, &uis.TargetRoute, &uis.Authenticated, &uis.ClientIP, &uis.UserAgent}
}

func (uis UISession) InsertParams() string {
	params := []string{}
	for i := range strings.Split(uis.InsertColumns(), ",") {
		params = append(params, fmt.Sprintf("$%v", i+1))
	}
	return strings.Join(params, ", ")
}

func (uis UISession) HasEncryptedToken() bool {
	return len(uis.EncryptedUserToken) > 0
}

func (uis UISession) IsExpired() bool {
	return time.Now().UTC().After(uis.Expires)
}
func (uis UISession) IsValid() bool {
	return !uis.IsExpired() && uis.Authenticated && uis.GitHubUser != "" && len(uis.State) != 0 && uis.HasEncryptedToken()
}

// EncryptUserToken takes a user token, encrypts it and sets EncryptedUserToken accordingly
func (uis *UISession) EncryptandSetUserToken(tkn []byte, key [32]byte) error {
	var nonce [24]byte
	if n, err := rand.Read(nonce[:]); err != nil || n != len(nonce) {
		return errors.Wrapf(err, "error reading random bytes for nonce (read: %v)", n)
	}
	uis.EncryptedUserToken = secretbox.Seal(nonce[:], tkn, &nonce, &key)
	return nil
}

// GetUserToken returns the decrypted user token using key or error
func (uis *UISession) GetUserToken(key [32]byte) (string, error) {
	var nonce [24]byte
	copy(nonce[:], uis.EncryptedUserToken[:24])
	tkn, ok := secretbox.Open(nil, uis.EncryptedUserToken[24:], &nonce, &key)
	if !ok {
		return "", errors.New("decryption error (incorrect key?)")
	}
	return string(tkn), nil
}
