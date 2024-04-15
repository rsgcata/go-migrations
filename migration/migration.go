package migration

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"
)

//go:embed migration.go.template
var tmplContents string

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

func NewMigrationsDirPath(dirPath string) (MigrationsDirPath, error) {
	fileInfo, err := os.Stat(dirPath)

	if err != nil {
		return "", fmt.Errorf(
			"could not create new migration directory path. Could not initialize file info"+
				" with error: %w", err,
		)
	}

	if !fileInfo.IsDir() {
		return "", errors.New(
			"could not create new migration directory path. The provided path is not a directory",
		)
	}

	return MigrationsDirPath(strings.TrimRight(dirPath, string(os.PathSeparator))), nil
}

func newMigrationTemplateData(dirPath MigrationsDirPath) migrationTemplateData {
	return migrationTemplateData{uint64(time.Now().Unix()), filepath.Base(string(dirPath))}
}

func GenerateBlankMigration(dirPath MigrationsDirPath) error {
	tmpl, err := template.New("migration").Parse(tmplContents)

	if err != nil {
		return fmt.Errorf(
			"could not generate blank migration. Temaplate parsing failed with"+
				" error: %w", err,
		)
	}

	tmplData := newMigrationTemplateData(dirPath)
	filePath := string(dirPath) + string(os.PathSeparator) +
		FileNamePrefix + FileNameSeparator + strconv.Itoa(int(tmplData.Version)) + ".go"
	fmt.Println(filePath)
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)

	if err != nil {
		return fmt.Errorf(
			"could not generate blank migration. File creation failed with error: %w", err,
		)
	}
	defer file.Close()

	if err = tmpl.Execute(file, tmplData); err != nil {
		return fmt.Errorf(
			"could not generate blank migration. Failed to generate contents with"+
				" error: %w", err,
		)
	}

	return nil
}
