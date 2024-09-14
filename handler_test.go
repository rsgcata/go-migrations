package main

import (
	"errors"
	"github.com/rsgcata/go-migrations/execution"
	"github.com/rsgcata/go-migrations/migration"
	"github.com/stretchr/testify/suite"
	"testing"
	"time"
)

type RepoMock struct {
	init           func() error
	loadExecutions func() ([]execution.MigrationExecution, error)
	save           func(execution execution.MigrationExecution) error
	remove         func(execution execution.MigrationExecution) error
	findOne        func(version uint64) (*execution.MigrationExecution, error)
}

func (rm *RepoMock) Init() error {
	if rm.init == nil {
		return nil
	}
	return rm.init()
}
func (rm *RepoMock) LoadExecutions() ([]execution.MigrationExecution, error) {
	if rm.loadExecutions == nil {
		return make([]execution.MigrationExecution, 0), nil
	}
	return rm.loadExecutions()
}
func (rm *RepoMock) Save(execution execution.MigrationExecution) error {
	return rm.save(execution)
}
func (rm *RepoMock) Remove(execution execution.MigrationExecution) error {
	return rm.remove(execution)
}
func (rm *RepoMock) FindOne(version uint64) (*execution.MigrationExecution, error) {
	return rm.findOne(version)
}

type HandlerTestSuite struct {
	suite.Suite
}

func TestHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}

func (suite *HandlerTestSuite) TestItCanCreateExecutionPlan() {
	repo := &RepoMock{
		loadExecutions: func() ([]execution.MigrationExecution, error) {
			return []execution.MigrationExecution{
				{Version: 1, ExecutedAtMs: 2, FinishedAtMs: 3},
				{Version: 2, ExecutedAtMs: 4, FinishedAtMs: 5},
			}, nil
		},
	}

	registry := migration.NewGenericRegistry()
	_ = registry.Register(migration.NewDummyMigration(1))
	_ = registry.Register(migration.NewDummyMigration(2))

	plan, err := NewPlan(registry, repo)

	suite.Assert().Nil(err)
	suite.Assert().NotNil(plan)
}

func (suite *HandlerTestSuite) TestItFailsToCreateExecutionPlanFromInvalidState() {
	scenarios := map[string]struct {
		persistedExecutions  []execution.MigrationExecution
		registeredMigrations []migration.Migration
	}{
		"more executions than registered migrations": {
			[]execution.MigrationExecution{
				{Version: 1, ExecutedAtMs: 2, FinishedAtMs: 3},
				{Version: 2, ExecutedAtMs: 2, FinishedAtMs: 3},
				{Version: 3, ExecutedAtMs: 2, FinishedAtMs: 3},
			},
			[]migration.Migration{
				migration.NewDummyMigration(1),
				migration.NewDummyMigration(3),
			},
		},
		"multiple executions which are not finished": {
			[]execution.MigrationExecution{
				{Version: 1, ExecutedAtMs: 2, FinishedAtMs: 3},
				{Version: 2, ExecutedAtMs: 2, FinishedAtMs: 0},
				{Version: 3, ExecutedAtMs: 2, FinishedAtMs: 0},
			},
			[]migration.Migration{
				migration.NewDummyMigration(1),
				migration.NewDummyMigration(2),
				migration.NewDummyMigration(3),
			},
		},
		"Migrations and executions are out of order": {
			[]execution.MigrationExecution{
				{Version: 1, ExecutedAtMs: 2, FinishedAtMs: 3},
				{Version: 4, ExecutedAtMs: 2, FinishedAtMs: 3},
				{Version: 3, ExecutedAtMs: 2, FinishedAtMs: 3},
			},
			[]migration.Migration{
				migration.NewDummyMigration(1),
				migration.NewDummyMigration(2),
				migration.NewDummyMigration(3),
			},
		},
	}

	for scenarioName, scenarioData := range scenarios {
		repo := &RepoMock{
			loadExecutions: func() ([]execution.MigrationExecution, error) {
				return scenarioData.persistedExecutions, nil
			},
		}

		registry := migration.NewGenericRegistry()
		for _, mig := range scenarioData.registeredMigrations {
			_ = registry.Register(mig)
		}

		plan, err := NewPlan(registry, repo)

		suite.Assert().Nil(plan, "Failed scenario: %s", scenarioName)
		suite.Assert().NotNil(err, "Failed scenario: %s", scenarioName)
		suite.Assert().ErrorContains(
			err, scenarioName,
			"Failed scenario: %s", scenarioName,
		)
	}
}

