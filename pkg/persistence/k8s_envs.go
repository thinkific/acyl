package persistence

import (
	"context"
	"database/sql"

	"github.com/dollarshaveclub/acyl/pkg/models"
	"github.com/pkg/errors"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

var _ K8sEnvDataLayer = &PGLayer{}

// GetK8sEnv gets a k8s environment by environment name
func (pg *PGLayer) GetK8sEnv(span tracer.Span, name string) (*models.KubernetesEnvironment, error) {
	out := &models.KubernetesEnvironment{}
	q := `SELECT ` + models.KubernetesEnvironment{}.Columns() + ` FROM kubernetes_environments WHERE env_name = $1;`
	ctx := tracer.ContextWithSpan(context.Background(), span)
	if err := pg.db.QueryRowContext(ctx, q, name).Scan(out.ScanValues()...); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, errors.Wrap(err, "error getting k8s environment")
	}
	return out, nil
}

// GetK8sEnvsByNamespace returns any KubernetesEnvironments associated with a specific namespace name
func (pg *PGLayer) GetK8sEnvsByNamespace(span tracer.Span, ns string) ([]models.KubernetesEnvironment, error) {
	q := `SELECT ` + models.KubernetesEnvironment{}.Columns() + ` FROM kubernetes_environments WHERE namespace = $1;`
	ctx := tracer.ContextWithSpan(context.Background(), span)
	return collectK8sEnvRows(pg.db.QueryContext(ctx, q, ns))
}

// CreateK8sEnv inserts a new k8s environment into the DB
func (pg *PGLayer) CreateK8sEnv(span tracer.Span, env *models.KubernetesEnvironment) error {
	q := `INSERT INTO kubernetes_environments (` + env.InsertColumns() + `) VALUES (` + env.InsertParams() + `);`
	ctx := tracer.ContextWithSpan(context.Background(), span)
	_, err := pg.db.ExecContext(ctx, q, env.InsertValues()...)
	return errors.Wrap(err, "error inserting k8s environment")
}

// DeleteK8sEnv deletes a k8s environment from the DB
func (pg *PGLayer) DeleteK8sEnv(span tracer.Span, name string) error {
	q := `DELETE FROM kubernetes_environments WHERE env_name = $1;`
	ctx := tracer.ContextWithSpan(context.Background(), span)
	_, err := pg.db.ExecContext(ctx, q, name)
	return errors.Wrap(err, "error deleting k8s environment")
}

// UpdateK8sEnvTillerAddr updates an existing k8s environment with the provided tiller address
func (pg *PGLayer) UpdateK8sEnvTillerAddr(span tracer.Span, envname, taddr string) error {
	q := `UPDATE kubernetes_environments SET tiller_addr = $1 WHERE env_name = $2;`
	ctx := tracer.ContextWithSpan(context.Background(), span)
	_, err := pg.db.ExecContext(ctx, q, taddr, envname)
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
