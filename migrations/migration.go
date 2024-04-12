package migrations

import (
	_ "embed"
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

// const MaxMigrationNameLen = 128

// type MigrationName string

// func NewMigrationName(name string) (MigrationName, error) {
// 	if len(name) > MaxMigrationNameLen {
// 		return "", errors.New(
// 			"Could not create new migration name. The name should have max " +
// 				strconv.Itoa(MaxMigrationNameLen) + " characters",
// 		)
// 	}

// 	return MigrationName(name), nil
// }

type Migration interface {
	Version() uint64
	Up() error
	Down() error
}

type migrationTemplateData struct {
	Version     uint64
	PackageName string
}

func newMigrationTemplateData(folderPath string) migrationTemplateData {
	return migrationTemplateData{uint64(time.Now().Unix()), filepath.Base(folderPath)}
}

func GenerateBlankMigration(folderPath string) error {
	tmpl, err := template.New("migration").Parse(tmplContents)

	if err != nil {
		return fmt.Errorf(
			"could not generate blank migration. Temaplate parsing failed with"+
				" error: %w", err,
		)
	}

	tmplData := newMigrationTemplateData(folderPath)
	filePath := strings.TrimRight(folderPath, string(os.PathSeparator)) +
		string(os.PathSeparator) + FileNamePrefix + FileNameSeparator +
		strconv.Itoa(int(tmplData.Version)) + ".go"
	fmt.Println(filePath)
	file, err := os.OpenFile(
		filePath,
		os.O_WRONLY|os.O_CREATE|os.O_APPEND,
		0600,
	)

	if err != nil {
		return fmt.Errorf(
			"could not generate blank migration. File creation failed with"+
				" error: %w", err,
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

// type Migration struct {
// 	version         uint64
// 	name            MigrationName
// 	executedAt      uint64
// 	executionMillis uint32
// }

// func NewMigration(name MigrationName, handler MigrationHandler) Migration {
// 	return Migration{uint64(time.Now().Unix()), name, 0, 0, handler}
// }

// func ReconstituteMigration(
// 	version uint64,
// 	name string,
// 	executedAt uint64,
// 	executionMillis uint32,
// 	handler MigrationHandler,
// ) Migration {
// 	return Migration{version, MigrationName(name), executedAt, executionMillis, handler}
// }

// func (migration *Migration) Handler() MigrationHandler {
// 	return migration.handler
// }