func (suite *HandlerTestSuite) TestItFailsToCreateExecutionsPlanWhenLoadingFromRepoFails() {
	loadErr := errors.New("load err")
	repo := &RepoMock{
		loadExecutions: func() ([]execution.MigrationExecution, error) {
			return make([]execution.MigrationExecution, 0), loadErr
		},
	}
	registry := migration.NewGenericRegistry()
	_ = registry.Register(migration.NewDummyMigration(123))
	plan, err := NewPlan(registry, repo)

	suite.Assert().Nil(plan)
	suite.Assert().ErrorContains(err, loadErr.Error())
}

func (suite *HandlerTestSuite) TestItCanGetNextMigrationFromExecutionPlan() {
	scenarios := map[string]struct {
		persistedExecutions  []execution.MigrationExecution
		registeredMigrations []migration.Migration
		expectedMigration    migration.Migration
	}{
		"no migrations": {
			[]execution.MigrationExecution{}, []migration.Migration{}, nil,
		},
		"all migrations executed": {
			[]execution.MigrationExecution{
				{Version: 1, ExecutedAtMs: 2, FinishedAtMs: 3},
				{Version: 2, ExecutedAtMs: 2, FinishedAtMs: 3},
				{Version: 3, ExecutedAtMs: 2, FinishedAtMs: 3},
			},
			[]migration.Migration{
				migration.NewDummyMigration(1), migration.NewDummyMigration(2),
				migration.NewDummyMigration(3),
			},
			nil,
		},
		"unfinished migration": {
			[]execution.MigrationExecution{
				{Version: 1, ExecutedAtMs: 2, FinishedAtMs: 3},
				{Version: 2, ExecutedAtMs: 2, FinishedAtMs: 0},
			},
			[]migration.Migration{
				migration.NewDummyMigration(1), migration.NewDummyMigration(2),
				migration.NewDummyMigration(3),
			},
			migration.NewDummyMigration(2),
		},
		"new registered migrations not executed": {
			[]execution.MigrationExecution{
				{Version: 1, ExecutedAtMs: 2, FinishedAtMs: 3},
				{Version: 2, ExecutedAtMs: 2, FinishedAtMs: 3},
			},
			[]migration.Migration{
				migration.NewDummyMigration(1), migration.NewDummyMigration(2),
				migration.NewDummyMigration(3), migration.NewDummyMigration(4),
			},
			migration.NewDummyMigration(3),
		},
	}

	for scenarioName, scenarioData := range scenarios {
		repo := &RepoMock{
			loadExecutions: func() ([]execution.MigrationExecution, error) {
				return scenarioData.persistedExecutions, nil
			},
		}

		registry := migration.NewGenericRegistry()
		for _, mig := range scenarioData.registeredMigrations {
			_ = registry.Register(mig)
		}

		plan, _ := NewPlan(registry, repo)
		nextMig := plan.NextToExecute()

		suite.Assert().Equal(
			scenarioData.expectedMigration, nextMig,
			"Failed scenario: %s", scenarioName,
		)
	}
}

