//go:build postgres

package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/lib/pq"
	"github.com/rsgcata/go-migrations/execution"
)

// PostgresHandler Repository implementation for PostgresSQL integration
type PostgresHandler struct {
	db        *sql.DB
	tableName string
	ctx       context.Context
}

// NewPostgresHandler Builds a new PostgresHandler. If db is nil, it will try to build a db handle
// from the provided dsn. It's preferable to not share the db handle used by the handler with
// the one you pass in your migrations (this way, db sessions will not be mixed)
func NewPostgresHandler(
	dsn string,
	tableName string,
	ctx context.Context,
	db *sql.DB,
) (*PostgresHandler, error) {
	if db == nil {
		var err error
		db, err = newDbHandle(dsn, "postgres")

		if err != nil {
			return nil, err
		}
	}

	return &PostgresHandler{db, tableName, ctx}, nil
}

func (h *PostgresHandler) Context() context.Context {
	return h.ctx
}

func (h *PostgresHandler) Init() error {
	query := fmt.Sprintf(
		`
		CREATE TABLE IF NOT EXISTS "%s" (
			version BIGINT NOT NULL,
			executed_at_ms BIGINT NOT NULL,
			finished_at_ms BIGINT NOT NULL,
			PRIMARY KEY (version)
		)
		`,
		h.tableName,
	)

	_, err := h.db.ExecContext(h.ctx, query)
	return err
}

func (h *PostgresHandler) LoadExecutions() (executions []execution.MigrationExecution, err error) {
	query := fmt.Sprintf(`SELECT * FROM "%s"`, h.tableName)
	rows, err := h.db.QueryContext(h.ctx, query)

	if err != nil {
		return executions, err
	}

	defer func(rows *sql.Rows) {
		if closeErr := rows.Close(); closeErr != nil && err != nil {
			err = errors.Join(err, closeErr)
		}
	}(rows)

	for rows.Next() {
		var exec execution.MigrationExecution
		if err = rows.Scan(&exec.Version, &exec.ExecutedAtMs, &exec.FinishedAtMs); err != nil {
			return executions, err
		}
		executions = append(executions, exec)
	}

	err = rows.Err()
	return executions, err
}

func (h *PostgresHandler) Save(execution execution.MigrationExecution) error {
	// PostgresSQL uses ON CONFLICT for upsert operations
	query := fmt.Sprintf(
		`
		INSERT INTO "%s" (version, executed_at_ms, finished_at_ms) 
		VALUES ($1, $2, $3) 
		ON CONFLICT (version) DO UPDATE SET 
		executed_at_ms = $2, 
		finished_at_ms = $3
		`,
		h.tableName,
	)

	_, err := h.db.ExecContext(
		h.ctx,
		query,
		execution.Version, execution.ExecutedAtMs, execution.FinishedAtMs,
	)
	return err
}

func (h *PostgresHandler) Remove(execution execution.MigrationExecution) error {
	query := fmt.Sprintf(`DELETE FROM "%s" WHERE version = $1`, h.tableName)
	_, err := h.db.ExecContext(h.ctx, query, execution.Version)
	return err
}

func (h *PostgresHandler) FindOne(version uint64) (*execution.MigrationExecution, error) {
	query := fmt.Sprintf(`SELECT * FROM "%s" WHERE version = $1`, h.tableName)
	row := h.db.QueryRowContext(h.ctx, query, version)

	if row == nil {
		return nil, nil
	}

	var exec execution.MigrationExecution
	err := row.Scan(&exec.Version, &exec.ExecutedAtMs, &exec.FinishedAtMs)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return &exec, row.Err()
}
