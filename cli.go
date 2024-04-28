package main

import (
	"fmt"
	"os"
	"text/tabwriter"
)

type Command interface {
	Name() string
	Description() string
	Exec() error
}

func Bootstrap(args []string, handler *MigrationsHandler) {
	availableCommands := make(map[string]Command)

	prev := &MigratePrevCommand{}
	next := &MigrateNextCommand{}
	up := &MigrateUpCommand{handler: handler}
	down := &MigrateDownCommand{handler: handler}
	stats := &MigrateStatsCommand{}
	availableCommands[prev.Name()] = prev
	availableCommands[next.Name()] = next
	availableCommands[up.Name()] = up
	availableCommands[down.Name()] = down
	availableCommands[stats.Name()] = stats

	help := &HelpCommand{availableCommands: availableCommands}

	inputCmd := "help"

	if len(args) > 1 {
		inputCmd = args[1]
	}

	for _, cmd := range availableCommands {
		if inputCmd == cmd.Name() {
			err := cmd.Exec()

			if err != nil {
				fmt.Println(err)
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
	for _, ac := range c.availableCommands {
		fmt.Fprintln(wirter, ac.Name()+"\t"+ac.Description())
	}
	wirter.Flush()

	return nil
}

type MigratePrevCommand struct {
}

func (c *MigratePrevCommand) Name() string {
	return "prev"
}

func (c *MigratePrevCommand) Description() string {
	return "Executes Down() for the last executed migration"
}

func (c *MigratePrevCommand) Exec() error {
	return nil
}

type MigrateNextCommand struct {
}

func (c *MigrateNextCommand) Name() string {
	return "next"
}

func (c *MigrateNextCommand) Description() string {
	return "Executes Up() for the next registered and not yet executed migration"
}

func (c *MigrateNextCommand) Exec() error {
	return nil
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
	mig, err := c.handler.MigrateAllUp()

	fmt.Printf("Executed %d migrations\n", len(mig))

	for _, hmig := range mig {
		success := "success"

		if !hmig.Execution.Finished() {
			success = "failed"
		}

		fmt.Printf("Executed Up() for %d migration - %s\n", hmig.Execution.Version, success)
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
	mig, err := c.handler.MigrateAllDown()

	fmt.Printf("Executed %d migrations\n", len(mig))

	for _, hmig := range mig {
		fmt.Printf("Executed Down() for %d migration\n", hmig.Execution.Version)
	}

	return err
}

type MigrateStatsCommand struct {
}

func (c *MigrateStatsCommand) Name() string {
	return "stats"
}

func (c *MigrateStatsCommand) Description() string {
	return "Displays statistics about registered migrations and executions"
}

func (c *MigrateStatsCommand) Exec() error {
	return nil
}
