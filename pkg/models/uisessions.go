package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
)

type UISession struct {
	ID            int         `json:"id"`
	Created       time.Time   `json:"created"`
	Updated       pq.NullTime `json:"updated"`
	Expires       time.Time   `json:"expires"`
	State         []byte      `json:"state"`
	GitHubUser    string      `json:"github_user"`
	TargetRoute   string      `json:"target_route"`
	ClientIP      string      `json:"client_ip"`
	UserAgent     string      `json:"user_agent"`
	Authenticated bool        `json:"authenticated"`
}

func (uis UISession) Columns() string {
	return strings.Join([]string{"id", "created", "updated", "expires", "state", "github_user", "target_route", "authenticated", "client_ip", "user_agent"}, ",")
}

func (uis UISession) InsertColumns() string {
	return strings.Join([]string{"expires", "state", "github_user", "target_route", "authenticated", "client_ip", "user_agent"}, ",")
}

func (uis *UISession) ScanValues() []interface{} {
	return []interface{}{&uis.ID, &uis.Created, &uis.Updated, &uis.Expires, &uis.State, &uis.GitHubUser, &uis.TargetRoute, &uis.Authenticated, &uis.ClientIP, &uis.UserAgent}
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

func (uis UISession) IsExpired() bool {
	return time.Now().UTC().After(uis.Expires)
}
func (uis UISession) IsValid() bool {
	return !uis.IsExpired() && uis.Authenticated && uis.GitHubUser != "" && len(uis.State) != 0
}
