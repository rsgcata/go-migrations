package main

import (
	"errors"
	"fmt"
	"slices"

	"github.com/rsgcata/go-migrations/execution"
	"github.com/rsgcata/go-migrations/migration"
)

type MigrationsHandler struct {
	availableMigrationsRegistry migration.MigrationsRegistry
	executions                  map[uint64]*execution.MigrationExecution
	executionsRepository        execution.Repository
}

func NewExecutionsHandler(
	availableMigrationsRegistry migration.MigrationsRegistry,
	executionsRepository execution.Repository,
) (*MigrationsHandler, error) {
	executedMigrations, err := executionsRepository.LoadExecutions()
	const newErrMsg = "could not create new executions registry"

	if err != nil {
		return nil, fmt.Errorf(
			newErrMsg+". Failed to load executions with error: %w",
			err,
		)
	}

	handler := MigrationsHandler{
		availableMigrationsRegistry,
		make(map[uint64]*execution.MigrationExecution),
		executionsRepository,
	}

	for _, execution := range executedMigrations {
		handler.executions[execution.Version] = &execution
	}

	executedVersions := handler.getOrderedExecutedVersions()
	availableVersionsSlice := availableMigrationsRegistry.
		OrderedVersions()[0:len(executedMigrations)]

	if !slices.Equal(availableVersionsSlice, executedVersions) {
		return nil, errors.New(
			newErrMsg + ". Finalized executions do not match with the available migrations." +
				" Check that the order and versions for executed migrations" +
				" match with the available migrations",
		)
	}

	err = handler.executionsRepository.Init()

	if err != nil {
		return nil, fmt.Errorf(
			newErrMsg+". Failed to initialize the repository with error: %w", err,
		)
	}

	return &handler, nil
}

func (handler *MigrationsHandler) getOrderedExecutedVersions() []uint64 {
	var executedVersions []uint64
	for _, version := range handler.executions {
		executedVersions = append(executedVersions, version.Version)
	}
	slices.Sort(executedVersions)
	return executedVersions
}

func (handler *MigrationsHandler) getPrevExecution() *execution.MigrationExecution {
	if len(handler.executions) == 0 {
		return nil
	}

	orderedVersions := handler.getOrderedExecutedVersions()
	return handler.executions[orderedVersions[len(orderedVersions)-1]]
}

func (handler *MigrationsHandler) MigrateUp(migration migration.Migration) (
	migration.Migration, *execution.MigrationExecution, error,
) {
	executionsCount := len(handler.executions)
	if handler.availableMigrationsRegistry.Count() == executionsCount {
		return nil, nil, nil
	}

	orderedAvailableVersions := handler.availableMigrationsRegistry.OrderedVersions()
	nextVersion := orderedAvailableVersions[executionsCount+1]
	prevExecution := handler.getPrevExecution()

	if !prevExecution.Finished() {
		nextVersion = prevExecution.Version
	}

	nextMigration := handler.availableMigrationsRegistry.Get(nextVersion)
	execution := execution.StartExecution(nextMigration)
	err := nextMigration.Up()

	if err != nil {
		return nextMigration, execution, fmt.Errorf(
			"failed to migrate to next version with error: %w", err,
		)
	}

	execution.FinishExecution()
	handler.executions[execution.Version] = execution

	return nextMigration, execution, nil
}

func (handler *MigrationsHandler) MigrateDown(migration migration.Migration) (
	migration.Migration, *execution.MigrationExecution, error,
) {
	prevExecution := handler.getPrevExecution()
	if prevExecution == nil {
		return nil, nil, nil
	}

	prevMigration := handler.availableMigrationsRegistry.Get(prevExecution.Version)
	err := prevMigration.Down()

	if err != nil {
		return prevMigration, prevExecution, fmt.Errorf(
			"failed to migrate to previous version with error: %w", err,
		)
	}

	delete(handler.executions, prevExecution.Version)
	return prevMigration, prevExecution, nil
}
