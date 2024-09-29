package cli

import (
	"errors"
	"github.com/rsgcata/go-migrations/execution"
	"github.com/rsgcata/go-migrations/migration"
	"github.com/stretchr/testify/suite"
	"io"
	"os"
	"testing"
)

type CliTestSuite struct {
	suite.Suite
}

func TestCliTestSuite(t *testing.T) {
	suite.Run(t, new(CliTestSuite))
}

func (suite *CliTestSuite) TestItFailsToBootstrapCliWhenMigrationsHandlerInitFails() {
	expectedErr := errors.New("init failed")
	repo := &execution.InMemoryRepository{
		InitErr: expectedErr,
	}

	defer func() {
		actualErr := recover().(error)
		suite.Assert().ErrorContains(actualErr, expectedErr.Error())
	}()

	var migPath migration.MigrationsDirPath
	var registry migration.MigrationsRegistry
	Bootstrap([]string{}, registry, repo, migPath, nil)
}

func (suite *CliTestSuite) TestItCanRunTheGivenCommand() {
	helpCmdOutput := "Displays helpful information about this tool"
	scenarios := map[string]struct {
		inputArgs      []string
		expectedOutput string
	}{
		"help default":              {[]string{"test123"}, helpCmdOutput},
		"help default with go run":  {[]string{"--", "test123"}, helpCmdOutput},
		"help explicit":             {[]string{"help"}, helpCmdOutput},
		"help explicit with go run": {[]string{"--", "help"}, helpCmdOutput},
		"up explicit":               {[]string{"up"}, "Executed Up() for 0 migrations"},
		"down explicit":             {[]string{"down"}, "Executed Down() for 0 migrations"},
		"force up up explicit": {
			[]string{"force:up", "123"},
			"No forced Up() migration executed",
		},
		"force down explicit": {
			[]string{"force:down", "123"},
			"No forced Down() migration executed",
		},
	}

	for name, scenario := range scenarios {
		rescueStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		migPath, _ := migration.NewMigrationsDirPath(suite.T().TempDir())
		registry := migration.NewEmptyDirMigrationsRegistry(migPath)
		Bootstrap(
			scenario.inputArgs,
			registry,
			&execution.InMemoryRepository{},
			migPath,
			nil,
		)

		_ = w.Close()
		actualOutput, _ := io.ReadAll(r)
		os.Stdout = rescueStdout
		suite.Assert().Contains(
			string(actualOutput), scenario.expectedOutput, "failed scenario %s", name,
		)
	}
}
