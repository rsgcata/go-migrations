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

const DSN_ENV = "MYSQL_DSN"
const DB_NAME_ENV = "MYSQL_DATABASE"
const EXECUTIONS_TABLE = "migration_executions"

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
	suite.dbName = os.Getenv(DB_NAME_ENV)
	suite.dsn = os.Getenv(DSN_ENV)

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
	suite.handler = &MysqlHandler{suite.db, EXECUTIONS_TABLE, context.Background()}
}

func (suite *MysqlTestSuite) TearDownSuite() {
	suite.db.Exec("DROP DATABASE IF EXISTS " + suite.dbName)
	suite.db.Close()
}

func (suite *MysqlTestSuite) SetupTest() {
	suite.handler.Init()
	suite.db.Exec("DELETE FROM " + EXECUTIONS_TABLE)
}

func (suite *MysqlTestSuite) TearDownTest() {
	suite.db.Exec("DELETE FROM " + EXECUTIONS_TABLE)
}

func (suite *MysqlTestSuite) TestItCanInitializeExecutionsTable() {
	suite.db.Exec("DROP TABLE IF EXISTS " + EXECUTIONS_TABLE)
	tableExists := func() bool {
		var table string
		suite.db.QueryRow("show tables like '" + EXECUTIONS_TABLE + "'").Scan(&table)
		return table == EXECUTIONS_TABLE
	}

	suite.Assert().False(tableExists())
	suite.handler.Init()
	suite.Assert().True(tableExists())
}

func (suite *MysqlTestSuite) TestItCanHandleRepositoryLocking() {
	con2, _ := sql.Open("mysql", suite.dsn)
	con2.SetMaxOpenConns(1)
	defer con2.Close()

	version := 123
	suite.db.Exec(
		"insert into " + EXECUTIONS_TABLE + " values (" + strconv.Itoa(version) + ",123,123)",
	)

	isLocked := func() bool {
		var foundVersion int
		tx, _ := con2.Begin()
		tx.Exec("SET @@lock_wait_timeout=1")
		tx.QueryRow("select version from " + EXECUTIONS_TABLE).Scan(&foundVersion)
		tx.Commit()
		return foundVersion != version
	}

	// not locked yet
	suite.Assert().False(isLocked())

	// lock repo
	suite.handler.Lock()
	suite.Assert().True(isLocked())

	// Unlock again
	suite.handler.Unlock()
	suite.Assert().False(isLocked())
}

func executionsPorvider() map[uint64]execution.MigrationExecution {
	executions := make(map[uint64]execution.MigrationExecution)
	executions[uint64(1)] = execution.MigrationExecution{
		Version: 1, ExecutedAtMs: 2, FinishedAtMs: 3,
	}
	executions[uint64(4)] = execution.MigrationExecution{
		Version: 4, ExecutedAtMs: 5, FinishedAtMs: 6,
	}
	executions[uint64(7)] = execution.MigrationExecution{
		Version: 7, ExecutedAtMs: 8, FinishedAtMs: 9,
	}
	return executions
}

func (suite *MysqlTestSuite) TestItCanLoadExecutions() {
	executions := executionsPorvider()

	for _, exec := range executions {
		suite.db.Exec(
			"insert into " + EXECUTIONS_TABLE + " values (" +
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
