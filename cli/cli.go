// Package cli provides functionality for building a command-line interface to manage database migrations.
//
// This package allows client code to bootstrap and handle user commands executed via the command
// line.
// It defines a Command interface that all migration commands must implement, and provides several
// built-in commands for common migration operations like migrating up, down, generating blank
// migrations, and displaying migration statistics.
package cli

import (
	"flag"
	"fmt"
	"github.com/rsgcata/go-cli-command/cli"
	"github.com/rsgcata/go-migrations/execution"
	"github.com/rsgcata/go-migrations/handler"
	"github.com/rsgcata/go-migrations/migration"
	"io"
	"strconv"
	"strings"
)

const MigrationsCmdLockName = "rsgcata-go-migrations"

type BootstrapSettings struct {
	// if the migration commands should lock the execution for exclusive runs
	RunMigrationsExclusively bool

	// The directory where the lock files will be saved
	RunLockFilesDirPath string

	// The name that will be used for generating the lock file name
	MigrationsCmdLockName string
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
//		cli.Bootstrap(
//			os.Args[1:],
//			migration.NewDirMigrationsRegistry(dirPath, allMigrations),
//			repository.NewMysqlHandler(dbDsn, "migration_executions", ctx, nil),
//			dirPath,
//			nil,
//			os.Stdout,
//			os.Exit,
//	        nil,
//		)
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
	outputWriter io.Writer,
	processExit func(code int),
	settings *BootstrapSettings,
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

	var up, down, forceUp, forceDown cli.Command
	up = &MigrateUpCommand{handler: migrationsHandler}
	down = &MigrateDownCommand{handler: migrationsHandler}
	forceUp = &MigrateForceUpCommand{handler: migrationsHandler}
	forceDown = &MigrateForceDownCommand{handler: migrationsHandler}

	if settings != nil && settings.RunMigrationsExclusively {
		lockName := MigrationsCmdLockName
		if inputLockName := strings.TrimSpace(settings.MigrationsCmdLockName); inputLockName != "" {
			lockName = inputLockName
		}

		up = cli.NewLockableCommandWithLockName(up, settings.RunLockFilesDirPath, lockName)
		down = cli.NewLockableCommandWithLockName(down, settings.RunLockFilesDirPath, lockName)
		forceUp = cli.NewLockableCommandWithLockName(
			forceUp,
			settings.RunLockFilesDirPath,
			lockName,
		)
		forceDown = cli.NewLockableCommandWithLockName(
			forceDown,
			settings.RunLockFilesDirPath,
			lockName,
		)
	}

	stats := &MigrateStatsCommand{registry: registry, repository: repository}
	blank := &GenerateBlankMigrationCommand{migrationsDir: dirPath}

	availableCommands := []cli.Command{
		up, down, forceUp, forceDown, blank, stats,
	}
	help := &HelpCommand{*cli.NewHelpCommand(availableCommands)}
	availableCommands = append(availableCommands, help)

	cmdRegistry := cli.NewCommandsRegistry()
	for _, cmd := range availableCommands {
		err = cmdRegistry.Register(cmd)
		if err != nil {
			panic(
				fmt.Errorf(
					"could not bootstrap cli, %s: %w",
					"failed to register migrations with error", err,
				),
			)
		}
	}

	cli.Bootstrap(args, cmdRegistry, outputWriter, processExit)
}

// HelpCommand implements the Command interface to display help information about all available commands.
// It serves as both the default command when no command is specified and as an explicit help command.
type HelpCommand struct {
	cli.HelpCommand
}

// MigrateUpCommand implements the Command interface to execute the Up() method
// of migrations that haven't been executed yet.
type MigrateUpCommand struct {
	steps     string
	numOfRuns handler.NumOfRuns
	handler   *handler.MigrationsHandler // Handler for executing migrations
}

func (c *MigrateUpCommand) Id() string {
	return "up"
}

func (c *MigrateUpCommand) Description() string {
	return "Executes Up() for the specified number of registered and not yet executed migrations."
}

