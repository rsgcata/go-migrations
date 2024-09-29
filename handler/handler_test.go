package handler

import (
	"errors"
	"github.com/rsgcata/go-migrations/execution"
	"github.com/rsgcata/go-migrations/migration"
	"github.com/stretchr/testify/suite"
	"slices"
	"testing"
	"time"
)

type HandlerTestSuite struct {
	suite.Suite
}

func TestHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}

func (suite *HandlerTestSuite) TestItCanCreateExecutionPlan() {
	repo := &execution.InMemoryRepository{}
	repo.SaveAll(
		[]execution.MigrationExecution{
			{Version: 1, ExecutedAtMs: 2, FinishedAtMs: 3},
			{Version: 2, ExecutedAtMs: 4, FinishedAtMs: 5},
		},
	)

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
		repo := &execution.InMemoryRepository{}
		repo.SaveAll(scenarioData.persistedExecutions)

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
	repo := &execution.InMemoryRepository{LoadErr: loadErr}
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
		repo := &execution.InMemoryRepository{}
		repo.SaveAll(scenarioData.persistedExecutions)

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
		repo := &execution.InMemoryRepository{}
		repo.SaveAll(scenarioData.persistedExecutions)

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
		repo := &execution.InMemoryRepository{}
		repo.SaveAll(executions)
		plan, _ := NewPlan(migrationsRegistry, repo)

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
		repo := &execution.InMemoryRepository{}
		repo.SaveAll(executions)
		plan, _ := NewPlan(migrationsRegistry, repo)

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

	repo := &execution.InMemoryRepository{}
	repo.SaveAll(
		[]execution.MigrationExecution{
			{Version: 1, ExecutedAtMs: 2, FinishedAtMs: 3},
			{Version: 2, ExecutedAtMs: 4, FinishedAtMs: 5},
			{Version: 3, ExecutedAtMs: 4, FinishedAtMs: 0},
		},
	)
	plan, _ := NewPlan(registry, repo)
	suite.Assert().Equal(plan.RegisteredMigrationsCount(), 3)
	suite.Assert().Equal(plan.FinishedExecutionsCount(), 2)
}

func (suite *HandlerTestSuite) TestItFailsToBuildHandlerWhenRepoInitializationFails() {
	errMsg := "init failed"
	handler, err := NewHandler(
		migration.NewGenericRegistry(),
		&execution.InMemoryRepository{InitErr: errors.New(errMsg)},
		nil,
	)
	suite.Assert().Nil(handler)
	suite.Assert().NotNil(err)
	suite.Assert().Contains(err.Error(), errMsg)
}

