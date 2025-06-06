// Package cli provides functionality for building a command-line interface to manage database migrations.
//
// This package allows client code to bootstrap and handle user commands executed via the command
// line.
// It defines a Command interface that all migration commands must implement, and provides several
// built-in commands for common migration operations like migrating up, down, generating blank
// migrations, and displaying migration statistics.
package cli

import (
	"errors"
	"fmt"
	"github.com/rsgcata/go-migrations/handler"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/rsgcata/go-migrations/execution"
	"github.com/rsgcata/go-migrations/migration"
)

// Command defines the interface that all migration commands must implement.
// This interface serves as the specification for all commands the tool exposes as entrypoints.
type Command interface {
	// Name returns the command's name as it should be invoked from the command line.
	// For example, "up", "down", "stats", etc.
	Name() string

	// Description returns a human-readable description of what the command does,
	// including usage examples if applicable.
	Description() string

	// Exec executes the command's logic and returns an error if the execution fails.
	// This method is called when the command is invoked from the command line.
	Exec() error
}

// Bootstrap initializes the CLI application and processes user commands.
//
// This function sets up all the necessary components for handling migration commands,
// parses the command-line arguments, and executes the requested command.
// If no command is specified or an invalid command is provided, it displays the help information.
//
// Parameters:
//   - args: Command-line arguments (typically os.Args[1:])
//   - registry: Registry containing all available migrations
//   - repository: Repository for storing migration execution state
//   - dirPath: Path to the directory containing migration files
//   - newHandler: Optional function to create a custom migrations handler; if nil, the default handler.NewHandler is used
//
// Example:
//
//	cli.Bootstrap(
//		os.Args[1:],
//		migration.NewDirMigrationsRegistry(dirPath, allMigrations),
//		repository.NewMysqlHandler(dbDsn, "migration_executions", ctx, nil),
//		dirPath,
//		nil,
//	)
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

	up := &MigrateUpCommand{handler: migrationsHandler, args: args}
	down := &MigrateDownCommand{handler: migrationsHandler, args: args}
	forceUp := &MigrateForceUpCommand{handler: migrationsHandler, args: args}
	forceDown := &MigrateForceDownCommand{handler: migrationsHandler, args: args}
	stats := &MigrateStatsCommand{registry: registry, repository: repository}
	blank := &GenerateBlankMigrationCommand{dirPath}

	availableCommands := []Command{
		up, down, forceUp, forceDown, blank, stats,
	}

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

// HelpCommand implements the Command interface to display help information about all available commands.
// It serves as both the default command when no command is specified and as an explicit help command.
type HelpCommand struct {
	availableCommands []Command // List of all available commands to display information about
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

	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
	_, _ = fmt.Fprintln(writer, c.Name()+"\tDisplays helpful information about this tool")

	chunkDescription := func(description string, size int) []string {
		if len(description) == 0 {
			return []string{""}
		}
		var chunks []string

		accumulator := ""
		for _, char := range description {
			accumulator += string(char)
			if (len(accumulator) >= size && string(char) == " ") || string(char) == "\n" {
				chunks = append(chunks, strings.TrimSpace(accumulator))
				accumulator = ""
			}
		}

		if len(accumulator) > 0 {
			chunks = append(chunks, accumulator)
		}

		return chunks
	}

	for _, command := range c.availableCommands {
		_, _ = fmt.Fprintln(writer, "_________\t")
		descChunks := chunkDescription(command.Description(), 80)
		_, _ = fmt.Fprintln(writer, command.Name()+"\t"+descChunks[0])
		if len(descChunks) > 1 {
			for _, descChunk := range descChunks[1:] {
				_, _ = fmt.Fprintln(writer, "\t"+descChunk)
			}
		}
	}
	_ = writer.Flush()

	return nil
}

// MigrateUpCommand implements the Command interface to execute the Up() method
// of migrations that haven't been executed yet.
type MigrateUpCommand struct {
	handler *handler.MigrationsHandler // Handler for executing migrations
	args    []string                   // Command-line arguments
}

