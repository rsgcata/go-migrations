//go:build postgres

package repository

import (
	"context"
	"database/sql"
	"os"
	_ "strconv"
	"strings"
	"testing"

	_ "github.com/lib/pq"
	"github.com/rsgcata/go-migrations/execution"
	"github.com/rsgcata/go-migrations/migration"
	"github.com/stretchr/testify/suite"
)

const PostgresDsnEnv = "POSTGRES_DSN"
const PostgresDbNameEnv = "POSTGRES_DATABASE"
const PostgresExecutionsTable = "migration_executions"

type PostgresTestSuite struct {
	suite.Suite
	dbName  string
	dsn     string
	db      *sql.DB
	handler *PostgresHandler
}

func TestPostgresTestSuite(t *testing.T) {
	suite.Run(t, new(PostgresTestSuite))
}

func (suite *PostgresTestSuite) SetupSuite() {
	suite.dbName = os.Getenv(PostgresDbNameEnv)
	suite.dsn = os.Getenv(PostgresDsnEnv)

	if suite.dbName == "" {
		// Needed if tests are ran on the host not docker
		suite.dbName = "migrations"
	}

	if suite.dsn == "" {
		// Needed if tests are ran on the host not docker
		suite.dsn = "postgres://postgres:123456789@localhost:5432/" + suite.dbName + "?sslmode=disable"
	}

	// Connect to postgres without database to create test database
	tmpDsn := strings.Replace(suite.dsn, "/"+suite.dbName, "/postgres", 1)
	tmpDb, _ := sql.Open("postgres", tmpDsn)
	_, _ = tmpDb.Exec("DROP DATABASE IF EXISTS " + suite.dbName)
	_, _ = tmpDb.Exec("CREATE DATABASE " + suite.dbName)
	_ = tmpDb.Close()

	suite.handler, _ = NewPostgresHandler(
		suite.dsn,
		PostgresExecutionsTable,
		context.Background(),
		nil,
	)
	suite.db = suite.handler.db
}

func (suite *PostgresTestSuite) TearDownSuite() {
	// Close the connection before dropping the database
	_ = suite.db.Close()

	// Connect to postgres without database to drop test database
	tmpDsn := strings.Replace(suite.dsn, "/"+suite.dbName, "/postgres", 1)
	tmpDb, _ := sql.Open("postgres", tmpDsn)
	//_, _ = tmpDb.Exec("DROP DATABASE IF EXISTS " + suite.dbName)
	_ = tmpDb.Close()
}

func (suite *PostgresTestSuite) SetupTest() {
	_ = suite.handler.Init()
	_, _ = suite.db.Exec(`DELETE FROM "` + PostgresExecutionsTable + `"`)
}

func (suite *PostgresTestSuite) TearDownTest() {
	_, _ = suite.db.Exec(`DELETE FROM "` + PostgresExecutionsTable + `"`)
}

func (suite *PostgresTestSuite) TestItCanBuildMigrationsExclusiveDbHandle() {
	handle, err := newDbHandle(suite.dsn, "postgres")

	suite.Assert().Nil(err)
	suite.Assert().Equal(1, handle.Stats().MaxOpenConnections)

	var dbName string
	_ = handle.QueryRow("SELECT current_database()").Scan(&dbName)
	suite.Assert().Equal(suite.dbName, dbName)
}

func (suite *PostgresTestSuite) TestItCanBuildHandlerWithProvidedContext() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	handler, err := NewPostgresHandler(suite.dsn, "migration_execs", ctx, nil)
	suite.Assert().Nil(err)
	suite.Assert().Same(ctx, handler.Context())
}

func (suite *PostgresTestSuite) TestItCanInitializeExecutionsTable() {
	_, _ = suite.db.Exec(`DROP TABLE IF EXISTS "` + PostgresExecutionsTable + `"`)
	tableExists := func() bool {
		var exists bool
		_ = suite.db.QueryRow(
			`
			SELECT EXISTS (
				SELECT FROM pg_tables 
				WHERE schemaname = 'public' 
				AND tablename = $1
			)`, PostgresExecutionsTable,
		).Scan(&exists)
		return exists
	}

	suite.Assert().False(tableExists())
	_ = suite.handler.Init()
	suite.Assert().True(tableExists())
}

