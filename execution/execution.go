// Package execution includes migration execution structures and execution persistence related
// logic via Repository implementations.
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
type Repository interface {
	// Init Must handle the initialization phase of a repository/storage mechanism. This
	// can be setting up the database table if it's a sql type of storage
	Init() error

	// LoadExecutions Must return all persisted migration executions
	LoadExecutions() ([]MigrationExecution, error)

	// Save Must persist a migration execution
	Save(execution MigrationExecution) error

	// Remove Must remove a migration execution
	Remove(execution MigrationExecution) error

	// FindOne Must find a migration execution using the version as filter
	FindOne(version uint64) (*MigrationExecution, error)
}

// InMemoryRepository Implementation of Repository. Can be used in unit tests.
// All {method}Err properties can be used to force the specific method to return an error
type InMemoryRepository struct {
	InitErr             error
	LoadErr             error
	SaveErr             error
	RemoveErr           error
	FindOneErr          error
	PersistedExecutions []MigrationExecution
}

func (repo *InMemoryRepository) Init() error {
	return repo.InitErr
}

func (repo *InMemoryRepository) LoadExecutions() ([]MigrationExecution, error) {
	return repo.PersistedExecutions, repo.LoadErr
}

func (repo *InMemoryRepository) Save(execution MigrationExecution) error {
	repo.PersistedExecutions = append(repo.PersistedExecutions, execution)
	return repo.SaveErr
}

func (repo *InMemoryRepository) Remove(execution MigrationExecution) error {
	var newPersistedExecutions []MigrationExecution
	for _, e := range repo.PersistedExecutions {
		if e.Version != execution.Version {
			newPersistedExecutions = append(newPersistedExecutions, e)
		}
	}
	repo.PersistedExecutions = newPersistedExecutions
	return repo.RemoveErr
}

func (repo *InMemoryRepository) FindOne(version uint64) (*MigrationExecution, error) {
	for _, e := range repo.PersistedExecutions {
		if e.Version == version {
			return &e, repo.FindOneErr
		}
	}
	return nil, repo.FindOneErr
}

func (repo *InMemoryRepository) SaveAll(executions []MigrationExecution) {
	for _, execution := range executions {
		_ = repo.Save(execution)
	}
}
