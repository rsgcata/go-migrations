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

	plan, err := NewExecutionPlan(registry, repo)

	suite.Assert().Nil(err)
	suite.Assert().NotNil(plan)
}

func (suite *HandlerTestSuite) TestItFailsToCreateExecutionPlanFromInvalidState() {
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
			_ = registry.Register(mig)
		}

		plan, err := NewExecutionPlan(registry, repo)

		suite.Assert().Nil(plan, "Failed scenario: %s", scenarioName)
		suite.Assert().NotNil(err, "Failed scenario: %s", scenarioName)
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
	plan, err := NewExecutionPlan(registry, repo)

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
			_ = registry.Register(mig)
		}

		plan, _ := NewExecutionPlan(registry, repo)
		prevMig := plan.Prev()

		suite.Assert().Equal(
			scenarioData.expectedMigration, prevMig, "Failed scenario: %s", scenarioName,
		)
	}
}

func (suite *HandlerTestSuite) TestItCanAddAndPopExecutionToPlan() {
	migrationsRegistry := migration.NewGenericRegistry()
	_ = migrationsRegistry.Register(migration.NewDummyMigration(1))
	_ = migrationsRegistry.Register(migration.NewDummyMigration(2))
	_ = migrationsRegistry.Register(migration.NewDummyMigration(3))

	plan, _ := NewExecutionPlan(migrationsRegistry, &RepoMock{})

	suite.Assert().Nil(plan.PopExecution())

	plan.AddExecution(*execution.StartExecution(migrationsRegistry.Get(1)))
	plan.AddExecution(*execution.StartExecution(migrationsRegistry.Get(2)))

	suite.Assert().True(plan.Prev().Version() == 2)
	suite.Assert().True(plan.PopExecution().Version == 2)
	suite.Assert().True(plan.Prev().Version() == 1)
}

func (suite *HandlerTestSuite) TestItCanCountMigrationsAndExecutionsFromPlan() {
	registry := migration.NewGenericRegistry()
	_ = registry.Register(migration.NewDummyMigration(1))
	_ = registry.Register(migration.NewDummyMigration(2))
	_ = registry.Register(migration.NewDummyMigration(3))

	plan, _ := NewExecutionPlan(
		registry, &RepoMock{
			loadExecutions: func() ([]execution.MigrationExecution, error) {
				return []execution.MigrationExecution{
					{Version: 1, ExecutedAtMs: 2, FinishedAtMs: 3},
					{Version: 2, ExecutedAtMs: 4, FinishedAtMs: 5},
				}, nil
			},
		},
	)
	suite.Assert().Equal(plan.RegisteredMigrationsCount(), 3)
	suite.Assert().Equal(plan.ExecutionsCount(), 2)
}

func (suite *HandlerTestSuite) TestItFailsToBuildHandlerWhenRepoInitializationFails() {
	errMsg := "init failed"
	handler, err := NewHandler(
		migration.NewGenericRegistry(), &RepoMock{
			init: func() error {
				return errors.New(errMsg)
			},
		},
	)
	suite.Assert().Nil(handler)
	suite.Assert().NotNil(err)
	suite.Assert().Contains(err.Error(), errMsg)
}

func (suite *HandlerTestSuite) TestItCanMigrateUpNextAvailableMigration() {
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
				suite.Assert().Equal(
					scenario.expectedExecution.Version, execution.Version,
					"failed scenario: %s", name,
				)
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
	}
}

func (suite *HandlerTestSuite) TestItFailsToMigrateNextWithMissingExecutionPlan() {
	errMsg := "init failed"
	registry := migration.NewGenericRegistry()
	_ = registry.Register(migration.NewDummyMigration(1))
	handler, _ := NewHandler(
		registry, &RepoMock{
			loadExecutions: func() ([]execution.MigrationExecution, error) {
				return nil, errors.New(errMsg)
			},
		},
	)
	handledMigration, err := handler.MigrateNext()
	suite.Assert().NotNil(handledMigration)
	suite.Assert().NotNil(err)
	suite.Assert().Nil(handledMigration.Execution)
	suite.Assert().Nil(handledMigration.Migration)
	suite.Assert().Contains(err.Error(), errMsg)
}

//func (suite *HandlerTestSuite) TestItCanMigrateAllUp() {
//
//}