func (suite *HandlerTestSuite) TestItCanGetLastExecutedMigrationFromExecutionPlan() {
	scenarios := map[string]struct {
		persistedExecutions  []execution.MigrationExecution
		registeredMigrations []migration.Migration
		expectedMigration    migration.Migration
	}{
		"no executions": {
			[]execution.MigrationExecution{},
			[]migration.Migration{
				migration.NewDummyMigration(1), migration.NewDummyMigration(2),
				migration.NewDummyMigration(3), migration.NewDummyMigration(4),
			},
			nil,
		},
		"with executions": {
			[]execution.MigrationExecution{
				{Version: 1, ExecutedAtMs: 2, FinishedAtMs: 3},
				{Version: 2, ExecutedAtMs: 2, FinishedAtMs: 3},
			},
			[]migration.Migration{
				migration.NewDummyMigration(1), migration.NewDummyMigration(2),
				migration.NewDummyMigration(3), migration.NewDummyMigration(4),
			},
			migration.NewDummyMigration(2),
		},
	}

	for scenarioName, scenarioData := range scenarios {
		repo := &RepoMock{
			loadExecutions: func() ([]execution.MigrationExecution, error) {
				return scenarioData.persistedExecutions, nil
			},
		}

		registry := migration.NewGenericRegistry()
		for _, mig := range scenarioData.registeredMigrations {
			_ = registry.Register(mig)
		}

		plan, _ := NewPlan(registry, repo)
		lastExec := plan.LastExecuted()

		suite.Assert().Equal(
			scenarioData.expectedMigration, lastExec.Migration,
			"Failed scenario: %s", scenarioName,
		)
		if lastExec.Migration != nil {
			suite.Assert().Equal(
				lastExec.Migration.Version(), lastExec.Execution.Version,
				"Failed scenario: %s", scenarioName,
			)
		}
	}
}

func (suite *HandlerTestSuite) TestItCanGetAllMigrationsToBeExecuted() {
	scenarios := map[string]struct {
		migVersions            []uint64
		finishedExecVersions   []uint64
		unfinishedExecVersions []uint64
		expected               []uint64
	}{
		"0 executions": {
			migVersions: []uint64{1, 2, 3, 4},
			expected:    []uint64{1, 2, 3, 4},
		},
		"1 finished execution": {
			migVersions:          []uint64{1, 2, 3, 4},
			finishedExecVersions: []uint64{1},
			expected:             []uint64{2, 3, 4},
		},
		"1 finished 1 unfinished executions": {
			migVersions:            []uint64{1, 2, 3, 4},
			finishedExecVersions:   []uint64{1, 2},
			unfinishedExecVersions: []uint64{3},
			expected:               []uint64{3, 4},
		},
		"no migrations and no executions": {},
	}

	for scenarioName, scenarioData := range scenarios {
		migrationsRegistry := migration.NewGenericRegistry()
		for _, version := range scenarioData.migVersions {
			_ = migrationsRegistry.Register(migration.NewDummyMigration(version))
		}

		plan, _ := NewPlan(
			migrationsRegistry, &RepoMock{
				loadExecutions: func() ([]execution.MigrationExecution, error) {
					var executions []execution.MigrationExecution
					for _, version := range scenarioData.finishedExecVersions {
						exec := execution.StartExecution(migrationsRegistry.Get(version))
						exec.FinishExecution()
						executions = append(executions, *exec)
					}
					for _, version := range scenarioData.unfinishedExecVersions {
						exec := execution.StartExecution(migrationsRegistry.Get(version))
						executions = append(executions, *exec)
					}
					return executions, nil
				},
			},
		)

		var toBeExecutedVersions []uint64
		for _, mig := range plan.AllToBeExecuted() {
			toBeExecutedVersions = append(toBeExecutedVersions, mig.Version())
		}

		suite.Assert().Equal(
			scenarioData.expected,
			toBeExecutedVersions,
			"failed scenario: %s", scenarioName,
		)
	}
}

func (suite *HandlerTestSuite) TestItCanGetAllExecutedMigrations() {
	scenarios := map[string]struct {
		migVersions            []uint64
		finishedExecVersions   []uint64
		unfinishedExecVersions []uint64
		expected               []uint64
	}{
		"0 executions": {
			migVersions: []uint64{1, 2, 3, 4},
		},
		"1 finished execution": {
			migVersions:          []uint64{1, 2, 3, 4},
			finishedExecVersions: []uint64{1},
			expected:             []uint64{1},
		},
		"1 finished 1 unfinished executions": {
			migVersions:            []uint64{1, 2, 3, 4},
			finishedExecVersions:   []uint64{1, 2},
			unfinishedExecVersions: []uint64{3},
			expected:               []uint64{1, 2, 3},
		},
		"no migrations and no executions": {},
	}

	for scenarioName, scenarioData := range scenarios {
		migrationsRegistry := migration.NewGenericRegistry()
		for _, version := range scenarioData.migVersions {
			_ = migrationsRegistry.Register(migration.NewDummyMigration(version))
		}

		plan, _ := NewPlan(
			migrationsRegistry, &RepoMock{
				loadExecutions: func() ([]execution.MigrationExecution, error) {
					var executions []execution.MigrationExecution
					for _, version := range scenarioData.finishedExecVersions {
						exec := execution.StartExecution(migrationsRegistry.Get(version))
						exec.FinishExecution()
						executions = append(executions, *exec)
					}
					for _, version := range scenarioData.unfinishedExecVersions {
						exec := execution.StartExecution(migrationsRegistry.Get(version))
						executions = append(executions, *exec)
					}
					return executions, nil
				},
			},
		)

		var executedVersions []uint64
		for _, exec := range plan.AllExecuted() {
			suite.Assert().Equal(exec.Migration.Version(), exec.Execution.Version)
			executedVersions = append(executedVersions, exec.Migration.Version())
		}

		suite.Assert().Equal(
			scenarioData.expected,
			executedVersions,
			"failed scenario: %s", scenarioName,
		)
	}
}