func (c *MigrateUpCommand) DefineFlags(flagSet *flag.FlagSet) {
	flagSet.StringVar(
		&c.steps,
		"steps",
		"1",
		`
		Number of steps to execute. If the number of migrations to execute
		is not specified,defaults to 1.
		Allowed values for the number of migrations to run Up(): "all", 
		alias for 99999 and a valid integer greater than 0
		Examples: migrate up, migrate up --steps=all, migrate up --steps=3
		`,
	)
}

func (c *MigrateUpCommand) ValidateFlags() error {
	num, err := handler.NewNumOfRuns(c.steps)
	if err != nil {
		return err
	}
	c.numOfRuns = num
	return nil
}

func (c *MigrateUpCommand) Exec(stdWriter io.Writer) error {
	execs, err := c.handler.MigrateUp(c.numOfRuns)
	_, _ = fmt.Fprintf(stdWriter, "Executed Up() for %d migrations\n", len(execs))

	for _, execMig := range execs {
		if execMig.Execution != nil {
			_, _ = fmt.Fprintf(
				stdWriter, "Executed Up() for %d migration\n",
				execMig.Execution.Version,
			)
		}
	}

	return err
}

// MigrateDownCommand implements the Command interface to execute the Down() method
// of migrations that have been previously executed, effectively rolling them back.
type MigrateDownCommand struct {
	steps     string
	numOfRuns handler.NumOfRuns
	handler   *handler.MigrationsHandler // Handler for executing migrations
}

func (c *MigrateDownCommand) Id() string {
	return "down"
}

func (c *MigrateDownCommand) Description() string {
	return "Executes Down() for the specified number of executed migrations."
}

func (c *MigrateDownCommand) DefineFlags(flagSet *flag.FlagSet) {
	flagSet.StringVar(
		&c.steps,
		"steps",
		"1",
		"Number of steps to execute."+" If the number of migrations to execute is not specified, defaults to 1. Allowed"+
			" values for the number of migrations to run Down(): \"all\", "+
			"alias for 99999 and a valid"+
			" integer greater than 0\n"+
			"Examples: migrate down, migrate down --steps=all, migrate down --steps=3",
	)
}

func (c *MigrateDownCommand) ValidateFlags() error {
	num, err := handler.NewNumOfRuns(c.steps)
	if err != nil {
		return err
	}
	c.numOfRuns = num
	return nil
}

func (c *MigrateDownCommand) Exec(stdWriter io.Writer) error {
	execs, err := c.handler.MigrateDown(c.numOfRuns)
	_, _ = fmt.Fprintf(stdWriter, "Executed Down() for %d migrations\n", len(execs))

	for _, execMig := range execs {
		if execMig.Execution != nil {
			_, _ = fmt.Fprintf(
				stdWriter, "Executed Down() for %d migration\n",
				execMig.Execution.Version,
			)
		}
	}

	return err
}

// MigrateStatsCommand implements the Command interface to display statistics
// about registered migrations and their execution status.
type MigrateStatsCommand struct {
	cli.CommandWithoutFlags
	registry   migration.MigrationsRegistry // Registry containing all available migrations
	repository execution.Repository         // Repository for accessing migration execution state
}

func (c *MigrateStatsCommand) Id() string {
	return "stats"
}

func (c *MigrateStatsCommand) Description() string {
	return "Displays statistics about registered migrations and executions\n" +
		"Examples: migrate stats"
}

func (c *MigrateStatsCommand) Exec(stdWriter io.Writer) error {
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

		_, _ = fmt.Fprintln(stdWriter, "")
		_, _ = fmt.Fprintf(
			stdWriter,
			"Registered migrations count: %d\n",
			plan.RegisteredMigrationsCount(),
		)
		_, _ = fmt.Fprintf(
			stdWriter, "Executions count: %d\n", plan.FinishedExecutionsCount(),
		)
		_, _ = fmt.Fprintf(
			stdWriter, "Next to execute migration file: %s\n", nextMigFile,
		)
		_, _ = fmt.Fprintf(
			stdWriter, "Last executed migration file: %s\n", lastMigFile,
		)
	}

	return err
}

