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

// TmplContents Migration file template to be used to generate a new, base migration file
// to be easier for devs to create new migrations.
//
//go:embed migration.go.template
var TmplContents string

// FileNamePrefix Migration file name prefix, static value,
// which will be set for all migration files.
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

// DummyMigration struct that should be used only in tests
type DummyMigration struct {
	version uint64
}

func NewDummyMigration(version uint64) *DummyMigration {
	return &DummyMigration{version: version}
}

func (dm *DummyMigration) Version() uint64 {
	return dm.version
}

func (dm *DummyMigration) Up() error   { return nil }
func (dm *DummyMigration) Down() error { return nil }

type migrationTemplateData struct {
	Version     uint64
	PackageName string
}

// MigrationsDirPath should be used, preferably, as a global, static value, to determine where
// the migration files are placed in the file system.
type MigrationsDirPath string

// ErrCreateMigrationsDirPath is a generic error for the scenarios when the migrations
// directory path can't be created (for example, nonexistent directory in the file system).
var ErrCreateMigrationsDirPath = errors.New("could not create new migrations directory path")

// ErrBlankMigration is a generic error for failing to create a blank migration
var ErrBlankMigration = errors.New("could not generate blank migration")

// NewMigrationsDirPath can be used to create a new MigrationsDirPath
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

func newMigrationTemplateData(dirPath MigrationsDirPath) migrationTemplateData {
	return migrationTemplateData{uint64(time.Now().Unix()), filepath.Base(string(dirPath))}
}

// GenerateBlankMigration generates a blank migration file in the specified directory
// Returns the generated file name
// Errors if template processing failed or file creation failed
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
