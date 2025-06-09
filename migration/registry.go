package migration

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// MigrationsRegistry allows implementations to manage a collection of migration files.
// Implementations should act as a single source for all created migrations.
type MigrationsRegistry interface {
	// Register must push a migration in the registry. It should fail with error if the
	// migration can't be registered, for example, it its version overlaps with an
	// already registered migration
	Register(migration Migration) error

	// OrderedVersions must return a list of all registered migration versions,
	// ordered in ascending order. Can be used to determine the order in which the migrations
	// should run.
	OrderedVersions() []uint64

	// OrderedMigrations must return a list of all registered migrations,
	// ordered in ascending order by using their version. Can be used to determine the
	// order in which the migrations should run.
	OrderedMigrations() []Migration

	// Get must find and return the migration from the registry, by using its version.
	Get(version uint64) Migration

	// Count must return the total number of registered migrations.
	Count() int
}

// GenericRegistry is a generic implementation for MigrationsRegistry
type GenericRegistry struct {
	migrations map[uint64]Migration
}

// NewGenericRegistry creates a new, empty registry
func NewGenericRegistry() *GenericRegistry {
	return &GenericRegistry{make(map[uint64]Migration)}
}

func (registry *GenericRegistry) Register(migration Migration) error {
	if _, ok := registry.migrations[migration.Version()]; ok {
		return errors.New(
			"failed to register new migration. The migration is already registered",
		)
	}

	registry.migrations[migration.Version()] = migration
	return nil
}

func (registry *GenericRegistry) OrderedVersions() []uint64 {
	var versions []uint64
	for _, mig := range registry.migrations {
		versions = append(versions, mig.Version())
	}
	slices.Sort(versions)
	return versions
}

func (registry *GenericRegistry) OrderedMigrations() []Migration {
	var orderedMigrations []Migration
	for _, mig := range registry.migrations {
		orderedMigrations = append(orderedMigrations, mig)
	}

	sort.Slice(
		orderedMigrations, func(i, j int) bool {
			return orderedMigrations[i].Version() < orderedMigrations[j].Version()
		},
	)

	return orderedMigrations
}

func (registry *GenericRegistry) Get(version uint64) Migration {
	if mig, ok := registry.migrations[version]; ok {
		return mig
	}
	return nil
}

func (registry *GenericRegistry) Count() int {
	return len(registry.migrations)
}

// DirMigrationsRegistry is an implementation of MigrationsRegistry. It will include
// all migrations available in the specified directory (see struct builder function, there
// you can specify the used directory).
type DirMigrationsRegistry struct {
	GenericRegistry
	dirPath MigrationsDirPath
}

// NewEmptyDirMigrationsRegistry builds an empty migrations registry which can be used
// for the use case where migrations are saved in a directory.
func NewEmptyDirMigrationsRegistry(dirPath MigrationsDirPath) *DirMigrationsRegistry {
	return &DirMigrationsRegistry{*NewGenericRegistry(), dirPath}
}

// NewDirMigrationsRegistry builds a migrations registry with all migrations available
// in the specified directory. Panics if it detects that allMigrations argument does not
// match with whatever migration files exist in the specified dirPath
func NewDirMigrationsRegistry(
	dirPath MigrationsDirPath,
	allMigrations []Migration,
) *DirMigrationsRegistry {
	migRegistry := NewEmptyDirMigrationsRegistry(dirPath)

	for _, mig := range allMigrations {
		if regErr := migRegistry.Register(mig); regErr != nil {
			panic(
				fmt.Errorf(
					"failed to register migration %d: %w", mig.Version(), regErr,
				),
			)
		}
	}

	migRegistry.AssertValidRegistry()
	return migRegistry
}

// HasAllMigrationsRegistered checks if everything from the migrations directory has been
// registered in the registry.
// If it returns false, next 2 return values show which file names are missing and which
// file names are extra, compare to the registered migrations.
// Errors if reading the directory fails (maybe insufficient permissions?)
func (registry *DirMigrationsRegistry) HasAllMigrationsRegistered() (
	bool, []string, []string, error,
) {
	dirEntries, err := os.ReadDir(string(registry.dirPath))
	if err != nil {
		return false, []string{}, []string{}, fmt.Errorf(
			"failed to check if all migrations have been registered."+
				" Dir entries read failed with error: %w", err,
		)
	}

	registeredCopy := make(map[uint64]Migration)
	for _, mig := range registry.migrations {
		registeredCopy[mig.Version()] = mig
	}

	var missing, extra []string
	for _, item := range dirEntries {
		if item.IsDir() || !strings.HasPrefix(item.Name(), FileNamePrefix+FileNameSeparator) {
			continue
		}

		fname := strings.TrimLeft(item.Name(), FileNamePrefix+FileNameSeparator)
		version, err := strconv.Atoi(strings.TrimRight(fname, ".go"))

		if err != nil {
			continue
		}

		if _, ok := registeredCopy[uint64(version)]; ok {
			delete(registeredCopy, uint64(version))
		} else {
			missing = append(missing, item.Name())
		}
	}

	for version := range registeredCopy {
		extra = append(extra, FileNamePrefix+FileNameSeparator+strconv.Itoa(int(version))+".go")
	}

	return len(missing) == 0 && len(extra) == 0, missing, extra, nil
}

// AssertValidRegistry checks if there are any issues with the list of registered
// migrations and panics if it finds any
func (registry *DirMigrationsRegistry) AssertValidRegistry() {
	allRegistered, notRegistered, extraRegistered, registryErr :=
		registry.HasAllMigrationsRegistered()

	if registryErr != nil {
		panic(fmt.Errorf("registry has invalid state: %w", registryErr))
	}

	if !allRegistered {
		notRegisteredMigrations := strings.Join(notRegistered, ", ")
		extraMigrations := strings.Join(extraRegistered, ", ")
		if notRegisteredMigrations == "" {
			notRegisteredMigrations = "none"
		}
		if extraMigrations == "" {
			extraMigrations = "none"
		}

		panic(
			fmt.Errorf(
				"registry has invalid state. %s. Not registered: %s. Extra migrations: %s",
				"You must register all migrations before running migrations",
				notRegisteredMigrations,
				extraMigrations,
			),
		)
	}
}
