package execution

import (
	"time"

	"github.com/rsgcata/go-migrations/migration"
)

type MigrationExecution struct {
	Version      uint64
	ExecutedAtMs uint64
	FinishedAtMs uint64
}

func StartExecution(migration migration.Migration) *MigrationExecution {
	return &MigrationExecution{migration.Version(), uint64(time.Now().UnixMilli()), 0}
}

func (execution *MigrationExecution) FinishExecution() {
	if !execution.Finished() {
		execution.FinishedAtMs = uint64(time.Now().UnixMilli())
	}
}

func (execution MigrationExecution) Finished() bool {
	return execution.FinishedAtMs > 0
}

type Repository interface {
	Init() error
	Lock() error
	Unlock() error
	LoadExecutions() ([]MigrationExecution, error)
	Save(execution MigrationExecution) error
	Remove(execution MigrationExecution) error
}
