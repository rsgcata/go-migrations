package migration

import (
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type MigrationTestSuite struct {
	suite.Suite
	migrationsDirPath string
}

func TestMigrationTestSuite(t *testing.T) {
	suite.Run(t, new(MigrationTestSuite))
}

func (suite *MigrationTestSuite) cleanupIntegrations() {
	if err := os.RemoveAll(suite.migrationsDirPath); err != nil {
		panic("could not cleanup test migrations dir")
	}
}

func (suite *MigrationTestSuite) SetupTest() {
	suite.migrationsDirPath = path.Join(os.TempDir(), "migrationsTestDir")
	suite.cleanupIntegrations()

	if err := os.MkdirAll(suite.migrationsDirPath, os.ModeDir); err != nil {
		panic("could not create test migrations dir")
	}
}

func (suite *MigrationTestSuite) TearDownTest() {
	suite.cleanupIntegrations()
}

func (suite *MigrationTestSuite) TestItCanCreateNewMigrationsDirPath() {
	migDir, _ := NewMigrationsDirPath(suite.migrationsDirPath)
	suite.Assert().Equal(suite.migrationsDirPath, string(migDir))
}

func (suite *MigrationTestSuite) TestItFailsToCreateNewMigrationsDirPathFromInvalidDirPath() {
	_, err := NewMigrationsDirPath("+=;.")
	suite.Assert().ErrorContains(err, "file info init")
}

func (suite *MigrationTestSuite) TestItFailsToCreateNewMigrationsDirPathFromFilePath() {
	dirPath := filepath.Join(suite.migrationsDirPath, "testEmpty")
	f, _ := os.OpenFile(dirPath, os.O_RDONLY|os.O_CREATE, 0666)
	defer func(f *os.File) {
		_ = f.Close()
	}(f)

	_, err := NewMigrationsDirPath(dirPath)
	suite.Assert().ErrorContains(err, "not a directory")
}

func (suite *MigrationTestSuite) TestItCanGenerateBlankMigrationFile() {
	migDir, _ := NewMigrationsDirPath(suite.migrationsDirPath)
	timeBefore := time.Now().Unix()
	fileName, err := GenerateBlankMigration(migDir)
	timeAfter := time.Now().Unix()
	fileContents, _ := os.ReadFile(filepath.Join(suite.migrationsDirPath, fileName))
	versionString := strings.TrimRight(
		strings.TrimLeft(fileName, FileNamePrefix+FileNameSeparator),
		".go",
	)
	versionInt, _ := strconv.Atoi(versionString)

	suite.Assert().Nil(err)
	suite.Assert().True(
		int64(versionInt) >= timeBefore && int64(versionInt) <= timeAfter,
	)
	suite.Assert().Regexp(
		"package "+filepath.Base(suite.migrationsDirPath)+".*",
		string(fileContents),
	)
	suite.Assert().Regexp(
		"type Migration"+versionString+" struct.*",
		string(fileContents),
	)
	suite.Assert().Regexp(
		"func\\(migration \\*Migration"+versionString+"\\) Version\\(\\) uint64 \\{[\\s]+return "+versionString,
		string(fileContents),
	)
}

func (suite *MigrationTestSuite) TestItFailsToGenerateBlankMigrationFromInvalidTemplate() {
	TmplContentsCopy := TmplContents
	TmplContents = "{{if pipeline}} T1 T0 {{end}} {{else}}"
	migDir, _ := NewMigrationsDirPath(suite.migrationsDirPath)
	_, err := GenerateBlankMigration(migDir)
	TmplContents = TmplContentsCopy

	suite.Assert().NotNil(err)
	suite.Assert().ErrorContains(err, "template parsing failed")
}

func (suite *MigrationTestSuite) TestItFailsToGenerateTemplateWithInvalidTemplateData() {
	oldTemplateContents := TmplContents
	TmplContents = "package {{.Missing}}"
	defer func() {
		TmplContents = oldTemplateContents
	}()
	migDir, _ := NewMigrationsDirPath(suite.migrationsDirPath)
	_, err := GenerateBlankMigration(migDir)

	suite.Assert().ErrorContains(err, "failed to generate contents")

	filesCount := 0
	items, _ := os.ReadDir(suite.migrationsDirPath)
	for _, item := range items {
		if !item.Type().IsDir() {
			filesCount++
		}
	}
	suite.Assert().Equal(0, filesCount, "generated migration file was not removed")
}

func (suite *MigrationTestSuite) TestItFailsToGenerateBlankMigrationWhenNewFileCreationFails() {
	migPath, _ := NewMigrationsDirPath(suite.migrationsDirPath)
	suite.cleanupIntegrations()
	_, err := GenerateBlankMigration(migPath)

	suite.Require().NotNil(err)
	expectedErr := &os.PathError{}
	suite.Assert().ErrorAs(err, &expectedErr)
}
