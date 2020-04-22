package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
)

type UISession struct {
	ID      int         `json:"id"`
	Created time.Time   `json:"created"`
	Updated pq.NullTime `json:"updated"`
	Expires time.Time   `json:"expires"`
	Key     []byte      `json:"key"`
	Data    []byte      `json:"data"`
	State   []byte      `json:"state"`
}

func (uis UISession) Columns() string {
	return strings.Join([]string{"id", "created", "updated", "expires", "key", "data", "state"}, ",")
}

func (uis UISession) InsertColumns() string {
	return strings.Join([]string{"expires", "key", "data", "state"}, ",")
}

func (uis *UISession) ScanValues() []interface{} {
	return []interface{}{&uis.ID, &uis.Created, &uis.Updated, &uis.Expires, &uis.Key, &uis.Data, &uis.State}
}

func (uis *UISession) InsertValues() []interface{} {
	return []interface{}{&uis.Expires, &uis.Key, &uis.Data, &uis.State}
}

func (uis UISession) InsertParams() string {
	params := []string{}
	for i := range strings.Split(uis.InsertColumns(), ",") {
		params = append(params, fmt.Sprintf("$%v", i+1))
	}
	return strings.Join(params, ", ")
}
