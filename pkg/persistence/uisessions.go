package persistence

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/dollarshaveclub/acyl/pkg/models"
)

func (pg *PGLayer) CreateUISession(key, data, state []byte, expires time.Time) error {
	if len(key) == 0 || len(data) == 0 || len(state) == 0 {
		return errors.New("key, data and state are required")
	}
	if expires.IsZero() || time.Now().UTC().After(expires) {
		return fmt.Errorf("invalid expires time: %v", expires)
	}
	uis := models.UISession{
		Key:     key,
		Data:    data,
		State:   state,
		Expires: expires,
	}
	q := `INSERT INTO ui_sessions (` + uis.InsertColumns() + `) VALUES (` + uis.InsertParams() + `);`
	_, err := pg.db.Exec(q, uis.InsertValues()...)
	return errors.Wrap(err, "error inserting ui session")
}

func (pg *PGLayer) UpdateUISession(key, data []byte, expires time.Time) error {
	if len(key) == 0 || len(data) == 0 {
		return errors.New("key and data are required")
	}
	if expires.IsZero() || time.Now().UTC().After(expires) {
		return fmt.Errorf("invalid expires time: %v", expires)
	}
	q := `UPDATE ui_sessions SET data = $1, expires = $2 WHERE key = $3;`
	_, err := pg.db.Exec(q, data, expires, key)
	return errors.Wrap(err, "error updating ui session")
}

func (pg *PGLayer) DeleteUISession(key []byte) error {
	if len(key) == 0 {
		return errors.New("key is required")
	}
	q := `DELETE FROM ui_sessions WHERE key = $1;`
	_, err := pg.db.Exec(q, key)
	return errors.Wrap(err, "error deleting ui session")
}

func (pg *PGLayer) GetUISession(key []byte) (*models.UISession, error) {
	if len(key) == 0 {
		return nil, errors.New("key is required")
	}
	out := &models.UISession{}
	q := `SELECT ` + out.Columns() + ` FROM ui_sessions WHERE key = $1;`
	if err := pg.db.QueryRow(q, key).Scan(out.ScanValues()...); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, errors.Wrap(err, "error getting ui session")
	}
	return out, nil
}

func (pg *PGLayer) DeleteExpiredUISessions() error {
	_, err := pg.db.Exec(`DELETE FROM ui_sessions WHERE expires < now();`)
	return errors.Wrap(err, "error deleting expired ui sessions")
}
