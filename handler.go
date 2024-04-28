package main

import (
	"errors"
	"fmt"
	"slices"

	"github.com/rsgcata/go-migrations/execution"
	"github.com/rsgcata/go-migrations/migration"
)

var ErrBuildHandler = errors.New("could not create new migrations handler")

type MigrationsHandler struct {
	availableMigrationsRegistry migration.MigrationsRegistry
	executions                  map[uint64]*execution.MigrationExecution
	executionsRepository        execution.Repository
}

func NewExecutionsHandler(
	availableMigrationsRegistry migration.MigrationsRegistry,
	executionsRepository execution.Repository,
) (*MigrationsHandler, error) {
	err := executionsRepository.Init()

	if err != nil {
		return nil, fmt.Errorf(
			"%w, failed to initialize the repository with error: %w", ErrBuildHandler, err,
		)
	}

	executedMigrations, err := executionsRepository.LoadExecutions()

	if err != nil {
		return nil, fmt.Errorf(
			"%w, failed to load executions with error: %w", ErrBuildHandler, err,
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
		return nil, fmt.Errorf(
			"%w, finalized executions do not match with the available migrations."+
				" There are inconsistencies between available migrations and executions."+
				" Probably an old migration has been registered after a newer one has been migrated",
			ErrBuildHandler,
		)
	}

	return &handler, nil
}

func (handler *MigrationsHandler) ExecutionsCount() int {
	return len(handler.executions)
}

func (handler *MigrationsHandler) MigrationsToExecuteCount() int {
	return handler.availableMigrationsRegistry.Count() - len(handler.executions)
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
	lastExecutedVersion := orderedVersions[len(orderedVersions)-1]
	return handler.executions[lastExecutedVersion]
}

type HandledMigration struct {
	Migration migration.Migration
	Execution *execution.MigrationExecution
}

func (handler *MigrationsHandler) MigrateUp() (HandledMigration, error) {
	executionsCount := len(handler.executions)
	if handler.availableMigrationsRegistry.Count() == executionsCount {
		return HandledMigration{nil, nil}, nil
	}

	orderedAvailableVersions := handler.availableMigrationsRegistry.OrderedVersions()
	nextVersion := orderedAvailableVersions[executionsCount]

	if executionsCount != 0 {
		prevExecution := handler.getPrevExecution()

		// Try to finish the last execution which was not completed sucessfully
		if !prevExecution.Finished() {
			nextVersion = prevExecution.Version
		}
	}

	nextMigration := handler.availableMigrationsRegistry.Get(nextVersion)
	execution := execution.StartExecution(nextMigration)
	err := nextMigration.Up()

	if err != nil {
		return HandledMigration{nextMigration, execution}, fmt.Errorf(
			"failed to migrate to next version with error: %w", err,
		)
	}

	execution.FinishExecution()
	handler.executions[execution.Version] = execution

	return HandledMigration{nextMigration, execution}, nil
}

func (handler *MigrationsHandler) MigrateDown() (HandledMigration, error) {
	prevExecution := handler.getPrevExecution()
	if prevExecution == nil {
		return HandledMigration{nil, nil}, nil
	}

	prevMigration := handler.availableMigrationsRegistry.Get(prevExecution.Version)
	err := prevMigration.Down()

	if err != nil {
		return HandledMigration{prevMigration, prevExecution}, fmt.Errorf(
			"failed to migrate to previous version with error: %w", err,
		)
	}

	delete(handler.executions, prevExecution.Version)
	return HandledMigration{prevMigration, prevExecution}, nil
}

func (handler *MigrationsHandler) MigrateAllDown() ([]HandledMigration, error) {
	handledMigrations := []HandledMigration{}

	var err error
	for {
		handledMig, errDown := handler.MigrateDown()
		err = errDown
		handledMigrations = append(handledMigrations, handledMig)
		if handledMig.Execution == nil || err != nil {
			break
		}
	}

	return handledMigrations, err
}

func (handler *MigrationsHandler) MigrateAllUp() ([]HandledMigration, error) {
	handledMigrations := []HandledMigration{}

	var err error
	for err == nil {
		handledMig, errUp := handler.MigrateUp()
		err = errUp

		if handledMig.Execution != nil {
			handledMigrations = append(handledMigrations, handledMig)
		} else {
			break
		}
	}

	return handledMigrations, err
}
