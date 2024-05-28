package main

import (
	"fmt"
	"sort"

	"github.com/rsgcata/go-migrations/execution"
	"github.com/rsgcata/go-migrations/migration"
)

type HandledMigration struct {
	Migration migration.Migration
	Execution *execution.MigrationExecution
}

type ExecutionPlan struct {
	orderedMigrations []migration.Migration
	orderedExecutions []execution.MigrationExecution
	registry          migration.MigrationsRegistry
}

func NewExecutionPlan(
	registry migration.MigrationsRegistry,
	repository execution.Repository,
) (*ExecutionPlan, error) {
	genericErrMsg := "failed to create new execution plan"
	errHelpMsg := "Fix executions issues before trying to manipulate their state"

	executions, err := repository.LoadExecutions()
	if err != nil {
		return nil, fmt.Errorf(
			"%s, failed to load executions with error: %w. %s", genericErrMsg, err, errHelpMsg,
		)
	}

	sort.Slice(executions, func(i, j int) bool {
		return executions[i].Version < executions[j].Version
	})

	plan := &ExecutionPlan{
		orderedMigrations: registry.OrderedMigrations(),
		orderedExecutions: executions,
		registry:          registry,
	}

	if len(plan.orderedExecutions) > len(plan.orderedMigrations) {
		return nil, fmt.Errorf(
			"%s, there are more executions than registered migrations. %s",
			genericErrMsg, errHelpMsg,
		)
	}

	for i, ev := range plan.orderedExecutions {
		if !ev.Finished() && i != len(plan.orderedExecutions)-1 {
			return nil, fmt.Errorf(
				"%s, there are multiple executions which are not finished."+
					" Only the last execution should have an \"unfinished\" state. %s"+
					genericErrMsg, errHelpMsg,
			)
		}

		if ev.Version != plan.orderedMigrations[i].Version() {
			return nil, fmt.Errorf(
				"%s, execution %d at index %d does not match with registered migration"+
					" %d at index %d. %s",
				genericErrMsg, ev, i, plan.orderedMigrations[i].Version(), i, errHelpMsg,
			)
		}
	}

	return plan, err
}

func (plan *ExecutionPlan) Next() migration.Migration {
	executionsCount := len(plan.orderedExecutions)

	if executionsCount > 0 &&
		!plan.orderedExecutions[executionsCount-1].Finished() {
		return plan.registry.Get(plan.orderedExecutions[executionsCount-1].Version)
	} else if plan.registry.Count() > executionsCount {
		nextVersion := plan.orderedMigrations[executionsCount].Version()
		return plan.registry.Get(nextVersion)
	}

	return nil
}

func (plan *ExecutionPlan) Prev() migration.Migration {
	executionsCount := len(plan.orderedExecutions)

	if executionsCount > 0 {
		return plan.registry.Get(plan.orderedExecutions[executionsCount-1].Version)
	}

	return nil
}

func (plan *ExecutionPlan) AddExecution(execution execution.MigrationExecution) {
	plan.orderedExecutions = append(plan.orderedExecutions, execution)
}

func (plan *ExecutionPlan) PopExecution() *execution.MigrationExecution {
	executionsCount := len(plan.orderedExecutions)

	if executionsCount > 0 {
		exec := plan.orderedExecutions[executionsCount-1]
		plan.orderedExecutions = plan.orderedExecutions[:executionsCount-1]
		return &exec
	}

	return nil
}

func (plan *ExecutionPlan) RegisteredMigrationsCount() int {
	return len(plan.orderedMigrations)
}

func (plan *ExecutionPlan) ExecutionsCount() int {
	return len(plan.orderedExecutions)
}

type MigrationsHandler struct {
	registry   migration.MigrationsRegistry
	repository execution.Repository
}

func NewHandler(
	registry migration.MigrationsRegistry,
	repository execution.Repository,
) (*MigrationsHandler, error) {
	err := repository.Init()
	errMsg := "could not create new migrations handler"

	if err != nil {
		return nil, fmt.Errorf(
			"%s, failed to initialize the repository with error: %w", errMsg, err,
		)
	}

	return &MigrationsHandler{registry: registry, repository: repository}, nil
}