func (suite *HandlerTestSuite) TestItCanCountMigrationsAndFinishedExecutionsFromPlan() {
	registry := migration.NewGenericRegistry()
	_ = registry.Register(migration.NewDummyMigration(1))
	_ = registry.Register(migration.NewDummyMigration(2))
	_ = registry.Register(migration.NewDummyMigration(3))

	plan, _ := NewPlan(
		registry, &RepoMock{
			loadExecutions: func() ([]execution.MigrationExecution, error) {
				return []execution.MigrationExecution{
					{Version: 1, ExecutedAtMs: 2, FinishedAtMs: 3},
					{Version: 2, ExecutedAtMs: 4, FinishedAtMs: 5},
					{Version: 3, ExecutedAtMs: 4, FinishedAtMs: 0},
				}, nil
			},
		},
	)
	suite.Assert().Equal(plan.RegisteredMigrationsCount(), 3)
	suite.Assert().Equal(plan.FinishedExecutionsCount(), 2)
}

func (suite *HandlerTestSuite) TestItFailsToBuildHandlerWhenRepoInitializationFails() {
	errMsg := "init failed"
	handler, err := NewHandler(
		migration.NewGenericRegistry(), &RepoMock{
			init: func() error {
				return errors.New(errMsg)
			},
		}, nil,
	)
	suite.Assert().Nil(handler)
	suite.Assert().NotNil(err)
	suite.Assert().Contains(err.Error(), errMsg)
}

type FakeUpMigration struct {
	upRan   bool
	downRan bool
	migration.DummyMigration
}

func (f *FakeUpMigration) Up() error {
	f.upRan = true
	return nil
}

func (f *FakeUpMigration) Down() error {
	f.downRan = true
	return nil
}

