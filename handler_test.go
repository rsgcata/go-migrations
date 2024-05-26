package main

import (
	"errors"
	"testing"
	"time"

	"github.com/rsgcata/go-migrations/execution"
	"github.com/rsgcata/go-migrations/migration"
	"github.com/stretchr/testify/suite"
)

type RepoMock struct {
	locked         bool
	init           func() error
	lock           func() error
	unlock         func() error
	loadExecutions func() ([]execution.MigrationExecution, error)
	save           func(execution execution.MigrationExecution) error
	remove         func(execution execution.MigrationExecution) error
}

func (rm *RepoMock) Init() error { return rm.init() }
func (rm *RepoMock) Lock() error {
	rm.locked = true
	return rm.lock()
}
func (rm *RepoMock) Unlock() error {
	rm.locked = false
	return rm.unlock()
}
func (rm *RepoMock) LoadExecutions() ([]execution.MigrationExecution, error) {
	return rm.loadExecutions()
}
func (rm *RepoMock) Save(execution execution.MigrationExecution) error {
	return rm.save(execution)
}
func (rm *RepoMock) Remove(execution execution.MigrationExecution) error {
	return rm.remove(execution)
}

type HandlerTestSuite struct {
	suite.Suite
}

func TestHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}

var exclusiveStateChangesPanicMsg string = `concurent migration execution handling 
without exclusive lock, rights must not persist any exectuion state`

func (suite *HandlerTestSuite) TestItDoesNotAllowConcurrentMigrateRuns() {
	expectedErr := errors.New("locked error")
	repo := &RepoMock{
		lock:   func() error { return expectedErr },
		save:   func(execution.MigrationExecution) error { panic(exclusiveStateChangesPanicMsg) },
		remove: func(execution.MigrationExecution) error { panic(exclusiveStateChangesPanicMsg) },
	}
	registry := migration.NewGenericRegistry()
	registry.Register(migration.NewDummyMigration(1))
	handler, _ := NewHandler(registry, repo)

	assertRunLocked := func(handledMigration HandledMigration, actualErr error) {
		suite.Assert().ErrorIs(actualErr, expectedErr)
		suite.Assert().Nil(handledMigration.Migration)
		suite.Assert().Nil(handledMigration.Execution)
	}
	assertAllRunsLocked := func(handledMigrations []HandledMigration, actualErr error) {
		suite.Assert().ErrorIs(actualErr, expectedErr)
		suite.Assert().Len(handledMigrations, 0)
	}

	assertRunLocked(handler.MigrateNext())
	assertRunLocked(handler.MigratePrev())
	assertAllRunsLocked(handler.MigrateUp())
	assertAllRunsLocked(handler.MigrateDown())
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
	registry.Register(migration.NewDummyMigration(1))
	registry.Register(migration.NewDummyMigration(2))

	plan, err := NewExecutionPlan(registry, repo)

	suite.Assert().Nil(err)
	suite.Assert().NotNil(plan)
}

func (suite *HandlerTestSuite) TestItFailsToCreateExecutionPlanFromInvalidExecutionPath() {
	scenarios := map[string]struct {
		persistedExecutions  []execution.MigrationExecution
		registeredMigrations []migration.Migration
	}{
		"more executions than migrations": {
			[]execution.MigrationExecution{
				{Version: 1, ExecutedAtMs: 2, FinishedAtMs: 3},
				{Version: 2, ExecutedAtMs: 2, FinishedAtMs: 3},
				{Version: 3, ExecutedAtMs: 2, FinishedAtMs: 3},
			},
			[]migration.Migration{
				migration.NewDummyMigration(1), migration.NewDummyMigration(3),
			},
		},
		"missing execution for past migration": {
			[]execution.MigrationExecution{
				{Version: 1, ExecutedAtMs: 2, FinishedAtMs: 3},
				{Version: 3, ExecutedAtMs: 2, FinishedAtMs: 3},
			},
			[]migration.Migration{
				migration.NewDummyMigration(1), migration.NewDummyMigration(2),
				migration.NewDummyMigration(3),
			},
		},
		"multiple unfinished executions": {
			[]execution.MigrationExecution{
				{Version: 1, ExecutedAtMs: 2, FinishedAtMs: 3},
				{Version: 2, ExecutedAtMs: 2, FinishedAtMs: 0},
				{Version: 3, ExecutedAtMs: 2, FinishedAtMs: 0},
			},
			[]migration.Migration{
				migration.NewDummyMigration(1), migration.NewDummyMigration(2),
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
			registry.Register(mig)
		}

		plan, err := NewExecutionPlan(registry, repo)

		suite.Assert().Nil(plan, "Failed scenario: %s", scenarioName)
		suite.Assert().NotNil(err, "Failed scenario: %s", scenarioName)
	}
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
			registry.Register(mig)
		}

		plan, _ := NewExecutionPlan(registry, repo)
		nextMig := plan.Next()

		suite.Assert().Equal(
			scenarioData.expectedMigration, nextMig, "Failed scenario: %s", scenarioName,
		)
	}
}