func (suite *PostgresTestSuite) TestItCanLoadExecutions() {
	executions := executionsProvider()

	for _, exec := range executions {
		_, _ = suite.db.Exec(
			`INSERT INTO "`+PostgresExecutionsTable+`" VALUES ($1, $2, $3)`,
			exec.Version, exec.ExecutedAtMs, exec.FinishedAtMs,
		)
	}

	loadedExecs, err := suite.handler.LoadExecutions()

	suite.Assert().NoError(err)
	for _, exec := range loadedExecs {
		suite.Assert().Contains(executions, exec.Version)
		suite.Assert().Equal(executions[exec.Version], exec)
		delete(executions, exec.Version)
	}
	suite.Assert().Len(executions, 0)
}

func (suite *PostgresTestSuite) TestItFailsToExecuteAnyChangesWhenMissingTable() {
	_, _ = suite.db.Exec(`DROP TABLE IF EXISTS "` + ExecutionsTable + `"`)
	migrationExecution := execution.StartExecution(migration.NewDummyMigration(123))
	_, errLoad := suite.handler.LoadExecutions()
	errSave := suite.handler.Save(*migrationExecution)
	errRemove := suite.handler.Remove(*migrationExecution)
	_, errFindOne := suite.handler.FindOne(uint64(123))

	suite.Assert().Error(errLoad)
	suite.Assert().ErrorContains(errLoad, ExecutionsTable)
	suite.Assert().Error(errSave)
	suite.Assert().ErrorContains(errSave, ExecutionsTable)
	suite.Assert().Error(errRemove)
	suite.Assert().ErrorContains(errRemove, ExecutionsTable)
	suite.Assert().Error(errFindOne)
	suite.Assert().ErrorContains(errFindOne, ExecutionsTable)
}

func (suite *PostgresTestSuite) TestItFailsToLoadExecutionsFromInvalidRepoData() {
	_, _ = suite.db.Exec(
		`ALTER TABLE "` + ExecutionsTable + `" 
		 ALTER COLUMN finished_at_ms DROP NOT NULL`,
	)
	_, _ = suite.db.Exec(
		`INSERT INTO "` + ExecutionsTable + `" 
		 VALUES (1, 2, 1), (3, 4, NULL)`,
	)
	execs, err := suite.handler.LoadExecutions()
	suite.Assert().Len(execs, 1)
	suite.Assert().Error(err)
	suite.Assert().ErrorContains(err, "Scan error")
}

func (suite *PostgresTestSuite) TestItCanSaveExecutions() {
	// Insert
	executions := executionsProvider()

	for _, exec := range executions {
		err := suite.handler.Save(exec)
		suite.Assert().NoError(err)
	}

	savedExecs, _ := suite.handler.LoadExecutions()
	for _, exec := range savedExecs {
		suite.Assert().Contains(executions, exec.Version)
		suite.Assert().Equal(executions[exec.Version], exec)
	}

	// Update
	for i, exec := range executions {
		exec.FinishedAtMs++
		exec.ExecutedAtMs++
		executions[i] = exec
		err := suite.handler.Save(executions[i])
		suite.Assert().NoError(err)
	}

	savedExecs, _ = suite.handler.LoadExecutions()
	for _, exec := range savedExecs {
		suite.Assert().Contains(executions, exec.Version)
		suite.Assert().Equal(executions[exec.Version], exec)
	}
}

func (suite *PostgresTestSuite) TestItCanRemoveExecution() {
	executions := executionsProvider()

	for _, exec := range executions {
		_ = suite.handler.Save(exec)
		err := suite.handler.Remove(exec)
		suite.Assert().NoError(err)
	}

	savedExecs, _ := suite.handler.LoadExecutions()

	suite.Assert().Len(savedExecs, 0)
}

func (suite *PostgresTestSuite) TestItCanFindOne() {
	executions := executionsProvider()

	for _, exec := range executions {
		_, _ = suite.db.Exec(
			`INSERT INTO "`+PostgresExecutionsTable+`" VALUES ($1, $2, $3)`,
			exec.Version, exec.ExecutedAtMs, exec.FinishedAtMs,
		)
	}

	execToFind := executions[uint64(4)]
	foundExec, err := suite.handler.FindOne(uint64(4))
	suite.Assert().Equal(&execToFind, foundExec)
	suite.Assert().Nil(err)
	_, _ = suite.db.Exec(`DELETE FROM "` + ExecutionsTable + `"`)
	foundExec, err = suite.handler.FindOne(uint64(4))
	suite.Assert().Nil(foundExec)
	suite.Assert().Nil(err)
}
