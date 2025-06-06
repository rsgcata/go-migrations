// Package execution provides functionality for tracking and persisting the execution state of migrations.
//
// This package defines the MigrationExecution struct to represent the execution state of a migration,
// and the Repository interface for persisting these states. It also includes an in-memory implementation
// of the Repository interface that can be used for testing.
//
// The execution state of a migration includes when it started, when it finished (if it has finished),
// and its version number. This information is used to determine which migrations have been applied
// and which ones need to be applied.
package execution

import (
	"time"

	"github.com/rsgcata/go-migrations/migration"
)

// MigrationExecution represents the execution state of a migration.
// Each MigrationExecution has a one-to-one relationship with a migration file,
// linked via the migration version number (migration identifier).
type MigrationExecution struct {
	// Version is the unique identifier of the migration, matching the Migration.Version() value
	Version uint64

	// ExecutedAtMs is the Unix timestamp in milliseconds when the migration execution started
	ExecutedAtMs uint64

	// FinishedAtMs is the Unix timestamp in milliseconds when the migration execution finished
	// A value of 0 indicates that the migration has not finished yet
	FinishedAtMs uint64
}

// StartExecution creates a new MigrationExecution for the given migration and marks it as unfinished.
// It sets the Version to the migration's version and ExecutedAtMs to the current time.
//
// Parameters:
//   - migration: The migration to create an execution for
//
// Returns:
//   - *MigrationExecution: A new execution instance for the migration
func StartExecution(migration migration.Migration) *MigrationExecution {
	return &MigrationExecution{migration.Version(), uint64(time.Now().UnixMilli()), 0}
}

// FinishExecution marks the MigrationExecution as finished by setting FinishedAtMs to the current time.
// If the execution is already marked as finished, this method does nothing.
func (execution *MigrationExecution) FinishExecution() {
	if !execution.Finished() {
		execution.FinishedAtMs = uint64(time.Now().UnixMilli())
	}
}

// Finished checks if the MigrationExecution has been marked as finished.
// An execution is considered finished if FinishedAtMs is greater than 0.
//
// Returns:
//   - bool: true if the execution is finished, false otherwise
func (execution *MigrationExecution) Finished() bool {
	return execution.FinishedAtMs > 0
}

// Repository defines the interface for storing and retrieving migration execution states.
// Any storage mechanism (SQL database, NoSQL database, file system, etc.) must implement
// this interface to be used with the migration system.
type Repository interface {
	// Init initializes the repository, setting up any necessary structures.
	// For SQL databases, this might involve creating tables. For file-based
	// repositories, this might involve creating directories.
	//
	// Returns:
	//   - error: An error if initialization fails
	Init() error

	// LoadExecutions retrieves all persisted migration executions from the repository.
	//
	// Returns:
	//   - []MigrationExecution: A slice of all migration executions
	//   - error: An error if loading fails
	LoadExecutions() ([]MigrationExecution, error)

	// Save persists a migration execution to the repository.
	// If an execution with the same version already exists, it should be updated.
	//
	// Parameters:
	//   - execution: The migration execution to save
	//
	// Returns:
	//   - error: An error if saving fails
	Save(execution MigrationExecution) error

	// Remove deletes a migration execution from the repository.
	//
	// Parameters:
	//   - execution: The migration execution to remove
	//
	// Returns:
	//   - error: An error if removal fails
	Remove(execution MigrationExecution) error

	// FindOne retrieves a specific migration execution by its version.
	//
	// Parameters:
	//   - version: The version of the migration execution to find
	//
	// Returns:
	//   - *MigrationExecution: The found migration execution, or nil if not found
	//   - error: An error if the search fails
	FindOne(version uint64) (*MigrationExecution, error)
}

// InMemoryRepository is an in-memory implementation of the Repository interface.
// It's primarily intended for use in unit tests, as it doesn't persist data between application restarts.
// Each of the error fields can be set to force the corresponding method to return that error,
// which is useful for testing error handling.
type InMemoryRepository struct {
	// InitErr is returned by the Init method if set
	InitErr error

	// LoadErr is returned by the LoadExecutions method if set
	LoadErr error

	// SaveErr is returned by the Save method if set
	SaveErr error

	// RemoveErr is returned by the Remove method if set
	RemoveErr error

	// FindOneErr is returned by the FindOne method if set
	FindOneErr error

	// PersistedExecutions holds the migration executions in memory
	PersistedExecutions []MigrationExecution
}

// Init implements the Repository.Init method.
// It simply returns the InitErr field, which can be set to simulate initialization errors.
func (repo *InMemoryRepository) Init() error {
	return repo.InitErr
}

// LoadExecutions implements the Repository.LoadExecutions method.
// It returns the PersistedExecutions slice and the LoadErr field.
func (repo *InMemoryRepository) LoadExecutions() ([]MigrationExecution, error) {
	return repo.PersistedExecutions, repo.LoadErr
}

// Save implements the Repository.Save method.
// It appends the execution to the PersistedExecutions slice and returns the SaveErr field.
func (repo *InMemoryRepository) Save(execution MigrationExecution) error {
	repo.PersistedExecutions = append(repo.PersistedExecutions, execution)
	return repo.SaveErr
}

// Remove implements the Repository.Remove method.
// It removes the execution with the matching version from the PersistedExecutions slice
// and returns the RemoveErr field.
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

// FindOne implements the Repository.FindOne method.
// It searches for an execution with the matching version in the PersistedExecutions slice
// and returns it along with the FindOneErr field.
func (repo *InMemoryRepository) FindOne(version uint64) (*MigrationExecution, error) {
	for _, e := range repo.PersistedExecutions {
		if e.Version == version {
			return &e, repo.FindOneErr
		}
	}
	return nil, repo.FindOneErr
}

// SaveAll is a convenience method that saves multiple executions at once.
// It calls Save for each execution in the provided slice.
func (repo *InMemoryRepository) SaveAll(executions []MigrationExecution) {
	for _, execution := range executions {
		_ = repo.Save(execution)
	}
}
