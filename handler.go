package main

import (
	"fmt"
	"slices"
	"sort"

	"github.com/rsgcata/go-migrations/execution"
	"github.com/rsgcata/go-migrations/migration"
)

type ExecutedMigration struct {
	Migration migration.Migration
	Execution *execution.MigrationExecution
}

type ExecutionPlan struct {
	orderedMigrations []migration.Migration
	orderedExecutions []execution.MigrationExecution
}

func NewPlan(
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

	sort.Slice(
		executions, func(i, j int) bool {
			return executions[i].Version < executions[j].Version
		},
	)

	plan := &ExecutionPlan{
		orderedMigrations: registry.OrderedMigrations(),
		orderedExecutions: executions,
	}

	if len(plan.orderedExecutions) > len(plan.orderedMigrations) {
		return nil, fmt.Errorf(
			"%s, there are more executions than registered migrations. %s",
			genericErrMsg, errHelpMsg,
		)
	}

	for i, exec := range plan.orderedExecutions {
		if !exec.Finished() && i != len(plan.orderedExecutions)-1 {
			return nil, fmt.Errorf(
				"%s, there are multiple executions which are not finished."+
					" Only the last execution should have an \"unfinished\" state. %s",
				genericErrMsg, errHelpMsg,
			)
		}

		if exec.Version != plan.orderedMigrations[i].Version() {
			return nil, fmt.Errorf(
				"%s, execution %d at index %d does not match with registered migration"+
					" %d at index %d. Migrations and executions are out of order. %s",
				genericErrMsg, exec, i, plan.orderedMigrations[i].Version(), i, errHelpMsg,
			)
		}
	}

	return plan, err
}

func (plan *ExecutionPlan) NextToExecute() migration.Migration {
	finishedExecCount := plan.FinishedExecutionsCount()

	if len(plan.orderedMigrations) > finishedExecCount {
		return plan.orderedMigrations[finishedExecCount]
	}

	return nil
}

func (plan *ExecutionPlan) LastExecuted() ExecutedMigration {
	executionsCount := len(plan.orderedExecutions)

	if executionsCount > 0 {
		return ExecutedMigration{
			plan.orderedMigrations[executionsCount-1],
			&plan.orderedExecutions[executionsCount-1],
		}
	}

	return ExecutedMigration{}
}

func (plan *ExecutionPlan) AllToBeExecuted() []migration.Migration {
	finishedExecCount := plan.FinishedExecutionsCount()

	if finishedExecCount < plan.RegisteredMigrationsCount() {
		return plan.orderedMigrations[finishedExecCount:]
	}

	return []migration.Migration{}
}

func (plan *ExecutionPlan) AllExecuted() []ExecutedMigration {
	var execMigrations []ExecutedMigration

	for i, exec := range plan.orderedExecutions {
		execMigrations = append(
			execMigrations, ExecutedMigration{
				Migration: plan.orderedMigrations[i],
				Execution: &exec,
			},
		)
	}

	return execMigrations
}

func (plan *ExecutionPlan) RegisteredMigrationsCount() int {
	return len(plan.orderedMigrations)
}

func (plan *ExecutionPlan) FinishedExecutionsCount() int {
	count := len(plan.orderedExecutions)
	if count > 0 && !plan.orderedExecutions[count-1].Finished() {
		count--
	}
	return count
}

type ExecutionPlanBuilder func(
	registry migration.MigrationsRegistry,
	repository execution.Repository,
) (*ExecutionPlan, error)

type MigrationsHandler struct {
	registry         migration.MigrationsRegistry
	repository       execution.Repository
	newExecutionPlan ExecutionPlanBuilder
}

func (handler *MigrationsHandler) New(
	registry migration.MigrationsRegistry,
	repository execution.Repository,
	newExecutionPlan ExecutionPlanBuilder,
) (*MigrationsHandler, error) {
	err := repository.Init()

	if err != nil {
		return nil, fmt.Errorf(
			"could not create new migrations handler,"+
				" failed to initialize the repository with error: %w", err,
		)
	}

	if newExecutionPlan == nil {
		newExecutionPlan = NewPlan
	}

	return &MigrationsHandler{
		registry:         registry,
		repository:       repository,
		newExecutionPlan: newExecutionPlan,
	}, nil
}

func NewHandler(
	registry migration.MigrationsRegistry,
	repository execution.Repository,
	newExecutionPlan ExecutionPlanBuilder,
) (*MigrationsHandler, error) {
	err := repository.Init()

	if err != nil {
		return nil, fmt.Errorf(
			"could not create new migrations handler,"+
				" failed to initialize the repository with error: %w", err,
		)
	}

	if newExecutionPlan == nil {
		newExecutionPlan = NewPlan
	}

	return &MigrationsHandler{
		registry:         registry,
		repository:       repository,
		newExecutionPlan: newExecutionPlan,
	}, nil
}

