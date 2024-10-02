package cli

import (
	"errors"
	"fmt"
	"github.com/rsgcata/go-migrations/handler"
	"os"
	"sort"
	"strconv"
	"text/tabwriter"

	"github.com/rsgcata/go-migrations/execution"
	"github.com/rsgcata/go-migrations/migration"
)

type Command interface {
	Name() string
	Description() string
	Exec() error
}

func Bootstrap(
	args []string,
	registry migration.MigrationsRegistry,
	repository execution.Repository,
	dirPath migration.MigrationsDirPath,
	newHandler func(
		registry migration.MigrationsRegistry,
		repository execution.Repository,
		newExecutionPlan handler.ExecutionPlanBuilder,
	) (*handler.MigrationsHandler, error),
) {
	if newHandler == nil {
		newHandler = handler.NewHandler
	}

	migrationsHandler, err := newHandler(registry, repository, nil)

	if err != nil {
		panic(
			fmt.Errorf(
				"coult not bootstrap cli, %s: %w",
				"failed to create new migrations migrationsHandler with error", err,
			),
		)
	}

	inputCmd := "help"

	if len(args) >= 1 {
		if args[0] == "--" {
			args = args[1:]
		}

		inputCmd = args[0]
	}

	availableCommands := make(map[string]Command)

	allUp := &MigrateUpCommand{handler: migrationsHandler, args: args}
	allDown := &MigrateDownCommand{handler: migrationsHandler, args: args}
	forceUp := &MigrateForceUpCommand{handler: migrationsHandler, args: args}
	forceDown := &MigrateForceDownCommand{handler: migrationsHandler, args: args}
	stats := &MigrateStatsCommand{registry: registry, repository: repository}
	blank := &GenerateBlankMigrationCommand{dirPath}
	availableCommands[allUp.Name()] = allUp
	availableCommands[allDown.Name()] = allDown
	availableCommands[forceUp.Name()] = forceUp
	availableCommands[forceDown.Name()] = forceDown
	availableCommands[stats.Name()] = stats
	availableCommands[blank.Name()] = blank

	help := &HelpCommand{availableCommands: availableCommands}

	for _, cmd := range availableCommands {
		if inputCmd == cmd.Name() {
			if cmdErr := cmd.Exec(); cmdErr != nil {
				fmt.Println("Failed to execute \"" + cmd.Name() + "\" with error: " + cmdErr.Error())
			}
			return
		}
	}

	if cmdErr := help.Exec(); cmdErr != nil {
		fmt.Println("Failed to execute \"" + help.Name() + "\" with error: " + cmdErr.Error())
	}
}

type HelpCommand struct {
	availableCommands map[string]Command
}

func (c *HelpCommand) Name() string {
	return "help"
}

func (c *HelpCommand) Description() string {
	return "Go Migrations is a database schema versioning tool" +
		" which helps to easily deploy schema changes"
}

func (c *HelpCommand) Exec() error {
	fmt.Println("")
	fmt.Println(c.Description())
	fmt.Println("")
	fmt.Println("Available commands:")
	fmt.Println("")

	writer := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	_, _ = fmt.Fprintln(writer, c.Name()+"\tDisplays helpful information about this tool")

	var names []string

	for key := range c.availableCommands {
		names = append(names, key)
	}
	sort.Strings(names)

	for _, name := range names {
		_, _ = fmt.Fprintln(
			writer, c.availableCommands[name].Name()+"\t"+c.availableCommands[name].Description(),
		)
	}
	_ = writer.Flush()

	return nil
}

type MigrateUpCommand struct {
	handler *handler.MigrationsHandler
	args    []string
}

func (c *MigrateUpCommand) Name() string {
	return "up"
}

func (c *MigrateUpCommand) Description() string {
	return "Executes Up() for the specified number of registered and not yet executed migrations." +
		" If the number of migrations to execute is not specified, defaults to 1. Allowed" +
		" values for the number of migrations to run Up(): \"all\", alias for 99999 and a valid" +
		" integer greater than 0"
}

func (c *MigrateUpCommand) Exec() error {
	var numOfRuns handler.NumOfRuns
	var argErr error

	if len(c.args) < 2 {
		numOfRuns, argErr = handler.NewNumOfRuns("1")
	} else {
		numOfRuns, argErr = handler.NewNumOfRuns(c.args[1])
	}

	if argErr != nil {
		fmt.Printf("Failed to execute Up(). %s\n", argErr)
		return argErr
	}

	execs, err := c.handler.MigrateUp(numOfRuns)
	fmt.Printf("Executed Up() for %d migrations\n", len(execs))

	for _, execMig := range execs {
		if execMig.Execution != nil {
			fmt.Printf("Executed Up() for %d migration\n", execMig.Execution.Version)
		}
	}

	return err
}

type MigrateDownCommand struct {
	handler *handler.MigrationsHandler
	args    []string
}

func (c *MigrateDownCommand) Name() string {
	return "down"
}

