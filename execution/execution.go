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

func (execution *MigrationExecution) EndExecution() {
	if execution.FinishedAtMs == 0 {
		execution.FinishedAtMs = uint64(time.Now().UnixMilli())
	}
}

func (execution MigrationExecution) ExecutionFinished() bool {
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

// type ExecutionsRegistry struct {
// 	availableMigrationsRegistry migration.MigrationsRegistry
// 	executedMigrations          []MigrationExecution
// }

// func NewExecutionsRegistry(
// 	availableMigrationsRegistry migration.MigrationsRegistry,
// 	executedMigrations []MigrationExecution,
// ) (*ExecutionsRegistry, error) {
// 	var executedVersions []uint64
// 	for _, version := range executedMigrations {
// 		executedVersions = append(executedVersions, version.Version)
// 		if availableMigrationsRegistry.Get(version.Version) == nil {
// 			return nil, errors.New(
// 				"could not create new executions registry. Executed version provided " +
// 					strconv.Itoa(int(version.Version)) +
// 					" does not exist in the available migrations",
// 			)
// 		}
// 	}

// 	slicedAvailableVersions := availableMigrationsRegistry.OrderedVersions()[0:len(executedMigrations)]
// 	slices.Sort(executedVersions)

// 	if !slices.Equal(slicedAvailableVersions, executedVersions) {
// 		return nil, errors.New(
// 			"could not create new executions registry. Executed versions provided" +
// 				" do not match with the available migrations. Check that the order and versions" +
// 				" for executed migrations match with the available migrations",
// 		)
// 	}

// 	return &ExecutionsRegistry{availableMigrationsRegistry, executedMigrations}, nil
// }

// func (registry *ExecutionsRegistry) Register(migration Migration) error {

// 	if slices.Contains(registry.executedMigrations, migration.Version()) {
// 		return errors.New(
// 			"failed to register new executed migration. The migration is laready registered",
// 		)
// 	}

// 	if registry.availableMigrationsRegistry.Get(migration.Version()) == nil {
// 		return errors.New(
// 			"failed to register new executed migration. The migration is missing from" +
// 				" all available migrations",
// 		)
// 	}

// 	nextVersion := registry.availableMigrationsRegistry.
// 		OrderedVersions()[len(registry.executedMigrations)]

// 	if nextVersion != migration.Version() {
// 		return errors.New(
// 			"failed to register new executed migration. The provided migration is not" +
// 				" the one that should be next",
// 		)
// 	}

// 	registry.executedMigrations = append(registry.executedMigrations, migration.Version())
// 	return nil
// }

// func (registry *ExecutionsRegistry) OrderedVersions() []uint64 {
// 	var executedVersions []uint64
// 	for _, version := range registry.executedMigrations {
// 		executedVersions = append(executedVersions, version.Version)
// 	}
// 	slices.Sort(executedVersions)
// 	return executedVersions
// }

// func (registry *ExecutionsRegistry) Get(version uint64) (Migration, repository.MigrationExecution) {
// 	registry.availableMigrationsRegistry
// 	if slices.Contains(registry.executedMigrations, version) {
// 		return registry.Get(version)
// 	}
// 	return nil
// }