func (handler *MigrationsHandler) MigrateOneUp() (ExecutedMigration, error) {
	if handler.registry.Count() == 0 {
		return ExecutedMigration{nil, nil}, nil
	}

	errMsg := "failed to migrate one up"

	plan, err := handler.newExecutionPlan(handler.registry, handler.repository)
	if err != nil {
		return ExecutedMigration{nil, nil}, fmt.Errorf(
			"%s, failed to create execution plan with error: %w", errMsg, err,
		)
	}

	migrationToExec := plan.NextToExecute()

	if migrationToExec == nil {
		return ExecutedMigration{nil, nil}, nil
	}

	exec := execution.StartExecution(migrationToExec)

	err = migrationToExec.Up()
	if err == nil {
		exec.FinishExecution()
	}

	saveErr := handler.repository.Save(*exec)

	if err != nil || saveErr != nil {
		err = fmt.Errorf(
			"%s, errors: %w, %w", errMsg, err, saveErr,
		)
	}

	return ExecutedMigration{migrationToExec, exec}, err
}

func (handler *MigrationsHandler) MigrateAllUp() ([]ExecutedMigration, error) {
	if handler.registry.Count() == 0 {
		return []ExecutedMigration{}, nil
	}

	errMsg := "failed to migrate all up"

	plan, err := handler.newExecutionPlan(handler.registry, handler.repository)
	if err != nil {
		return []ExecutedMigration{}, fmt.Errorf(
			"%s, failed to create execution plan with error: %w", errMsg, err,
		)
	}

	allToBeExec := plan.AllToBeExecuted()
	var handledMigrations []ExecutedMigration
	for _, migrationToExec := range allToBeExec {
		exec := execution.StartExecution(migrationToExec)

		if err = migrationToExec.Up(); err == nil {
			exec.FinishExecution()
		}

		handledMigrations = append(handledMigrations, ExecutedMigration{migrationToExec, exec})
		saveErr := handler.repository.Save(*exec)

		if err != nil || saveErr != nil {
			err = fmt.Errorf("%s, errors: %w, %w", errMsg, err, saveErr)
			break
		}
	}

	return handledMigrations, err
}

func (handler *MigrationsHandler) MigrateOneDown() (ExecutedMigration, error) {
	errMsg := "failed to migrate one down"

	plan, err := handler.newExecutionPlan(handler.registry, handler.repository)
	if err != nil {
		return ExecutedMigration{nil, nil}, fmt.Errorf(
			"%s, failed to create execution plan with error: %w", errMsg, err,
		)
	}

	lastExec := plan.LastExecuted()
	if lastExec.Migration == nil {
		return ExecutedMigration{nil, nil}, nil
	}

	if err = lastExec.Migration.Down(); err != nil {
		return ExecutedMigration{lastExec.Migration, nil}, err
	}

	err = handler.repository.Remove(*lastExec.Execution)
	return lastExec, err
}

func (handler *MigrationsHandler) MigrateAllDown() ([]ExecutedMigration, error) {
	errMsg := "failed to migrate all down"

	plan, err := handler.newExecutionPlan(handler.registry, handler.repository)
	if err != nil {
		return []ExecutedMigration{}, fmt.Errorf(
			"%s, failed to create execution plan with error: %w", errMsg, err,
		)
	}

	execMigrations := plan.AllExecuted()
	slices.Reverse(execMigrations)
	var handledMigrations []ExecutedMigration
	for _, execMig := range execMigrations {
		if err = execMig.Migration.Down(); err != nil {
			handledMigrations = append(handledMigrations, ExecutedMigration{execMig.Migration, nil})
			break
		}

		err = handler.repository.Remove(*execMig.Execution)

		if err != nil {
			handledMigrations = append(handledMigrations, ExecutedMigration{execMig.Migration, nil})
			break
		}

		handledMigrations = append(handledMigrations, execMig)
	}

	return handledMigrations, err
}

func (handler *MigrationsHandler) ForceUp(version uint64) (ExecutedMigration, error) {
	migrationToExec := handler.registry.Get(version)
	if migrationToExec == nil {
		return ExecutedMigration{nil, nil}, nil
	}

	exec := execution.StartExecution(migrationToExec)

	err := migrationToExec.Up()
	if err == nil {
		exec.FinishExecution()
	}

	errSave := handler.repository.Save(*exec)

	if err == nil {
		err = errSave
	} else if errSave != nil {
		err = fmt.Errorf("%w, %w", err, errSave)
	}

	return ExecutedMigration{migrationToExec, exec}, err
}

func (handler *MigrationsHandler) ForceDown(version uint64) (ExecutedMigration, error) {
	errMsg := "failed to migrate down forcefully"

	migrationToExec := handler.registry.Get(version)
	if migrationToExec == nil {
		return ExecutedMigration{nil, nil}, nil
	}

	exec, err := handler.repository.FindOne(version)
	if err != nil {
		return ExecutedMigration{migrationToExec, nil}, fmt.Errorf(
			"%s, failed to load execution with error: %w", errMsg, err,
		)
	}

	if exec == nil {
		return ExecutedMigration{migrationToExec, nil}, fmt.Errorf(
			"%s, execution not found. Maybe the migration was never executed", errMsg,
		)
	}

	if errDown := migrationToExec.Down(); errDown != nil {
		return ExecutedMigration{migrationToExec, nil}, fmt.Errorf(
			"%s, down() failed with error: %w", errMsg, errDown,
		)
	}

	err = handler.repository.Remove(*exec)

	return ExecutedMigration{migrationToExec, exec}, err
}
