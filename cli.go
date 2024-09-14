package main

import (
	"errors"
	"fmt"
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
) {
	handler, err := NewHandler(registry, repository, nil)

	if err != nil {
		panic(
			fmt.Errorf(
				"coult not bootstrap cli, %s: %w",
				"failed to create new migrations handler with error", err,
			),
		)
	}

	inputCmd := "help"

	if len(args) > 1 {
		if args[0] == "--" {
			args = args[1:]
		}

		inputCmd = args[0]
	}

	availableCommands := make(map[string]Command)

	oneUp := &MigrateOneDownCommand{handler: handler}
	oneDown := &MigrateOneUpCommand{handler: handler}
	allUp := &MigrateAllUpCommand{handler: handler}
	allDown := &MigrateAllDownCommand{handler: handler}
	forceUp := &MigrateForceUpCommand{handler: handler, args: args}
	forceDown := &MigrateForceDownCommand{handler: handler, args: args}
	stats := &MigrateStatsCommand{registry: registry, repository: repository}
	blank := &GenerateBlankMigrationCommand{dirPath}
	availableCommands[oneUp.Name()] = oneUp
	availableCommands[oneDown.Name()] = oneDown
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

type MigrateOneDownCommand struct {
	handler *MigrationsHandler
}

func (c *MigrateOneDownCommand) Name() string {
	return "one:down"
}

func (c *MigrateOneDownCommand) Description() string {
	return "Executes Down() for the last executed migration"
}

func (c *MigrateOneDownCommand) Exec() error {
	execMig, err := c.handler.MigrateOneDown()

	if execMig.Execution != nil {
		fmt.Printf("Executed Down() for %d migration\n", execMig.Execution.Version)
	} else {
		fmt.Print("No migration Down() executed\n")
	}

	return err
}

type MigrateOneUpCommand struct {
	handler *MigrationsHandler
}

func (c *MigrateOneUpCommand) Name() string {
	return "one:up"
}

func (c *MigrateOneUpCommand) Description() string {
	return "Executes Up() for the next registered and not yet executed migration"
}

func (c *MigrateOneUpCommand) Exec() error {
	execMig, err := c.handler.MigrateOneUp()

	if execMig.Execution != nil {
		fmt.Printf("Executed Up() for %d migration\n", execMig.Migration.Version())
	} else {
		fmt.Print("No migration Up() executed\n")
	}

	return err
}

type MigrateAllUpCommand struct {
	handler *MigrationsHandler
}

func (c *MigrateAllUpCommand) Name() string {
	return "all:up"
}

func (c *MigrateAllUpCommand) Description() string {
	return "Executes Up() for the all registered and not yet executed migrations"
}

func (c *MigrateAllUpCommand) Exec() error {
	execs, err := c.handler.MigrateAllUp()

	fmt.Printf("Executed %d migrations\n", len(execs))

	for _, execMig := range execs {
		if execMig.Execution != nil {
			fmt.Printf("Executed Up() for %d migration\n", execMig.Execution.Version)
		}
	}

	return err
}

type MigrateAllDownCommand struct {
	handler *MigrationsHandler
}

func (c *MigrateAllDownCommand) Name() string {
	return "all:down"
}

func (c *MigrateAllDownCommand) Description() string {
	return "Executes Down() for the all executed migrations"
}

func (c *MigrateAllDownCommand) Exec() error {
	execs, err := c.handler.MigrateAllDown()

	fmt.Printf("Executed %d migrations\n", len(execs))

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
	plan, err := NewPlan(c.registry, c.repository)

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
		fmt.Printf("NextToExecute migration file: %s\n", nextMigFile)
		fmt.Printf("LastExecuted migration file: %s\n", lastMigFile)
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
	handler *MigrationsHandler
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
		fmt.Print("No migration executed\n")
	}

	return err
}

type MigrateForceDownCommand struct {
	handler *MigrationsHandler
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
		fmt.Print("No migration executed\n")
	}

	return err
}