func (suite *HandlerTestSuite) TestItCanBuildNewNumOfRuns() {
	scenarios := map[string]struct {
		arg         string
		expectedNum int
	}{
		"0":                 {"0", 1},
		"all":               {"all", 99999},
		"1":                 {"1", 1},
		"-1":                {"-1", 1},
		"9":                 {"9", 9},
		"empty":             {"", 1},
		"empty with spaces": {"", 1},
	}

	for _, scenario := range scenarios {
		actualRuns, _ := NewNumOfRuns(scenario.arg)
		suite.Assert().Equal(int(actualRuns), scenario.expectedNum)
	}
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

func (suite *HandlerTestSuite) TestItCanHandleFailureWhenMigratingUp() {
	scenarios := map[string]struct {
		errMsg                  string
		expectedUpRan           bool
		expectedToHaveMigration bool
		expectedToHaveExecution bool
	}{
		"missing execution plan":    {"init failed", false, false, false},
		"failure to save execution": {"save failed", true, true, true},
	}

	for scenarioName, scenario := range scenarios {
		registry := migration.NewGenericRegistry()
		registeredMigration := &FakeUpMigration{
			DummyMigration: *migration.NewDummyMigration(1),
		}
		_ = registry.Register(registeredMigration)

		repoMock := &execution.InMemoryRepository{
			SaveErr: errors.New(scenario.errMsg),
		}

		if !scenario.expectedToHaveExecution {
			repoMock.LoadErr = errors.New(scenario.errMsg)
		}

		handler, _ := NewHandler(registry, repoMock, nil)
		numOfRuns, _ := NewNumOfRuns("all")
		handledMigrations, err := handler.MigrateUp(numOfRuns)
		handledMigrations = append(handledMigrations, ExecutedMigration{})
		handledMigration := handledMigrations[0]
		suite.Assert().Equal(
			scenario.expectedUpRan, registeredMigration.upRan,
			"failed scenario: %s", scenarioName,
		)
		suite.Assert().NotNil(err, "failed scenario: %s", scenarioName)
		suite.Assert().Equal(
			scenario.expectedToHaveExecution, handledMigration.Execution != nil,
			"failed scenario: %s", scenarioName,
		)
		suite.Assert().Equal(
			scenario.expectedToHaveMigration, handledMigration.Migration != nil,
			"failed scenario: %s", scenarioName,
		)
		suite.Assert().Contains(
			err.Error(), scenario.errMsg,
			"failed scenario: %s", scenarioName,
		)
	}
}

func (suite *HandlerTestSuite) TestItCanMigrateUp() {
	allRuns, _ := NewNumOfRuns("all")
	someRuns, _ := NewNumOfRuns("2")
	scenarios := map[string]struct {
		availableMigrations []migration.Migration
		initialExecutions   []execution.MigrationExecution
		expectedVersions    []uint64
		numOfRuns           NumOfRuns
	}{
		"empty migrations registry": {
			availableMigrations: []migration.Migration{},
			initialExecutions:   []execution.MigrationExecution{},
			numOfRuns:           allRuns,
		},
		"multiple registry entries and no executions": {
			availableMigrations: []migration.Migration{
				&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(1)},
				&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(2)},
			},
			initialExecutions: []execution.MigrationExecution{},
			expectedVersions:  []uint64{1, 2},
			numOfRuns:         allRuns,
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
			numOfRuns:        allRuns,
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
			numOfRuns:        allRuns,
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
			numOfRuns: allRuns,
		},
		"run only some migrations": {
			availableMigrations: []migration.Migration{
				&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(1)},
				&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(2)},
				&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(3)},
			},
			expectedVersions: []uint64{1, 2},
			numOfRuns:        someRuns,
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
		repo := &execution.InMemoryRepository{}
		repo.SaveAll(scenario.initialExecutions)

		handler, _ := NewHandler(
			buildRegistry(scenario.availableMigrations), repo, nil,
		)
		timeBefore := uint64(time.Now().UnixMilli())
		handledMigrations, err := handler.MigrateUp(scenario.numOfRuns)
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

		suite.Assert().NoError(err, "failed scenario: %s", name)
		suite.Assert().Equal(
			scenario.expectedVersions, uppedVersions,
			"failed scenario: %s", name,
		)

		var savedExecutions []uint64
		for _, saved := range repo.PersistedExecutions[len(scenario.initialExecutions):] {
			savedExecutions = append(savedExecutions, saved.Version)
		}
		suite.Assert().Equal(
			scenario.expectedVersions, savedExecutions,
			"failed scenario: %s", name,
		)
	}
}

func (suite *HandlerTestSuite) TestItCanMigrateDown() {
	allRuns, _ := NewNumOfRuns("all")
	someRuns, _ := NewNumOfRuns("2")
	scenarios := map[string]struct {
		availableMigrations []migration.Migration
		initialExecutions   []execution.MigrationExecution
		expectedVersions    []uint64
		numOfRuns           NumOfRuns
	}{
		"empty migrations registry": {
			availableMigrations: []migration.Migration{},
			initialExecutions:   []execution.MigrationExecution{},
			numOfRuns:           allRuns,
		},
		"multiple registry entries and no executions": {
			availableMigrations: []migration.Migration{
				&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(1)},
				&FakeUpMigration{DummyMigration: *migration.NewDummyMigration(2)},
			},
			initialExecutions: []execution.MigrationExecution{},
			numOfRuns:         allRuns,
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
			expectedVersions: []uint64{2, 1},
			numOfRuns:        allRuns,
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
			expectedVersions: []uint64{2, 1},
			numOfRuns:        allRuns,
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
			expectedVersions: []uint64{3, 2, 1},
			numOfRuns:        allRuns,
		},
		"run only some migrations": {
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
			expectedVersions: []uint64{3, 2},
			numOfRuns:        someRuns,
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
		repo := &execution.InMemoryRepository{}
		repo.SaveAll(scenario.initialExecutions)
		handler, _ := NewHandler(
			buildRegistry(scenario.availableMigrations), repo, nil,
		)
		handledMigrations, err := handler.MigrateDown(scenario.numOfRuns)

		var downVersions []uint64
		for _, mig := range handledMigrations {
			downVersions = append(downVersions, mig.Migration.Version())
			suite.Assert().Equal(
				mig.Migration.Version(),
				mig.Execution.Version,
				"failed scenario: %s", name,
			)
			suite.Assert().True(
				mig.Migration.(*FakeUpMigration).downRan,
				"failed scenario: %s", name,
			)
		}

		suite.Assert().NoError(err, "failed scenario: %s", name)
		suite.Assert().Equal(
			scenario.expectedVersions, downVersions,
			"failed scenario: %s", name,
		)

		var removedExecutions []uint64
		for _, removed := range scenario.initialExecutions[len(repo.PersistedExecutions):] {
			removedExecutions = append(removedExecutions, removed.Version)
		}
		slices.Reverse(removedExecutions)
		suite.Assert().Equal(
			scenario.expectedVersions, removedExecutions,
			"failed scenario: %s", name,
		)
	}
}
