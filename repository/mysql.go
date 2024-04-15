package repository

import (
	"context"
	"database/sql"

	"github.com/rsgcata/go-migrations/execution"
)

type MysqlHandler struct {
	db        *sql.DB
	tableName string
}

func NewMysqlHandler(db *sql.DB, tableName string) *MysqlHandler {
	return &MysqlHandler{db, tableName}
}

func (h *MysqlHandler) Init() error {
	_, err := h.db.QueryContext(
		context.Background(),
		"CREATE TABLE IF NOT EXISTS `"+h.tableName+"` ("+
			"`version` BIGINT UNSIGNED NOT NULL,"+
			"`executed_at_ms` BIGINT UNSIGNED NOT NULL,"+
			"`finished_at_ms` BIGINT UNSIGNED NOT NULL,"+
			") egine=InnoDB, collate=utf8mb4_general_ci",
	)
	return err
}

func (h *MysqlHandler) Lock() error {
	_, err := h.db.QueryContext(
		context.Background(),
		"LOCK TABLES `"+h.tableName+"` WRITE",
	)

	return err
}

func (h *MysqlHandler) Unlock() error {
	_, err := h.db.QueryContext(
		context.Background(),
		"UNLOCK TABLES",
	)
	return err
}

func (h *MysqlHandler) LoadExecutions() ([]execution.MigrationExecution, error) {
	rows, err := h.db.QueryContext(
		context.Background(),
		"SELECT * FROM "+"`"+h.tableName+"`",
	)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var executions []execution.MigrationExecution
	for rows.Next() {
		var execution execution.MigrationExecution
		if err := rows.Scan(&execution.Version, &execution.ExecutedAtMs, &execution.FinishedAtMs); err != nil {
			return executions, err
		}
		executions = append(executions, execution)
	}
	if err = rows.Err(); err != nil {
		return executions, err
	}

	return nil, err
}

func (h *MysqlHandler) Save(execution execution.MigrationExecution) error {
	_, err := h.db.QueryContext(
		context.Background(),
		"INSERT INTO `"+h.tableName+"` VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE"+
			" `executed_at_ms` = VALUES(`executed_at_ms`), `finished_at_ms` = VALUES(`finished_at_ms`)",
		execution.Version, execution.ExecutedAtMs, execution.FinishedAtMs,
	)
	return err
}

func (h *MysqlHandler) Remove(execution execution.MigrationExecution) error {
	_, err := h.db.QueryContext(
		context.Background(),
		"DELETE FROM `"+h.tableName+"` WHERE `version` = ?",
		execution.Version,
	)
	return err
}
