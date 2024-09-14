package repository

import (
	"context"
	"database/sql"
	"errors"

	_ "github.com/go-sql-driver/mysql"
	"github.com/rsgcata/go-migrations/execution"
)

type MysqlHandler struct {
	db        *sql.DB
	tableName string
	ctx       context.Context
}

func newDbHandle(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)

	if db == nil {
		return nil, err
	}

	db.SetMaxIdleConns(1)
	db.SetMaxOpenConns(1)
	db.SetConnMaxIdleTime(0)
	db.SetConnMaxLifetime(0)
	return db, err
}

func NewMysqlHandler(
	dsn string,
	tableName string,
	ctx context.Context,
) (*MysqlHandler, error) {
	db, err := newDbHandle(dsn)

	if err != nil {
		return nil, err
	}

	return &MysqlHandler{db, tableName, ctx}, nil
}

func (h *MysqlHandler) Context() context.Context {
	return h.ctx
}

func (h *MysqlHandler) Init() error {
	_, err := h.db.ExecContext(
		h.ctx,
		"CREATE TABLE IF NOT EXISTS `"+h.tableName+"` ("+
			"`version` BIGINT UNSIGNED NOT NULL,"+
			"`executed_at_ms` BIGINT UNSIGNED NOT NULL,"+
			"`finished_at_ms` BIGINT UNSIGNED NOT NULL,"+
			"PRIMARY KEY (`version`)"+
			") ENGINE=InnoDB CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci",
	)
	return err
}

func (h *MysqlHandler) LoadExecutions() (executions []execution.MigrationExecution, err error) {
	rows, err := h.db.QueryContext(
		h.ctx,
		"SELECT SQL_NO_CACHE * FROM `"+h.tableName+"`",
	)

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

func (h *MysqlHandler) Save(execution execution.MigrationExecution) error {
	_, err := h.db.ExecContext(
		h.ctx,
		"INSERT INTO `"+h.tableName+"` VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE "+
			" `executed_at_ms` = VALUES(`executed_at_ms`), "+
			" `finished_at_ms` = VALUES(`finished_at_ms`)",
		execution.Version, execution.ExecutedAtMs, execution.FinishedAtMs,
	)
	return err
}

func (h *MysqlHandler) Remove(execution execution.MigrationExecution) error {
	_, err := h.db.ExecContext(
		h.ctx,
		"DELETE FROM `"+h.tableName+"` WHERE `version` = ?",
		execution.Version,
	)
	return err
}

func (h *MysqlHandler) FindOne(version uint64) (*execution.MigrationExecution, error) {
	row := h.db.QueryRowContext(
		h.ctx,
		"SELECT SQL_NO_CACHE * FROM `"+h.tableName+"` WHERE `version` = ?",
		version,
	)

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
