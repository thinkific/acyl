package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type APIKey struct {
	APIKey           uuid.UUID   `json:"api_key"`
	Created          time.Time   `json:"created"`
	LastUsed         pq.NullTime `json:"last_used"`
	PermissionsLevel uint        `json:"permissions_level"`
	Name             string      `json:"name"`
	Description      string      `json:"description"`
	GitHubUser       string      `json:"github_user"`
}

func (apik APIKey) Columns() string {
	return strings.Join([]string{"api_key", "created", "last_used", "permissions_level", "name", "description", "github_user"}, ",")
}

func (apik APIKey) InsertColumns() string {
	return strings.Join([]string{"api_key", "permissions_level", "name", "description", "github_user"}, ",")
}

func (apik *APIKey) ScanValues() []interface{} {
	return []interface{}{&apik.APIKey, &apik.Created, &apik.LastUsed, &apik.PermissionsLevel, &apik.Name, &apik.Description, &apik.GitHubUser}
}

func (apik *APIKey) InsertValues() []interface{} {
	return []interface{}{&apik.APIKey, &apik.PermissionsLevel, &apik.Name, &apik.Description, &apik.GitHubUser}
}

func (apik APIKey) InsertParams() string {
	params := []string{}
	for i := range strings.Split(apik.InsertColumns(), ",") {
		params = append(params, fmt.Sprintf("$%v", i+1))
	}
	return strings.Join(params, ", ")
}