func (c *MigrateUpCommand) Name() string {
	return "up"
}

func (c *MigrateUpCommand) Description() string {
	return "Executes Up() for the specified number of registered and not yet executed migrations." +
		" If the number of migrations to execute is not specified, defaults to 1. Allowed" +
		" values for the number of migrations to run Up(): \"all\", alias for 99999 and a valid" +
		" integer greater than 0\n" +
		"Examples: migrate up, migrate up all, migrate up 3"
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

// MigrateDownCommand implements the Command interface to execute the Down() method
// of migrations that have been previously executed, effectively rolling them back.
type MigrateDownCommand struct {
	handler *handler.MigrationsHandler // Handler for executing migrations
	args    []string                   // Command-line arguments
}

func (c *MigrateDownCommand) Name() string {
	return "down"
}

func (c *MigrateDownCommand) Description() string {
	return "Executes Down() for the specified number of executed migrations." +
		" If the number of executions is not specified, defaults to 1. Allowed" +
		" values for the number of migrations to run Down(): \"all\", alias for 99999 and a valid" +
		" integer greater than 0\n" +
		"Examples: migrate down, migrate down all, migrate down 3"
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

// MigrateStatsCommand implements the Command interface to display statistics
// about registered migrations and their execution status.
type MigrateStatsCommand struct {
	registry   migration.MigrationsRegistry // Registry containing all available migrations
	repository execution.Repository         // Repository for accessing migration execution state
}

func (c *MigrateStatsCommand) Name() string {
	return "stats"
}

func (c *MigrateStatsCommand) Description() string {
	return "Displays statistics about registered migrations and executions\n" +
		"Examples: migrate stats"
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

// GenerateBlankMigrationCommand implements the Command interface to create a new
// blank migration file in the configured migrations' directory.
type GenerateBlankMigrationCommand struct {
	migrationsDir migration.MigrationsDirPath // Path to the directory where migration files are stored
}

func (c *GenerateBlankMigrationCommand) Name() string {
	return "blank"
}

func (c *GenerateBlankMigrationCommand) Description() string {
	return "Generates a new, blank migrations file in the configured migrations directory\n" +
		"Examples: migrate blank"
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

// getVersionFrom extracts and validates a migration version number from command-line arguments.
// It expects the version to be the second argument (index 1) in the args slice.
//
// Parameters:
//   - args: Command-line arguments containing the migration version
//
// Returns:
//   - uint64: The parsed migration version number
//   - error: An error if the version is missing or not a valid number
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

// MigrateForceUpCommand implements the Command interface to forcefully execute the Up() method
// of a specific migration, even if it has been executed before.
// This is useful for re-running migrations that need to be applied again.
type MigrateForceUpCommand struct {
	handler *handler.MigrationsHandler // Handler for executing migrations
	args    []string                   // Command-line arguments containing the migration version
}

func (c *MigrateForceUpCommand) Name() string {
	return "force:up"
}

func (c *MigrateForceUpCommand) Description() string {
	return "Executes Up() forcefully for the provided migration version" +
		" (even if it was executed before)\n" +
		"Examples: migrate force:up 1712953077"
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

// MigrateForceDownCommand implements the Command interface to forcefully execute the Down() method
// of a specific migration, even if it hasn't been executed or has already been rolled back.
// This is useful for forcing the rollback of specific migrations.
type MigrateForceDownCommand struct {
	handler *handler.MigrationsHandler // Handler for executing migrations
	args    []string                   // Command-line arguments containing the migration version
}

func (c *MigrateForceDownCommand) Name() string {
	return "force:down"
}

func (c *MigrateForceDownCommand) Description() string {
	return "Executes Down() forcefully for the provided migration version" +
		" (even if it was executed before)\n" +
		"Examples: migrate force:down 1712953077"
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
