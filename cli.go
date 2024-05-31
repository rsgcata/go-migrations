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
	handler, err := NewHandler(registry, repository)

	if err != nil {
		panic(
			fmt.Errorf(
				"coult not bootstrap cli, failed to create new migrations handler with error: %w", err,
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

	prev := &MigratePrevCommand{handler: handler}
	next := &MigrateNextCommand{handler: handler}
	up := &MigrateUpCommand{handler: handler}
	down := &MigrateDownCommand{handler: handler}
	forceUp := &MigrateForceUpCommand{handler: handler, args: args}
	forceDown := &MigrateForceDownCommand{handler: handler, args: args}
	stats := &MigrateStatsCommand{registry: registry, repository: repository}
	blank := &GenerateBlankMigrationCommand{dirPath}
	availableCommands[prev.Name()] = prev
	availableCommands[next.Name()] = next
	availableCommands[up.Name()] = up
	availableCommands[down.Name()] = down
	availableCommands[stats.Name()] = stats
	availableCommands[blank.Name()] = blank
	availableCommands[forceUp.Name()] = forceUp
	availableCommands[forceDown.Name()] = forceDown

	help := &HelpCommand{availableCommands: availableCommands}

	for _, cmd := range availableCommands {
		if inputCmd == cmd.Name() {
			err := cmd.Exec()

			if err != nil {
				fmt.Println("Failed to execute \"" + cmd.Name() + "\" with error: " + err.Error())
			}

			return
		}
	}

	help.Exec()
}

type HelpCommand struct {
	availableCommands map[string]Command
}

func (c *HelpCommand) Name() string {
	return "help"
}

func (c *HelpCommand) Description() string {
	return "Go Migrations is a database schema versioning tool" +
		" which helps to easly deploy schema changes"
}

func (c *HelpCommand) Exec() error {
	fmt.Println("")
	fmt.Println(c.Description())
	fmt.Println("")
	fmt.Println("Available commands:")
	fmt.Println("")

	wirter := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	fmt.Fprintln(wirter, c.Name()+"\tDisplays helpful information about this tool")

	var names []string

	for key := range c.availableCommands {
		names = append(names, key)
	}
	sort.Strings(names)

	for _, name := range names {
		fmt.Fprintln(
			wirter, c.availableCommands[name].Name()+"\t"+c.availableCommands[name].Description(),
		)
	}
	wirter.Flush()

	return nil
}

type MigratePrevCommand struct {
	handler *MigrationsHandler
}

func (c *MigratePrevCommand) Name() string {
	return "prev"
}

func (c *MigratePrevCommand) Description() string {
	return "Executes Down() for the last executed migration"
}

func (c *MigratePrevCommand) Exec() error {
	hmig, err := c.handler.MigratePrev()

	if hmig.Execution != nil {
		fmt.Printf("Executed Down() for %d migration\n", hmig.Execution.Version)
	} else {
		fmt.Print("No migration executed\n")
	}

	return err
}

type MigrateNextCommand struct {
	handler *MigrationsHandler
}

func (c *MigrateNextCommand) Name() string {
	return "next"
}

func (c *MigrateNextCommand) Description() string {
	return "Executes Up() for the next registered and not yet executed migration"
}

func (c *MigrateNextCommand) Exec() error {
	hmig, err := c.handler.MigrateNext()

	if hmig.Migration != nil {
		fmt.Printf("Executed Up() for %d migration\n", hmig.Migration.Version())
	} else {
		fmt.Print("No migration executed\n")
	}

	return err
}

type MigrateUpCommand struct {
	handler *MigrationsHandler
}

func (c *MigrateUpCommand) Name() string {
	return "up"
}

func (c *MigrateUpCommand) Description() string {
	return "Executes Up() for the all registered and not yet executed migrations"
}

func (c *MigrateUpCommand) Exec() error {
	mig, err := c.handler.MigrateUp()

	fmt.Printf("Executed %d migrations\n", len(mig))

	for _, hmig := range mig {
		if hmig.Execution != nil {
			fmt.Printf("Executed Up() for %d migration\n", hmig.Execution.Version)
		}
	}

	return err
}

type MigrateDownCommand struct {
	handler *MigrationsHandler
}

func (c *MigrateDownCommand) Name() string {
	return "down"
}

func (c *MigrateDownCommand) Description() string {
	return "Executes Down() for the all executed migrations"
}

func (c *MigrateDownCommand) Exec() error {
	mig, err := c.handler.MigrateDown()

	fmt.Printf("Executed %d migrations\n", len(mig))

	for _, hmig := range mig {
		if hmig.Execution != nil {
			fmt.Printf("Executed Down() for %d migration\n", hmig.Execution.Version)
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
	plan, err := NewExecutionPlan(c.registry, c.repository)

	if plan != nil {
		nextMigFile := "-"
		prevMigFile := "-"
		next := plan.Next()
		prev := plan.Prev()

		if next != nil {
			nextMigFile = migration.FileNamePrefix + migration.FileNameSeparator +
				strconv.Itoa(int(next.Version())) + ".go"
		}
		if prev != nil {
			prevMigFile = migration.FileNamePrefix + migration.FileNameSeparator +
				strconv.Itoa(int(prev.Version())) + ".go"
		}

		fmt.Println("")
		fmt.Printf("Registered migrations count: %d\n", plan.RegisteredMigrationsCount())
		fmt.Printf("Executions count: %d\n", plan.ExecutionsCount())
		fmt.Printf("Next migration file: %s\n", nextMigFile)
		fmt.Printf("Prev migration file: %s\n", prevMigFile)
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
	return "forceup"
}

func (c *MigrateForceUpCommand) Description() string {
	return "Executes Up() forcefully for the provided migration version"
}

func (c *MigrateForceUpCommand) Exec() error {
	migVersion, err := getVersionFrom(c.args)

	if err != nil {
		return err
	}

	hmig, err := c.handler.ForceUp(migVersion)

	if hmig.Execution != nil {
		fmt.Printf("Executed Up() forcefully for %d migration\n", hmig.Execution.Version)
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
	return "forcedown"
}

func (c *MigrateForceDownCommand) Description() string {
	return "Executes Down() forcefully for the provided migration version"
}

func (c *MigrateForceDownCommand) Exec() error {
	migVersion, err := getVersionFrom(c.args)

	if err != nil {
		return err
	}

	hmig, err := c.handler.ForceDown(migVersion)

	if hmig.Execution != nil {
		fmt.Printf("Executed Down() forcefully for %d migration\n", hmig.Execution.Version)
	} else {
		fmt.Print("No migration executed\n")
	}

	return err
}