func (c *MigrateDownCommand) Description() string {
	return "Executes Down() for the specified number of executed migrations." +
		" If the number of executions is not specified, defaults to 1. Allowed" +
		" values for the number of migrations to run Down(): \"all\", alias for 99999 and a valid" +
		" integer greater than 0"
}

func (c *MigrateDownCommand) Exec() error {
	var numOfRuns handler.NumOfRuns
	var argErr error

	if len(c.args) < 2 {
		numOfRuns, argErr = handler.NewNumOfRuns("1")
	} else {
		numOfRuns, argErr = handler.NewNumOfRuns(c.args[1])
	}

	if argErr != nil {
		fmt.Printf("Failed to execute Down(). %s\n", argErr)
		return argErr
	}

	execs, err := c.handler.MigrateDown(numOfRuns)

	fmt.Printf("Executed Down() for %d migrations\n", len(execs))

	for _, mig := range execs {
		if mig.Execution != nil {
			fmt.Printf("Executed Down() for %d migration\n", mig.Execution.Version)
		}

	}

	return err
}

type MigrateStatsCommand struct {
	registry   migration.MigrationsRegistry
	repository execution.Repository
}

func (c *MigrateStatsCommand) Name() string {
	return "stats"
}

func (c *MigrateStatsCommand) Description() string {
	return "Displays statistics about registered migrations and executions"
}

func (c *MigrateStatsCommand) Exec() error {
	plan, err := handler.NewPlan(c.registry, c.repository)

	if plan != nil {
		nextMigFile := "N/A"
		lastMigFile := "N/A"
		next := plan.NextToExecute()
		prev := plan.LastExecuted().Migration

		if next != nil {
			nextMigFile = migration.FileNamePrefix + migration.FileNameSeparator +
				strconv.Itoa(int(next.Version())) + ".go"
		}
		if prev != nil {
			lastMigFile = migration.FileNamePrefix + migration.FileNameSeparator +
				strconv.Itoa(int(prev.Version())) + ".go"
		}

		fmt.Println("")
		fmt.Printf("Registered migrations count: %d\n", plan.RegisteredMigrationsCount())
		fmt.Printf("Executions count: %d\n", plan.FinishedExecutionsCount())
		fmt.Printf("Next to execute migration file: %s\n", nextMigFile)
		fmt.Printf("Last executed migration file: %s\n", lastMigFile)
	}

	return err
}

type GenerateBlankMigrationCommand struct {
	migrationsDir migration.MigrationsDirPath
}

func (c *GenerateBlankMigrationCommand) Name() string {
	return "blank"
}

func (c *GenerateBlankMigrationCommand) Description() string {
	return "Generates a new, blank migrations file in the configured migrations directory"
}

func (c *GenerateBlankMigrationCommand) Exec() error {
	fileName, err := migration.GenerateBlankMigration(c.migrationsDir)

	if err != nil {
		return err
	}

	fmt.Println("")
	fmt.Println("New blank migration file generated: " + fileName)
	fmt.Println("")

	return nil
}

func getVersionFrom(args []string) (uint64, error) {
	if len(args) < 2 {
		return 0, errors.New(
			"migration version is expected to be the second argument. None provided",
		)
	}

	migVersion, err := strconv.Atoi(args[1])

	if err != nil {
		return 0, fmt.Errorf(
			"migration version must be a valid numeric value. Failed with error: %w", err,
		)
	}

	return uint64(migVersion), nil
}

type MigrateForceUpCommand struct {
	handler *handler.MigrationsHandler
	args    []string
}

func (c *MigrateForceUpCommand) Name() string {
	return "force:up"
}

func (c *MigrateForceUpCommand) Description() string {
	return "Executes Up() forcefully for the provided migration version" +
		" (even if it was executed before)"
}

func (c *MigrateForceUpCommand) Exec() error {
	migVersion, err := getVersionFrom(c.args)

	if err != nil {
		return err
	}

	exec, err := c.handler.ForceUp(migVersion)

	if exec.Execution != nil {
		fmt.Printf("Executed Up() forcefully for %d migration\n", exec.Execution.Version)
	} else {
		fmt.Print("No forced Up() migration executed\n")
	}

	return err
}

type MigrateForceDownCommand struct {
	handler *handler.MigrationsHandler
	args    []string
}

func (c *MigrateForceDownCommand) Name() string {
	return "force:down"
}

func (c *MigrateForceDownCommand) Description() string {
	return "Executes Down() forcefully for the provided migration version" +
		" (even if it was executed before)"
}

func (c *MigrateForceDownCommand) Exec() error {
	migVersion, err := getVersionFrom(c.args)

	if err != nil {
		return err
	}

	exec, err := c.handler.ForceDown(migVersion)

	if exec.Execution != nil {
		fmt.Printf("Executed Down() forcefully for %d migration\n", exec.Execution.Version)
	} else {
		fmt.Print("No forced Down() migration executed\n")
	}

	return err
}