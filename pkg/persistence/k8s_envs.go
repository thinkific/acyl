package persistence

import (
	"database/sql"

	"github.com/dollarshaveclub/acyl/pkg/models"
	"github.com/pkg/errors"
)

var _ K8sEnvDataLayer = &PGLayer{}

// GetK8sEnv gets a k8s environment by environment name
func (pg *PGLayer) GetK8sEnv(name string) (*models.KubernetesEnvironment, error) {
	out := &models.KubernetesEnvironment{}
	q := `SELECT ` + models.KubernetesEnvironment{}.Columns() + ` FROM kubernetes_environments WHERE env_name = $1;`
	if err := pg.db.QueryRow(q, name).Scan(out.ScanValues()...); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, errors.Wrap(err, "error getting k8s environment")
	}
	return out, nil
}

// GetK8sEnvsByNamespace returns any KubernetesEnvironments associated with a specific namespace name
func (pg *PGLayer) GetK8sEnvsByNamespace(ns string) ([]models.KubernetesEnvironment, error) {
	q := `SELECT ` + models.KubernetesEnvironment{}.Columns() + ` FROM kubernetes_environments WHERE namespace = $1;`
	return collectK8sEnvRows(pg.db.Query(q, ns))
}

// CreateK8sEnv inserts a new k8s environment into the DB
func (pg *PGLayer) CreateK8sEnv(env *models.KubernetesEnvironment) error {
	q := `INSERT INTO kubernetes_environments (` + env.InsertColumns() + `) VALUES (` + env.InsertParams() + `);`
	_, err := pg.db.Exec(q, env.InsertValues()...)
	return errors.Wrap(err, "error inserting k8s environment")
}

// DeleteK8sEnv deletes a k8s environment from the DB
func (pg *PGLayer) DeleteK8sEnv(name string) error {
	q := `DELETE FROM kubernetes_environments WHERE env_name = $1;`
	_, err := pg.db.Exec(q, name)
	return errors.Wrap(err, "error deleting k8s environment")
}

// UpdateK8sEnvTillerAddr updates an existing k8s environment with the provided tiller address
func (pg *PGLayer) UpdateK8sEnvTillerAddr(envname, taddr string) error {
	q := `UPDATE kubernetes_environments SET tiller_addr = $1 WHERE env_name = $2;`
	_, err := pg.db.Exec(q, taddr, envname)
	return errors.Wrap(err, "error updating k8s environment")
}

func collectK8sEnvRows(rows *sql.Rows, err error) ([]models.KubernetesEnvironment, error) {
	var k8senvs []models.KubernetesEnvironment
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, errors.Wrapf(err, "error querying")
	}
	defer rows.Close()
	for rows.Next() {
		ke := models.KubernetesEnvironment{}
		if err := rows.Scan(ke.ScanValues()...); err != nil {
			return nil, errors.Wrap(err, "error scanning row")
		}
		k8senvs = append(k8senvs, ke)
	}
	return k8senvs, nil
}