func (suite *HandlerTestSuite) TestItCanMigrateUpNextAvailableMigration() {
	scenarios := map[string]struct {
		availableMigrations []migration.Migration
		initialExecutions   []execution.MigrationExecution
		expectedVersion     uint64
	}{
		"empty migrations registry": {
			availableMigrations: []migration.Migration{},
			initialExecutions:   []execution.MigrationExecution{},
		},
		"multiple registry entries and no executions": {
			availableMigrations: []migration.Migration{
				&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(1)},
				&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(2)},
			},
			initialExecutions: []execution.MigrationExecution{},
			expectedVersion:   1,
		},
		"multiple registry entries and some executions": {
			availableMigrations: []migration.Migration{
				&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(1)},
				&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(2)},
				&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(3)},
				&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(4)},
			},
			initialExecutions: []execution.MigrationExecution{
				{Version: 1, ExecutedAtMs: 123, FinishedAtMs: 124},
				{Version: 2, ExecutedAtMs: 125, FinishedAtMs: 126},
			},
			expectedVersion: 3,
		},
		"multiple registry entries and unfinished execution": {
			availableMigrations: []migration.Migration{
				&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(1)},
				&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(2)},
				&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(3)},
			},
			initialExecutions: []execution.MigrationExecution{
				{Version: 1, ExecutedAtMs: 123, FinishedAtMs: 124},
				{Version: 2, ExecutedAtMs: 125, FinishedAtMs: 0},
			},
			expectedVersion: 2,
		},
		"all migrations executed": {
			availableMigrations: []migration.Migration{
				&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(1)},
				&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(2)},
				&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(3)},
			},
			initialExecutions: []execution.MigrationExecution{
				{Version: 1, ExecutedAtMs: 123, FinishedAtMs: 124},
				{Version: 2, ExecutedAtMs: 125, FinishedAtMs: 126},
				{Version: 3, ExecutedAtMs: 127, FinishedAtMs: 128},
			},
		},
	}

	buildRegistry := func(migrations []migration.Migration) *migration.GenericRegistry {
		registry := migration.NewGenericRegistry()
		for _, mig := range migrations {
			_ = registry.Register(mig)
		}
		return registry
	}

	for name, scenario := range scenarios {
		repoMock := &RepoMock{
			loadExecutions: func() ([]execution.MigrationExecution, error) {
				return scenario.initialExecutions, nil
			},
			save: func(execution execution.MigrationExecution) error {
				if scenario.expectedVersion == 0 {
					suite.Failf("no executions should be saved for scenario: %s", name)
				} else {
					suite.Assert().Equal(
						scenario.expectedVersion, execution.Version,
						"failed scenario: %s", name,
					)
				}
				return nil
			},
		}
		handler, _ := NewHandler(
			buildRegistry(scenario.availableMigrations), repoMock, nil,
		)
		timeBefore := uint64(time.Now().UnixMilli())
		handledMigration, err := handler.MigrateOneUp()
		timeAfter := uint64(time.Now().UnixMilli())

		suite.Assert().NoError(err)

		if scenario.expectedVersion == 0 {
			suite.Assert().Nil(handledMigration.Migration, "failed scenario: %s", name)
			suite.Assert().Nil(handledMigration.Execution, "failed scenario: %s", name)
		} else {
			suite.Assert().Equal(
				scenario.expectedVersion,
				handledMigration.Migration.Version(),
				"failed scenario: %s", name,
			)
			suite.Assert().Equal(
				scenario.expectedVersion,
				handledMigration.Execution.Version,
				"failed scenario: %s", name,
			)
			suite.Assert().True(
				handledMigration.Migration.(*FakeUpMigration).upRan, "failed scenario: %s", name,
			)
			suite.Assert().True(
				handledMigration.Execution.Finished(), "failed scenario: %s", name,
			)
			suite.Assert().True(
				timeBefore <= handledMigration.Execution.ExecutedAtMs,
				"failed scenario: %s", name,
			)
			suite.Assert().True(
				timeAfter >= handledMigration.Execution.FinishedAtMs,
				"failed scenario: %s", name,
			)
		}
	}
}

func (suite *HandlerTestSuite) TestItFailsToMigrateUpNextWithMissingExecutionPlan() {
	errMsg := "init failed"
	registry := migration.NewGenericRegistry()
	registeredMigration := &FakeUpMigration{DummyMigration: *migration.NewDummyMigration(1)}
	_ = registry.Register(registeredMigration)
	handler, _ := NewHandler(
		registry, &RepoMock{
			loadExecutions: func() ([]execution.MigrationExecution, error) {
				return nil, errors.New(errMsg)
			},
		}, nil,
	)
	handledMigration, err := handler.MigrateOneUp()
	suite.Assert().False(registeredMigration.upRan)
	suite.Assert().NotNil(handledMigration)
	suite.Assert().NotNil(err)
	suite.Assert().Nil(handledMigration.Execution)
	suite.Assert().Nil(handledMigration.Migration)
	suite.Assert().Contains(err.Error(), errMsg)
}

