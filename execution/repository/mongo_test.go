//go:build mongo

package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const MongoDnsEnv = "MONGO_DSN"
const MongoDbNameEnv = "MONGO_DATABASE"
const MongoCollectionName = "migration_executions"

type MongoTestSuite struct {
	suite.Suite
	dbName  string
	dsn     string
	client  *mongo.Client
	handler *MongoHandler
}

func TestMongoTestSuite(t *testing.T) {
	suite.Run(t, new(MongoTestSuite))
}

func (suite *MongoTestSuite) SetupSuite() {
	suite.dbName = os.Getenv(MongoDbNameEnv)
	suite.dsn = os.Getenv(MongoDnsEnv)

	if suite.dbName == "" {
		// Needed if tests are ran on the host not docker
		suite.dbName = "migrations"
	}

	if suite.dsn == "" {
		// Needed if tests are ran on the host not docker
		suite.dsn = "mongodb://localhost:27017"
	}

	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(suite.dsn).SetServerAPIOptions(serverAPI)
	opts.SetMaxPoolSize(1)
	opts.SetMaxConnIdleTime(3 * time.Second)
	opts.SetConnectTimeout(3 * time.Second)
	opts.SetServerSelectionTimeout(3 * time.Second)
	opts.SetTimeout(5 * time.Second)
	opts.SetSocketTimeout(5 * time.Second)
	client, _ := mongo.Connect(context.Background(), opts)

	suite.handler = &MongoHandler{client, suite.dbName, MongoCollectionName, context.Background()}
	suite.client = suite.handler.client
	_ = suite.handler.Init()
}

func (suite *MongoTestSuite) TearDownSuite() {
	_ = suite.client.Database(suite.dbName).Drop(context.Background())
}

func (suite *MongoTestSuite) SetupTest() {
	_, _ = suite.client.Database(suite.dbName).Collection(MongoCollectionName).DeleteMany(
		context.Background(), bson.D{},
	)
}

func (suite *MongoTestSuite) TearDownTest() {
	_, _ = suite.client.Database(suite.dbName).Collection(MongoCollectionName).DeleteMany(
		context.Background(), bson.D{},
	)
}

func (suite *MongoTestSuite) TestItCanInitializeTheRepository() {
	_ = suite.client.Database(suite.dbName).Collection(MongoCollectionName).
		Drop(context.Background())
	errInit1 := suite.handler.Init()
	errInit2 := suite.handler.Init()
	suite.Assert().Nil(errInit1)
	suite.Assert().Nil(errInit2)
	names, _ := suite.client.Database(suite.dbName).ListCollectionNames(suite.handler.ctx, bson.D{})
	suite.Assert().Contains(names, MongoCollectionName)
}

func (suite *MongoTestSuite) TestItCanLoadAllExecutions() {
	executions := executionsProvider()

	for _, exec := range executions {
		_, _ = suite.client.Database(suite.dbName).Collection(MongoCollectionName).InsertOne(
			context.Background(), toBsonExecution(exec),
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

func (suite *MongoTestSuite) TestItCanSaveExecutions() {
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

func (suite *MongoTestSuite) TestItCanRemoveExecution() {
	executions := executionsProvider()

	for _, exec := range executions {
		_ = suite.handler.Save(exec)
		err := suite.handler.Remove(exec)
		suite.Assert().NoError(err)
	}

	savedExecs, _ := suite.handler.LoadExecutions()

	suite.Assert().Len(savedExecs, 0)
}

func (suite *MongoTestSuite) TestItCanFindOne() {
	executions := executionsProvider()

	for _, exec := range executions {
		_ = suite.handler.Save(exec)
	}

	execToFind := executions[uint64(4)]
	foundExec, err := suite.handler.FindOne(uint64(4))
	suite.Assert().Equal(&execToFind, foundExec)
	suite.Assert().Nil(err)
	_ = suite.handler.Remove(*foundExec)
	foundExec, err = suite.handler.FindOne(uint64(4))
	suite.Assert().Nil(foundExec)
	suite.Assert().Nil(err)
}
