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

//go:embed migration.go.template
var TmplContents string

const FileNamePrefix = "version"
const FileNameSeparator = "_"

type Migration interface {
	Version() uint64
	Up() error
	Down() error
}

type migrationTemplateData struct {
	Version     uint64
	PackageName string
}

type MigrationsDirPath string

var ErrCreateMigrationsDirPath = errors.New("could not create new migrations directory path")
var ErrBlankMigration = errors.New("could not generate blank migration")

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

// Generates a blank migration file in the specified directory
// Returns the generated file name
// Errors if template processing failed or file creation failed
func GenerateBlankMigration(dirPath MigrationsDirPath) (string, error) {
	tmpl, err := template.New("migration").Parse(TmplContents)

	if err != nil {
		return "", fmt.Errorf(
			"%w, template parsing failed with error: %w", ErrBlankMigration, err,
		)
	}

	tmplData := newMigrationTemplateData(dirPath)
	fileName := FileNamePrefix + FileNameSeparator + strconv.Itoa(int(tmplData.Version)) + ".go"
	filePath := filepath.Join(string(dirPath), fileName)

	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)

	if err != nil {
		return "", fmt.Errorf(
			"%w, file creation failed with error: %w", ErrBlankMigration, err,
		)
	}
	defer file.Close()

	if err = tmpl.Execute(file, tmplData); err != nil {
		file.Close()
		os.Remove(filePath)
		return "", fmt.Errorf(
			"%w, failed to generate contents with error: %w", ErrBlankMigration, err,
		)
	}

	return fileName, nil
}
