package migration

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
)

type MigrationsRegistry interface {
	Register(migration Migration) error
	OrderedVersions() []uint64
	Get(version uint64) Migration
	Count() int
}

type GenericRegistry struct {
	migrations map[uint64]Migration
}

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

func (registry *GenericRegistry) Get(version uint64) Migration {
	if mig, ok := registry.migrations[version]; ok {
		return mig
	}
	return nil
}

func (registry *GenericRegistry) Count() int {
	return len(registry.migrations)
}

type DirMigrationsRegistry struct {
	GenericRegistry
	dirPath MigrationsDirPath
}

func NewDirMigrationsRegistry(dirPath MigrationsDirPath) *DirMigrationsRegistry {
	return &DirMigrationsRegistry{*NewGenericRegistry(), dirPath}
}

// Checks if everything from the migrations directory has been registered in the registry.
// If it returns false, next 2 return values show which file nanes are missing and which
// file names are extra, compare to the registered migrations.
// Errors if reading the directory fails (maybe insufficient permissions?)
func (registry *DirMigrationsRegistry) HasAllMigrationsRegistered() (
	bool, []string, []string, error,
) {
	dirEntries, err := os.ReadDir(string(registry.dirPath))
	if err != nil {
		return false, []string{}, []string{}, fmt.Errorf(
			"failed to check if all migrations have been registered. Dir entries read failed"+
				" with error: %w", err,
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