func (handler *MigrationsHandler) MigrateNext() (HandledMigration, error) {
	if handler.registry.Count() == 0 {
		return HandledMigration{nil, nil}, nil
	}

	errMsg := "failed to migrate up"

	plan, err := NewExecutionPlan(handler.registry, handler.repository)
	if err != nil {
		return HandledMigration{nil, nil}, fmt.Errorf(
			"%s, failed to create execution plan with error: %w", errMsg, err,
		)
	}

	migrationToExec := plan.Next()

	if migrationToExec == nil {
		return HandledMigration{nil, nil}, nil
	}

	exec := execution.StartExecution(migrationToExec)

	if err = migrationToExec.Up(); err == nil {
		exec.FinishExecution()
	}

	err = handler.repository.Save(*exec)
	return HandledMigration{migrationToExec, exec}, err
}

func (handler *MigrationsHandler) MigrateUp() ([]HandledMigration, error) {
	if handler.registry.Count() == 0 {
		return []HandledMigration{}, nil
	}

	errMsg := "failed to migrate all up"

	plan, err := NewExecutionPlan(handler.registry, handler.repository)
	if err != nil {
		return []HandledMigration{}, fmt.Errorf(
			"%s, failed to create execution plan with error: %w", errMsg, err,
		)
	}

	migrationToExec := plan.Next()
	handledMigrations := []HandledMigration{}
	for migrationToExec != nil {
		exec := execution.StartExecution(migrationToExec)

		if err = migrationToExec.Up(); err == nil {
			exec.FinishExecution()
		}

		err = handler.repository.Save(*exec)
		handledMigrations = append(handledMigrations, HandledMigration{migrationToExec, exec})
		plan.AddExecution(*exec)

		if err != nil {
			break
		}

		migrationToExec = plan.Next()
	}

	return handledMigrations, err
}

func (handler *MigrationsHandler) MigratePrev() (HandledMigration, error) {
	if handler.registry.Count() == 0 {
		return HandledMigration{nil, nil}, nil
	}

	errMsg := "failed to migrate down"

	plan, err := NewExecutionPlan(handler.registry, handler.repository)
	if err != nil {
		return HandledMigration{nil, nil}, fmt.Errorf(
			"%s, failed to create execution plan with error: %w", errMsg, err,
		)
	}

	migrationToExec := plan.Prev()

	if migrationToExec == nil {
		return HandledMigration{nil, nil}, nil
	}

	if err = migrationToExec.Down(); err != nil {
		return HandledMigration{migrationToExec, nil}, err
	}

	exec := plan.PopExecution()
	handler.repository.Remove(*exec)

	return HandledMigration{migrationToExec, exec}, err
}

func (handler *MigrationsHandler) MigrateDown() ([]HandledMigration, error) {
	if handler.registry.Count() == 0 {
		return []HandledMigration{}, nil
	}

	errMsg := "failed to migrate all down"

	plan, err := NewExecutionPlan(handler.registry, handler.repository)
	if err != nil {
		return []HandledMigration{}, fmt.Errorf(
			"%s, failed to create execution plan with error: %w", errMsg, err,
		)
	}

	migrationToExec := plan.Prev()
	handledMigrations := []HandledMigration{}
	for migrationToExec != nil {
		if err = migrationToExec.Down(); err != nil {
			handledMigrations = append(handledMigrations, HandledMigration{migrationToExec, nil})
			break
		}

		exec := plan.PopExecution()
		err = handler.repository.Remove(*exec)

		if err != nil {
			handledMigrations = append(handledMigrations, HandledMigration{migrationToExec, nil})
			break
		}

		handledMigrations = append(handledMigrations, HandledMigration{migrationToExec, exec})
		migrationToExec = plan.Prev()
	}

	return handledMigrations, err
}
