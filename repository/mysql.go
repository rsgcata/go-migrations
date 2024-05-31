package repository

import (
	"context"
	"database/sql"

	_ "github.com/go-sql-driver/mysql"
	"github.com/rsgcata/go-migrations/execution"
)

type MysqlHandler struct {
	db        *sql.DB
	tableName string
	ctx       context.Context
}

func NewDbHandle(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	db.SetMaxIdleConns(1)
	db.SetMaxOpenConns(1)
	db.SetConnMaxIdleTime(0)
	db.SetConnMaxLifetime(0)
	return db, err
}

// db should be a standalone, dedicated connection object. It should not be used by migrations.
// Using db for executions repository and migrations may cause unexpected behaviour when database
// locks are involved
func NewMysqlHandler(
	db *sql.DB,
	tableName string,
	ctx context.Context,
) *MysqlHandler {
	return &MysqlHandler{db, tableName, ctx}
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

func (h *MysqlHandler) LoadExecutions() ([]execution.MigrationExecution, error) {
	rows, err := h.db.QueryContext(h.ctx, "SELECT SQL_NO_CACHE * FROM "+"`"+h.tableName+"`")
	var executions []execution.MigrationExecution

	if err != nil {
		return executions, err
	}
	defer rows.Close()

	for rows.Next() {
		var execution execution.MigrationExecution
		if err := rows.Scan(&execution.Version, &execution.ExecutedAtMs, &execution.FinishedAtMs); err != nil {
			return executions, err
		}
		executions = append(executions, execution)
	}

	return executions, rows.Err()
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
		"SELECT SQL_NO_CACHE * FROM "+"`"+h.tableName+"` WHERE `version` = ?",
		version,
	)

	if row == nil {
		return nil, nil
	}

	var execution execution.MigrationExecution
	err := row.Scan(&execution.Version, &execution.ExecutedAtMs, &execution.FinishedAtMs)

	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return &execution, row.Err()
}
