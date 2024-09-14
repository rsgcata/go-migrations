package repository

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/rsgcata/go-migrations/execution"
	"github.com/rsgcata/go-migrations/migration"
	"github.com/stretchr/testify/suite"
)

const DnsEnv = "MYSQL_DSN"
const DbNameEnv = "MYSQL_DATABASE"
const ExecutionsTable = "migration_executions"

type MysqlTestSuite struct {
	suite.Suite
	dbName  string
	dsn     string
	db      *sql.DB
	handler *MysqlHandler
}

func TestMysqlTestSuite(t *testing.T) {
	suite.Run(t, new(MysqlTestSuite))
}

func (suite *MysqlTestSuite) SetupSuite() {
	suite.dbName = os.Getenv(DbNameEnv)
	suite.dsn = os.Getenv(DnsEnv)

	if suite.dbName == "" {
		// Needed if tests are ran on the host not docker
		suite.dbName = "migrations"
	}

	if suite.dsn == "" {
		// Needed if tests are ran on the host not docker
		suite.dsn = "root:123456789@tcp(localhost:3306)/" + suite.dbName
	}

	tmpDb, _ := sql.Open("mysql", strings.TrimRight(suite.dsn, suite.dbName))
	_, _ = tmpDb.Exec("DROP DATABASE IF EXISTS " + suite.dbName)
	_, _ = tmpDb.Exec("CREATE DATABASE " + suite.dbName)
	_ = tmpDb.Close()

	suite.handler, _ = NewMysqlHandler(suite.dsn, ExecutionsTable, context.Background())
	suite.db = suite.handler.db
}

func (suite *MysqlTestSuite) TearDownSuite() {
	_, _ = suite.db.Exec("DROP DATABASE IF EXISTS " + suite.dbName)
	_ = suite.db.Close()
}

func (suite *MysqlTestSuite) SetupTest() {
	_ = suite.handler.Init()
	_, _ = suite.db.Exec("DELETE FROM " + ExecutionsTable)
}

func (suite *MysqlTestSuite) TearDownTest() {
	_, _ = suite.db.Exec("DELETE FROM " + ExecutionsTable)
}

func (suite *MysqlTestSuite) TestItCanBuildMigrationsExclusiveDbHandle() {
	handle, err := newDbHandle(suite.dsn)

	suite.Assert().Nil(err)
	suite.Assert().Equal(1, handle.Stats().MaxOpenConnections)

	var dbName string
	_ = handle.QueryRow("select database()").Scan(&dbName)
	suite.Assert().Equal(suite.dbName, dbName)
}

func (suite *MysqlTestSuite) TestItCanBuildHandlerWithProvidedContext() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	handler, err := NewMysqlHandler(suite.dsn, "migration_execs", ctx)
	suite.Assert().Nil(err)
	suite.Assert().Same(ctx, handler.Context())
}

func (suite *MysqlTestSuite) TestItCanInitializeExecutionsTable() {
	_, _ = suite.db.Exec("DROP TABLE IF EXISTS " + ExecutionsTable)
	tableExists := func() bool {
		var table string
		_ = suite.db.QueryRow("SHOW TABLES LIKE '" + ExecutionsTable + "'").Scan(&table)
		return table == ExecutionsTable
	}

	suite.Assert().False(tableExists())
	_ = suite.handler.Init()
	suite.Assert().True(tableExists())
}

func executionsProvider() map[uint64]execution.MigrationExecution {
	return map[uint64]execution.MigrationExecution{
		uint64(1): {Version: 1, ExecutedAtMs: 2, FinishedAtMs: 3},
		uint64(4): {Version: 4, ExecutedAtMs: 5, FinishedAtMs: 6},
		uint64(7): {Version: 7, ExecutedAtMs: 8, FinishedAtMs: 9},
	}
}

func (suite *MysqlTestSuite) TestItCanLoadExecutions() {
	executions := executionsProvider()

	for _, exec := range executions {
		_, _ = suite.db.Exec(
			"insert into " + ExecutionsTable + " values (" +
				strconv.Itoa(int(exec.Version)) + "," +
				strconv.Itoa(int(exec.ExecutedAtMs)) + "," +
				strconv.Itoa(int(exec.FinishedAtMs)) + ")",
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

func (suite *MysqlTestSuite) TestItFailsToExecuteAnyChangesWhenMissingTable() {
	_, _ = suite.db.Exec("drop table `" + suite.handler.tableName + "`")
	migrationExecution := execution.StartExecution(migration.NewDummyMigration(123))
	_, errLoad := suite.handler.LoadExecutions()
	errSave := suite.handler.Save(*migrationExecution)
	errRemove := suite.handler.Remove(*migrationExecution)
	_, errFindOne := suite.handler.FindOne(uint64(123))

	suite.Assert().Error(errLoad)
	suite.Assert().ErrorContains(errLoad, suite.handler.tableName)
	suite.Assert().Error(errSave)
	suite.Assert().ErrorContains(errSave, suite.handler.tableName)
	suite.Assert().Error(errRemove)
	suite.Assert().ErrorContains(errRemove, suite.handler.tableName)
	suite.Assert().Error(errFindOne)
	suite.Assert().ErrorContains(errFindOne, suite.handler.tableName)
}

func (suite *MysqlTestSuite) TestItFailsToLoadExecutionsFromInvalidRepoData() {
	_, _ = suite.db.Exec(
		"alter table `" + suite.handler.tableName +
			"` modify column `finished_at_ms` bigint unsigned default null",
	)
	_, _ = suite.db.Exec("insert into `" + suite.handler.tableName + "` values (1,2,1), (3,4,null)")
	execs, err := suite.handler.LoadExecutions()
	suite.Assert().Len(execs, 1)
	suite.Assert().Error(err)
	suite.Assert().ErrorContains(err, "Scan error")
}

func (suite *MysqlTestSuite) TestItCanSaveExecutions() {
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

func (suite *MysqlTestSuite) TestItCanRemoveExecution() {
	executions := executionsProvider()

	for _, exec := range executions {
		_ = suite.handler.Save(exec)
		err := suite.handler.Remove(exec)
		suite.Assert().NoError(err)
	}

	savedExecs, _ := suite.handler.LoadExecutions()

	suite.Assert().Len(savedExecs, 0)
}

func (suite *MysqlTestSuite) TestItCanFindOne() {
	executions := executionsProvider()

	for _, exec := range executions {
		_, _ = suite.db.Exec(
			"insert into " + ExecutionsTable + " values (" +
				strconv.Itoa(int(exec.Version)) + "," +
				strconv.Itoa(int(exec.ExecutedAtMs)) + "," +
				strconv.Itoa(int(exec.FinishedAtMs)) + ")",
		)
	}

	execToFind := executions[uint64(4)]
	foundExec, err := suite.handler.FindOne(uint64(4))
	suite.Assert().Equal(&execToFind, foundExec)
	suite.Assert().Nil(err)
	_, _ = suite.db.Exec("delete from `" + suite.handler.tableName + "`")
	foundExec, err = suite.handler.FindOne(uint64(4))
	suite.Assert().Nil(foundExec)
	suite.Assert().Nil(err)
}
