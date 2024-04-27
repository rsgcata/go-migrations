package repository

import (
	"context"
	"database/sql"

	"github.com/rsgcata/go-migrations/execution"
)

type MysqlHandler struct {
	db        *sql.DB
	tableName string
	ctx       context.Context
}

func NewMysqlHandler(db *sql.DB, tableName string, ctx context.Context) *MysqlHandler {
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

func (h *MysqlHandler) Lock() error {
	_, err := h.db.ExecContext(h.ctx, "LOCK TABLES `"+h.tableName+"` WRITE")
	return err
}

func (h *MysqlHandler) Unlock() error {
	_, err := h.db.ExecContext(h.ctx, "UNLOCK TABLES")
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
	_, err := h.db.QueryContext(
		h.ctx,
		"DELETE FROM `"+h.tableName+"` WHERE `version` = ?",
		execution.Version,
	)
	return err
}
