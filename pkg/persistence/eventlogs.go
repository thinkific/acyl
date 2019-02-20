package persistence

import (
	"database/sql"

	"github.com/dollarshaveclub/acyl/pkg/models"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

// GetEventLogByID returns a single EventLog by id, or nil or error
func (pg *PGLayer) GetEventLogByID(id uuid.UUID) (*models.EventLog, error) {
	out := &models.EventLog{}
	q := `SELECT ` + out.Columns() + ` FROM event_logs WHERE id = $1;`
	if err := pg.db.QueryRow(q, id).Scan(out.ScanValues()...); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, errors.Wrap(err, "error getting event log by id")
	}
	return out, nil
}

// GetEventLogsByEnvName gets all EventLogs associated with an environment
func (pg *PGLayer) GetEventLogsByEnvName(name string) ([]models.EventLog, error) {
	q := `SELECT ` + models.EventLog{}.Columns() + ` FROM event_logs WHERE env_name = $1;`
	return collectEventLogRows(pg.db.Query(q, name))
}

// GetEventLogsByRepoAndPR gets all EventLogs associated with a repo and PR, sorted in reverse created order (newest first)
func (pg *PGLayer) GetEventLogsByRepoAndPR(repo string, pr uint) ([]models.EventLog, error) {
	q := `SELECT ` + models.EventLog{}.Columns() + ` FROM event_logs WHERE repo = $1 AND pull_request = $2 ORDER BY created DESC;`
	return collectEventLogRows(pg.db.Query(q, repo, pr))
}

// CreateEventLog creates a new EventLog
func (pg *PGLayer) CreateEventLog(elog *models.EventLog) error {
	q := `INSERT INTO event_logs (` + elog.InsertColumns() + `) VALUES (` + elog.InsertParams() + `);`
	_, err := pg.db.Exec(q, elog.InsertValues()...)
	return errors.Wrap(err, "error inserting event log")
}

// AppendToEventLog appends a new log line to an EventLog
func (pg *PGLayer) AppendToEventLog(id uuid.UUID, msg string) error {
	q := `UPDATE event_logs SET log = array_append(log, $1) WHERE id = $2;`
	_, err := pg.db.Exec(q, msg, id)
	return errors.Wrap(err, "error appending to event log")
}

// SetEventLogEnvName sets the env name for an EventLog
func (pg *PGLayer) SetEventLogEnvName(id uuid.UUID, name string) error {
	q := `UPDATE event_logs SET env_name = $1 WHERE id = $2;`
	_, err := pg.db.Exec(q, name, id)
	return errors.Wrap(err, "error setting eventlog env name")
}

// DeleteEventLog deletes an EventLog
func (pg *PGLayer) DeleteEventLog(id uuid.UUID) error {
	q := `DELETE FROM event_logs WHERE id = $1;`
	_, err := pg.db.Exec(q, id)
	return errors.Wrap(err, "error deleting event log")
}

// DeleteEventLogsByEnvName deletes all EventLogs associated with an environment and returns the number of EventLogs deleted, or error
func (pg *PGLayer) DeleteEventLogsByEnvName(name string) (uint, error) {
	q := `DELETE FROM event_logs WHERE env_name = $1;`
	res, err := pg.db.Exec(q, name)
	n, _ := res.RowsAffected()
	return uint(n), errors.Wrap(err, "error deleting event logs by env name")
}

// DeleteEventLogsByRepoAndPR deletes all EventLogs associated with a repo and PR and returns the number of EventLogs deleted, or error
func (pg *PGLayer) DeleteEventLogsByRepoAndPR(repo string, pr uint) (uint, error) {
	q := `DELETE FROM event_logs WHERE repo = $1 AND pull_request = $2;`
	res, err := pg.db.Exec(q, repo, pr)
	n, _ := res.RowsAffected()
	return uint(n), errors.Wrap(err, "error deleting event logs by repo and pr")
}

func collectEventLogRows(rows *sql.Rows, err error) ([]models.EventLog, error) {
	var logs []models.EventLog
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, errors.Wrapf(err, "error querying")
	}
	defer rows.Close()
	for rows.Next() {
		el := models.EventLog{}
		if err := rows.Scan(el.ScanValues()...); err != nil {
			return nil, errors.Wrap(err, "error scanning row")
		}
		logs = append(logs, el)
	}
	return logs, nil
}
