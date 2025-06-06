// Package migration provides the core functionality for defining, generating, and managing database migrations.
//
// This package includes the Migration interface that all migrations must implement, as well as
// utilities for generating blank migration files and managing migration directories.
//
// Migrations are Go files that implement the Migration interface with Version(), Up(), and Down() methods.
// The Version() method returns a unique identifier for the migration, while Up() and Down() methods
// contain the logic to apply and roll back the migration, respectively.
package migration

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"text/template"
	"time"
)

// TmplContents File template to be used to generate a new, base migration file
// to make it easier for devs to create new migrations.
//
//go:embed migration.go.template
var TmplContents string

// FileNamePrefix File name prefix, static value, which will be set for all migration files.
const FileNamePrefix = "version"

// FileNameSeparator A separator used to separate words in a migration file.
const FileNameSeparator = "_"

// Migration Represents the base behavior a migration should include
type Migration interface {
	// Version must be a static, globally unique value which identifies the migration file
	// globally. It must match the value set in the file name. By default, this will be
	// the unix timestamp in seconds when the migration file was generated.
	Version() uint64

	// Up must include any code that will change the structure and/or state of your database.
	// Preferably, it should be idempotent and resilient. It should not use, set or modify
	// global state, to not impact other migration executions. You can have multiple migrations
	// Up() act as a unit, but, care should be taken when coordinating them (use save points
	// for example, and save them in a central place which can be used as a persistent
	// source of truth).
	Up() error

	// Down must include all necessary code that will roll back the changes made by the Up()
	// function. Care should be taken when designing your rollback strategy, to not lose any
	// critical database state. Other things mentioned for Up() function are valid for Down()
	// also.
	Down() error
}

// DummyMigration is a simple implementation of the Migration interface
// that can be used for testing purposes. It implements the Migration interface
// with no-op Up() and Down() methods.
type DummyMigration struct {
	version uint64 // The migration version number
}

// NewDummyMigration creates a new DummyMigration with the specified version number.
//
// Parameters:
//   - version: The unique version identifier for the migration
//
// Returns:
//   - *DummyMigration: A new DummyMigration instance
func NewDummyMigration(version uint64) *DummyMigration {
	return &DummyMigration{version: version}
}

// Version returns the version number of the DummyMigration.
// This implements the Migration.Version() method.
func (dm *DummyMigration) Version() uint64 {
	return dm.version
}

// Up is a no-op implementation of the Migration.Up() method.
// It always returns nil (no error).
func (dm *DummyMigration) Up() error { return nil }

// Down is a no-op implementation of the Migration.Down() method.
// It always returns nil (no error).
func (dm *DummyMigration) Down() error { return nil }

// migrationTemplateData holds the data needed to generate a new migration file from a template.
type migrationTemplateData struct {
	Version     uint64 // The unique version identifier for the migration
	PackageName string // The package name for the migration file
}

// MigrationsDirPath represents a directory path where migration files are stored.
// It should be used, preferably, as a global, static value, to determine where
// the migration files are placed in the file system.
type MigrationsDirPath string

// ErrCreateMigrationsDirPath is returned when the migrations directory path can't be created
// or validated (for example, when the directory doesn't exist or the path is not a directory).
var ErrCreateMigrationsDirPath = errors.New("could not create new migrations directory path")

// ErrBlankMigration is returned when a blank migration file cannot be generated,
// such as when template parsing fails or file creation fails.
var ErrBlankMigration = errors.New("could not generate blank migration")

// NewMigrationsDirPath creates a new MigrationsDirPath from the given directory path.
// It validates that the path exists and is a directory.
//
// Parameters:
//   - dirPath: The filesystem path to the migrations directory
//
// Returns:
//   - MigrationsDirPath: A validated migrations directory path
//   - error: An error if the path doesn't exist or is not a directory
func NewMigrationsDirPath(dirPath string) (MigrationsDirPath, error) {
	fileInfo, err := os.Stat(dirPath)

	if err != nil {
		return "", fmt.Errorf(
			"%w, file info init error: %w", ErrCreateMigrationsDirPath, err,
		)
	}

	if !fileInfo.IsDir() {
		return "", fmt.Errorf(
			"%w, the provided path is not a directory", ErrCreateMigrationsDirPath,
		)
	}

	return MigrationsDirPath(dirPath), nil
}

// newMigrationTemplateData creates template data for a new migration file.
// It generates a version number based on the current Unix timestamp and
// extracts the package name from the directory path.
//
// Parameters:
//   - dirPath: The migrations directory path
//
// Returns:
//   - migrationTemplateData: Data to be used in the migration file template
func newMigrationTemplateData(dirPath MigrationsDirPath) migrationTemplateData {
	return migrationTemplateData{uint64(time.Now().Unix()), filepath.Base(string(dirPath))}
}

// GenerateBlankMigration creates a new blank migration file in the specified directory.
// The file will be named using the pattern "version_[timestamp].go" and will contain
// a template implementation of the Migration interface.
//
// Parameters:
//   - dirPath: The directory where the migration file should be created
//
// Returns:
//   - fileName: The name of the generated migration file
//   - err: An error if template processing or file creation fails
func GenerateBlankMigration(dirPath MigrationsDirPath) (fileName string, err error) {
	tmpl, err := template.New("migration").Parse(TmplContents)

	if err != nil {
		return "", fmt.Errorf(
			"%w, template parsing failed with error: %w", ErrBlankMigration, err,
		)
	}

	tmplData := newMigrationTemplateData(dirPath)
	fileName = FileNamePrefix + FileNameSeparator + strconv.Itoa(int(tmplData.Version)) + ".go"
	filePath := filepath.Join(string(dirPath), fileName)

	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)

	if err != nil {
		return "", fmt.Errorf(
			"%w, file creation failed with error: %w", ErrBlankMigration, err,
		)
	}

	defer func(file *os.File) {
		closeErr := file.Close()

		if err != nil {
			if removeErr := os.Remove(filePath); removeErr != nil || closeErr != nil {
				err = errors.Join(err, removeErr, closeErr)
			}
		}
	}(file)

	if err = tmpl.Execute(file, tmplData); err != nil {
		err = fmt.Errorf(
			"%w, failed to generate contents with error: %w", ErrBlankMigration, err,
		)

		return "", err
	}

	return fileName, err
}
