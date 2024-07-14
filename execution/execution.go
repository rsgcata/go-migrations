package execution

import (
	"time"

	"github.com/rsgcata/go-migrations/migration"
)

// MigrationExecution Struct that holds information about a migration execution.
// It has a 1 to 1 relation to a migration file, linked via the migration version number
// (migration identifier)
type MigrationExecution struct {
	Version      uint64
	ExecutedAtMs uint64
	FinishedAtMs uint64
}

// StartExecution Creates a new MigrationExecution and marks it as unfinished.
func StartExecution(migration migration.Migration) *MigrationExecution {
	return &MigrationExecution{migration.Version(), uint64(time.Now().UnixMilli()), 0}
}

// FinishExecution Marks the MigrationExecution as finished
func (execution *MigrationExecution) FinishExecution() {
	if !execution.Finished() {
		execution.FinishedAtMs = uint64(time.Now().UnixMilli())
	}
}

// Finished Helper function to see if the MigrationExecution is finished
func (execution *MigrationExecution) Finished() bool {
	return execution.FinishedAtMs > 0
}

// Repository Must be implemented by any storage mechanism and must handle everything related
// to migration executions persistence
// [Repository.Init] Must handle the initialization phase of a repository/storage mechanism. This
// can be setting up the database table if it's a sql type of storage or consistency checks between
// executions and migrations (for example, there may be executions persisted which do not have
// a match in the collection of registered migrations)
// [Repository.LoadExecutions] Must return all persisted migration executions
// [Repository.Save] Must persist a migration execution
// [Repository.Remove] Must remove a migration execution
// [Repository.FindOne] Must find a migration execution using the version as filter
type Repository interface {
	Init() error
	LoadExecutions() ([]MigrationExecution, error)
	Save(execution MigrationExecution) error
	Remove(execution MigrationExecution) error
	FindOne(version uint64) (*MigrationExecution, error)
}
