package persistence

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/pkg/errors"

	"github.com/dollarshaveclub/acyl/pkg/models"
)

// CreateAPIKey creates a new api key for github user
func (pg *PGLayer) CreateAPIKey() (int, error) {
	apik := models.APIKey{

	}
	q := `INSERT INTO ui_sessions (` + apik.InsertColumns() + `) VALUES (` + apik.InsertParams() + `) RETURNING id;`
	var id int
	if err := pg.db.QueryRow(q, apik.InsertValues()...).Scan(&id); err != nil {
		return 0, errors.Wrap(err, "error inserting ui session")
	}
	return id, nil
}

// UpdateAPIKey updates the api key last used field
func (pg *PGLayer) UpdateAPIKey(id uuid.UUID, githubUser string) error {
	q := `UPDATE api_keys SET last_used = $1 WHERE api_key = $2 AND github_user = $3;`
	_, err := pg.db.Exec(q, pq.NullTime{Time: time.Now().UTC(), Valid: true}, id, githubUser)
	return errors.Wrap(err, "error updating api key")
}

// DeleteAPIKey unconditionally deletes the session with ID
func (pg *PGLayer) DeleteAPIKey(id uuid.UUID, githubUser string) error {
	q := `DELETE FROM api_keys WHERE id = $1 AND github_user = 2;`
	_, err := pg.db.Exec(q, id, githubUser)
	return errors.Wrap(err, "error deleting api key")
}

// GetAPIKeys returns the api key for the name and github user or nil if not found
// TODO: consider returning all API Keys for github user
func (pg *PGLayer) GetAPIKey(name, githubUser string) (*models.APIKey, error) {
	out := &models.APIKey{}
	q := `SELECT ` + out.Columns() + ` FROM api_keys WHERE name = $1 AND github_user = $2;`
	if err := pg.db.QueryRow(q, name, githubUser).Scan(out.ScanValues()...); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, errors.Wrap(err, "error getting api key")
	}
	return out, nil
}