// GenerateBlankMigrationCommand implements the Command interface to create a new
// blank migration file in the configured migrations' directory.
type GenerateBlankMigrationCommand struct {
	cli.CommandWithoutFlags
	migrationsDir migration.MigrationsDirPath // Path to the directory where migration files are stored
}

func (c *GenerateBlankMigrationCommand) Id() string {
	return "blank"
}

func (c *GenerateBlankMigrationCommand) Description() string {
	return "Generates a new, blank migrations file in the configured migrations directory\n" +
		"Examples: migrate blank"
}

func (c *GenerateBlankMigrationCommand) Exec(stdWriter io.Writer) error {
	fileName, err := migration.GenerateBlankMigration(c.migrationsDir)

	if err != nil {
		return err
	}

	_, _ = fmt.Fprintln(stdWriter, "")
	_, _ = fmt.Fprintln(stdWriter, "New blank migration file generated: "+fileName)
	_, _ = fmt.Fprintln(stdWriter, "")

	return nil
}

func getVersionFrom(rawVersion string) (uint64, error) {
	migVersion, err := strconv.Atoi(rawVersion)

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
	rawVersion string
	migVersion uint64
	handler    *handler.MigrationsHandler // Handler for executing migrations
}

func (c *MigrateForceUpCommand) Id() string {
	return "force:up"
}

func (c *MigrateForceUpCommand) Description() string {
	return "Executes Up() forcefully for the provided migration version" +
		" (even if it was executed before)\n"
}

func (c *MigrateForceUpCommand) DefineFlags(flagSet *flag.FlagSet) {
	flagSet.StringVar(
		&c.rawVersion,
		"version",
		"",
		"Version number for force up.\n"+
			"Examples: migrate force:up --version=1712953077",
	)
}

func (c *MigrateForceUpCommand) ValidateFlags() error {
	version, err := getVersionFrom(c.rawVersion)
	if err != nil {
		return err
	}
	c.migVersion = version
	return nil
}

func (c *MigrateForceUpCommand) Exec(stdWriter io.Writer) error {
	exec, err := c.handler.ForceUp(c.migVersion)

	if exec.Execution != nil {
		_, _ = fmt.Fprintf(
			stdWriter, "Executed Up() forcefully for %d migration\n",
			exec.Execution.Version,
		)
	} else {
		_, _ = fmt.Fprintln(stdWriter, "No forced Up() migration executed")
	}

	return err
}

// MigrateForceDownCommand implements the Command interface to forcefully execute the Down() method
// of a specific migration, even if it hasn't been executed or has already been rolled back.
// This is useful for forcing the rollback of specific migrations.
type MigrateForceDownCommand struct {
	rawVersion string
	migVersion uint64
	handler    *handler.MigrationsHandler // Handler for executing migrations
}

func (c *MigrateForceDownCommand) Id() string {
	return "force:down"
}

func (c *MigrateForceDownCommand) Description() string {
	return "Executes Down() forcefully for the provided migration version" +
		" (even if it was executed before)\n"
}

func (c *MigrateForceDownCommand) DefineFlags(flagSet *flag.FlagSet) {
	flagSet.StringVar(
		&c.rawVersion,
		"version",
		"",
		"Version number for force down.\n"+
			"Examples: migrate force:down --version=1712953077",
	)
}

func (c *MigrateForceDownCommand) ValidateFlags() error {
	version, err := getVersionFrom(c.rawVersion)
	if err != nil {
		return err
	}
	c.migVersion = version
	return nil
}

func (c *MigrateForceDownCommand) Exec(stdWriter io.Writer) error {
	exec, err := c.handler.ForceDown(c.migVersion)

	if exec.Execution != nil {
		_, _ = fmt.Fprintf(
			stdWriter, "Executed Down() forcefully for %d migration\n",
			exec.Execution.Version,
		)
	} else {
		_, _ = fmt.Fprintln(stdWriter, "No forced Down() migration executed")
	}

	return err
}
