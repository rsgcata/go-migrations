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
	tmpDb.Exec("DROP DATABASE IF EXISTS " + suite.dbName)
	tmpDb.Exec("CREATE DATABASE " + suite.dbName)
	tmpDb.Close()

	suite.db, _ = sql.Open("mysql", suite.dsn)
	suite.handler = &MysqlHandler{suite.db, ExecutionsTable, context.Background()}
}

func (suite *MysqlTestSuite) TearDownSuite() {
	suite.db.Exec("DROP DATABASE IF EXISTS " + suite.dbName)
	suite.db.Close()
}

func (suite *MysqlTestSuite) SetupTest() {
	suite.handler.Init()
	suite.db.Exec("DELETE FROM " + ExecutionsTable)
}

func (suite *MysqlTestSuite) TearDownTest() {
	suite.db.Exec("DELETE FROM " + ExecutionsTable)
}

func (suite *MysqlTestSuite) TestItCanInitializeExecutionsTable() {
	suite.db.Exec("DROP TABLE IF EXISTS " + ExecutionsTable)
	tableExists := func() bool {
		var table string
		suite.db.QueryRow("SHOW TABLES LIKE '" + ExecutionsTable + "'").Scan(&table)
		return table == ExecutionsTable
	}

	suite.Assert().False(tableExists())
	suite.handler.Init()
	suite.Assert().True(tableExists())
}

func executionsPorvider() map[uint64]execution.MigrationExecution {
	return map[uint64]execution.MigrationExecution{
		uint64(1): {Version: 1, ExecutedAtMs: 2, FinishedAtMs: 3},
		uint64(4): {Version: 4, ExecutedAtMs: 5, FinishedAtMs: 6},
		uint64(7): {Version: 7, ExecutedAtMs: 8, FinishedAtMs: 9},
	}
}

func (suite *MysqlTestSuite) TestItCanLoadExecutions() {
	executions := executionsPorvider()

	for _, exec := range executions {
		suite.db.Exec(
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

func (suite *MysqlTestSuite) TestItCanSasveExecutions() {
	// Insert
	executions := executionsPorvider()

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
	executions := executionsPorvider()

	for _, exec := range executions {
		suite.handler.Save(exec)
		err := suite.handler.Remove(exec)
		suite.Assert().NoError(err)
	}

	savedExecs, _ := suite.handler.LoadExecutions()

	suite.Assert().Len(savedExecs, 0)
}
