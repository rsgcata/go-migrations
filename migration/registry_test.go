package migration

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/stretchr/testify/suite"
)

type RegistryTestSuite struct {
	suite.Suite
	migrationsDirPath string
}

func TestRegistryTestSuite(t *testing.T) {
	suite.Run(t, new(RegistryTestSuite))
}

func (suite *RegistryTestSuite) SetupTest() {
	suite.migrationsDirPath = os.TempDir() + string(os.PathSeparator) + "migrationsTestDir"

	if err := os.RemoveAll(suite.migrationsDirPath); err != nil {
		panic("could not cleanup test migrations dir")
	}

	if err := os.MkdirAll(suite.migrationsDirPath, os.ModeDir); err != nil {
		panic("could not create test migrations dir")
	}
}

func (suite *RegistryTestSuite) TearDownTest() {
	os.RemoveAll(suite.migrationsDirPath)
}

func (suite *RegistryTestSuite) TestItCanRegisterMigration() {
	version := uint64(1234)
	dm := &DummyMigration{version}
	registry := NewGenericRegistry()
	registry.Register(dm)
	suite.Assert().Equal(dm, registry.Get(version))
}

func (suite *RegistryTestSuite) TestItFailsToRegisterDuplicateMigration() {
	version := uint64(1234)
	dm1 := &DummyMigration{version}
	dm2 := &DummyMigration{version}
	registry := NewGenericRegistry()
	registry.Register(dm1)
	err := registry.Register(dm2)
	suite.Assert().ErrorContains(err, "already registered")
}

func (suite *RegistryTestSuite) TestItCanProvideOrderedRegisteredVersions() {
	versions := []uint64{123, 124, 125}
	registry := NewGenericRegistry()
	registry.Register(&DummyMigration{versions[1]})
	registry.Register(&DummyMigration{versions[0]})
	registry.Register(&DummyMigration{versions[2]})
	suite.Assert().Equal(versions, registry.OrderedVersions())
}

func (suite *RegistryTestSuite) TestItCanProvideOrderedRegisteredMigrations() {
	expectedMigrations := []Migration{
		&DummyMigration{123}, &DummyMigration{124}, &DummyMigration{125},
	}
	registry := NewGenericRegistry()
	registry.Register(expectedMigrations[1])
	registry.Register(expectedMigrations[0])
	registry.Register(expectedMigrations[2])
	suite.Assert().Equal(expectedMigrations, registry.OrderedMigrations())
}

func (suite *RegistryTestSuite) TestItCanGetSpecificRegisteredVersion() {
	registry := NewGenericRegistry()
	for i := 0; i < 999; i++ {
		registry.Register(&DummyMigration{uint64(i)})
	}
	for i := 0; i < 999; i++ {
		mig := registry.Get(uint64(i))
		suite.Assert().Equal(uint64(i), mig.Version())
	}
}

func (suite *RegistryTestSuite) TestItCanCountRegisteredMigrations() {
	registry := NewGenericRegistry()
	expectedCount := 321
	for i := 0; i < 321; i++ {
		registry.Register(&DummyMigration{uint64(i)})
	}
	suite.Assert().Equal(expectedCount, registry.Count())
}

func (suite *RegistryTestSuite) TestItCanValidateAllDirMigrationsAreRegistered() {
	migDir, _ := NewMigrationsDirPath(suite.migrationsDirPath)
	dirRegistry := NewEmptyDirMigrationsRegistry(migDir)

	for i := 1; i < 11; i++ {
		newVersion := uint64(i)
		dirRegistry.Register(&DummyMigration{newVersion})

		migFn := FileNamePrefix + FileNameSeparator + strconv.Itoa(int(newVersion)) + ".go"
		newFilePath := filepath.Join(suite.migrationsDirPath, migFn)
		fp, _ := os.OpenFile(newFilePath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
		fp.Close()
	}

	allRegistered, missing, extra, err := dirRegistry.HasAllMigrationsRegistered()
	suite.Assert().True(allRegistered)
	suite.Assert().Nil(missing)
	suite.Assert().Nil(extra)
	suite.Assert().Nil(err)
}

func (suite *RegistryTestSuite) TestItCanComputeExtraAndMissingRegisteredMigrations() {
	migDir, _ := NewMigrationsDirPath(suite.migrationsDirPath)
	dirRegistry := NewEmptyDirMigrationsRegistry(migDir)

	for i := 1; i < 5; i++ {
		newVersion := uint64(i)
		migFn := FileNamePrefix + FileNameSeparator + strconv.Itoa(int(newVersion)) + ".go"
		newFilePath := filepath.Join(suite.migrationsDirPath, migFn)
		fp, _ := os.OpenFile(newFilePath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
		fp.Close()
	}

	dirRegistry.Register(&DummyMigration{1})
	dirRegistry.Register(&DummyMigration{2})
	dirRegistry.Register(&DummyMigration{7})
	dirRegistry.Register(&DummyMigration{8})

	expectedMissing := []string{
		FileNamePrefix + FileNameSeparator + "3.go",
		FileNamePrefix + FileNameSeparator + "4.go",
	}
	expectedExtra := []string{
		FileNamePrefix + FileNameSeparator + "7.go",
		FileNamePrefix + FileNameSeparator + "8.go",
	}

	allRegistered, missing, extra, _ := dirRegistry.HasAllMigrationsRegistered()
	slices.Sort(missing)
	slices.Sort(extra)

	suite.Assert().False(allRegistered)
	suite.Assert().Equal(expectedMissing, missing)
	suite.Assert().Equal(expectedExtra, extra)
}