func (suite *HandlerTestSuite) TestItCanGetPreviousMigrationFromExecutionPlan() {
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
			registry.Register(mig)
		}

		plan, _ := NewExecutionPlan(registry, repo)
		prevMig := plan.Prev()

		suite.Assert().Equal(
			scenarioData.expectedMigration, prevMig, "Failed scenario: %s", scenarioName,
		)
	}
}

func (suite *HandlerTestSuite) TestItCanMigrateUpTheProperNextAvailableMigration() {
	scenarios := map[string]struct {
		availableMigrations []migration.Migration
		initialExecutions   []execution.MigrationExecution
		expectedExecution   *execution.MigrationExecution
		expectedMigration   migration.Migration
	}{
		"empty migrations registry": {
			availableMigrations: []migration.Migration{},
			initialExecutions:   []execution.MigrationExecution{},
			expectedExecution:   nil,
			expectedMigration:   nil,
		},
		"multiple registry entries and no executions": {
			availableMigrations: []migration.Migration{
				migration.NewDummyMigration(1),
				migration.NewDummyMigration(2),
			},
			initialExecutions: []execution.MigrationExecution{},
			expectedExecution: &execution.MigrationExecution{
				Version: 1,
			},
			expectedMigration: migration.NewDummyMigration(1),
		},
		"multiple registry entries and some executions": {
			availableMigrations: []migration.Migration{
				migration.NewDummyMigration(1),
				migration.NewDummyMigration(2),
				migration.NewDummyMigration(3),
				migration.NewDummyMigration(4),
			},
			initialExecutions: []execution.MigrationExecution{
				{Version: 1, ExecutedAtMs: 123, FinishedAtMs: 124},
				{Version: 2, ExecutedAtMs: 125, FinishedAtMs: 126},
			},
			expectedExecution: &execution.MigrationExecution{
				Version: 3,
			},
			expectedMigration: migration.NewDummyMigration(3),
		},
		"multiple registry entries and unfinished execution": {
			availableMigrations: []migration.Migration{
				migration.NewDummyMigration(1),
				migration.NewDummyMigration(2),
				migration.NewDummyMigration(3),
			},
			initialExecutions: []execution.MigrationExecution{
				{Version: 1, ExecutedAtMs: 123, FinishedAtMs: 124},
				{Version: 2, ExecutedAtMs: 125, FinishedAtMs: 0},
			},
			expectedExecution: &execution.MigrationExecution{
				Version: 2,
			},
			expectedMigration: migration.NewDummyMigration(2),
		},
		"all migrations executed": {
			availableMigrations: []migration.Migration{
				migration.NewDummyMigration(1),
				migration.NewDummyMigration(2),
				migration.NewDummyMigration(3),
			},
			initialExecutions: []execution.MigrationExecution{
				{Version: 1, ExecutedAtMs: 123, FinishedAtMs: 124},
				{Version: 2, ExecutedAtMs: 125, FinishedAtMs: 126},
				{Version: 3, ExecutedAtMs: 127, FinishedAtMs: 128},
			},
			expectedExecution: nil,
			expectedMigration: nil,
		},
	}

	buildRegistry := func(migrations []migration.Migration) *migration.GenericRegistry {
		registry := migration.NewGenericRegistry()
		for _, mig := range migrations {
			registry.Register(mig)
		}
		return registry
	}

	for name, scenario := range scenarios {
		var repoMock *RepoMock
		repoMock = &RepoMock{
			lock:   func() error { return nil },
			unlock: func() error { return nil },
			loadExecutions: func() ([]execution.MigrationExecution, error) {
				return scenario.initialExecutions, nil
			},
			save: func(execution execution.MigrationExecution) error {
				suite.Assert().Equal(
					scenario.expectedExecution.Version, execution.Version,
					"failed scenario: %s", name,
				)
				suite.Assert().True(repoMock.locked)
				return nil
			},
		}
		handler, _ := NewHandler(buildRegistry(scenario.availableMigrations), repoMock)
		timeBefore := uint64(time.Now().UnixMilli())
		handledMigration, err := handler.MigrateNext()
		timeAfter := uint64(time.Now().UnixMilli())

		suite.Assert().NoError(err)
		suite.Assert().Equal(
			scenario.expectedMigration, handledMigration.Migration, "failed scenario: %s", name,
		)

		if scenario.expectedExecution == nil {
			suite.Assert().Nil(
				handledMigration.Execution, "failed scenario: %s", name,
			)
		} else {
			suite.Assert().Equal(
				scenario.expectedExecution.Version, handledMigration.Execution.Version,
				"failed scenario: %s", name,
			)
			suite.Assert().Equal(
				scenario.expectedMigration.Version(), handledMigration.Execution.Version,
				"failed scenario: %s", name,
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

		suite.Assert().False(repoMock.locked)
	}
}