func (suite *HandlerTestSuite) TestItCanMigrateAllUp() {
	scenarios := map[string]struct {
		availableMigrations []migration.Migration
		initialExecutions   []execution.MigrationExecution
		expectedVersions    []uint64
	}{
		"empty migrations registry": {
			availableMigrations: []migration.Migration{},
			initialExecutions:   []execution.MigrationExecution{},
		},
		"multiple registry entries and no executions": {
			availableMigrations: []migration.Migration{
				&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(1)},
				&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(2)},
			},
			initialExecutions: []execution.MigrationExecution{},
			expectedVersions:  []uint64{1, 2},
		},
		"multiple registry entries and some executions": {
			availableMigrations: []migration.Migration{
				&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(1)},
				&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(2)},
				&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(3)},
				&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(4)},
			},
			initialExecutions: []execution.MigrationExecution{
				{Version: 1, ExecutedAtMs: 123, FinishedAtMs: 124},
				{Version: 2, ExecutedAtMs: 125, FinishedAtMs: 126},
			},
			expectedVersions: []uint64{3, 4},
		},
		"multiple registry entries and unfinished execution": {
			availableMigrations: []migration.Migration{
				&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(1)},
				&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(2)},
				&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(3)},
			},
			initialExecutions: []execution.MigrationExecution{
				{Version: 1, ExecutedAtMs: 123, FinishedAtMs: 124},
				{Version: 2, ExecutedAtMs: 125, FinishedAtMs: 0},
			},
			expectedVersions: []uint64{2, 3},
		},
		"all migrations executed": {
			availableMigrations: []migration.Migration{
				&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(1)},
				&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(2)},
				&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(3)},
			},
			initialExecutions: []execution.MigrationExecution{
				{Version: 1, ExecutedAtMs: 123, FinishedAtMs: 124},
				{Version: 2, ExecutedAtMs: 125, FinishedAtMs: 126},
				{Version: 3, ExecutedAtMs: 127, FinishedAtMs: 128},
			},
		},
	}

	buildRegistry := func(migrations []migration.Migration) *migration.GenericRegistry {
		registry := migration.NewGenericRegistry()
		for _, mig := range migrations {
			_ = registry.Register(mig)
		}
		return registry
	}

	for name, scenario := range scenarios {
		var savedExecutions []uint64
		repoMock := &RepoMock{
			loadExecutions: func() ([]execution.MigrationExecution, error) {
				return scenario.initialExecutions, nil
			},
			save: func(execution execution.MigrationExecution) error {
				savedExecutions = append(savedExecutions, execution.Version)
				return nil
			},
		}
		handler, _ := NewHandler(
			buildRegistry(scenario.availableMigrations), repoMock, nil,
		)
		timeBefore := uint64(time.Now().UnixMilli())
		handledMigrations, err := handler.MigrateAllUp()
		timeAfter := uint64(time.Now().UnixMilli())

		var uppedVersions []uint64
		for _, mig := range handledMigrations {
			uppedVersions = append(uppedVersions, mig.Migration.Version())
			suite.Assert().Equal(
				mig.Migration.Version(),
				mig.Execution.Version,
				"failed scenario: %s", name,
			)
			suite.Assert().True(
				mig.Execution.Finished(),
				"failed scenario: %s", name,
			)
			suite.Assert().True(
				mig.Migration.(*FakeUpMigration).upRan,
				"failed scenario: %s", name,
			)
			suite.Assert().True(
				timeBefore <= mig.Execution.ExecutedAtMs,
				"failed scenario: %s", name,
			)
			suite.Assert().True(
				timeAfter >= mig.Execution.FinishedAtMs,
				"failed scenario: %s", name,
			)
		}

		suite.Assert().NoError(err)
		suite.Assert().Equal(scenario.expectedVersions, uppedVersions)
	}
}

func (suite *HandlerTestSuite) TestItCanMigrateOneDown() {
	registry := migration.NewGenericRegistry()
	_ = registry.Register(&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(1)})
	_ = registry.Register(&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(2)})
	_ = registry.Register(&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(3)})

	initialExecutions := []execution.MigrationExecution{
		{Version: 1, ExecutedAtMs: 123, FinishedAtMs: 124},
		{Version: 2, ExecutedAtMs: 125, FinishedAtMs: 0},
	}

	expectedExec := initialExecutions[1]

	repoMock := &RepoMock{
		loadExecutions: func() ([]execution.MigrationExecution, error) {
			return initialExecutions, nil
		},
		remove: func(execution execution.MigrationExecution) error {
			suite.Assert().Equal(expectedExec.Version, execution.Version)
			return nil
		},
	}

	handler, _ := NewHandler(registry, repoMock, nil)
	execMig, err := handler.MigrateOneDown()

	suite.Assert().Nil(err)
	suite.Assert().Equal(expectedExec.Version, execMig.Migration.Version())
	suite.Assert().True(execMig.Migration.(*FakeUpMigration).downRan)
	suite.Assert().Equal(execMig.Migration.Version(), execMig.Execution.Version)
}
