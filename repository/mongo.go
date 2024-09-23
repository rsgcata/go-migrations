package repository

import (
	"context"
	"errors"
	"github.com/rsgcata/go-migrations/execution"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type bsonExecution struct {
	Version      uint64 `bson:"_id"`
	ExecutedAtMs uint64 `bson:"executedAtMs"`
	FinishedAtMs uint64 `bson:"finishedAtMs"`
}

func toBsonExecution(exec execution.MigrationExecution) bsonExecution {
	return bsonExecution{
		Version:      exec.Version,
		ExecutedAtMs: exec.ExecutedAtMs,
		FinishedAtMs: exec.FinishedAtMs,
	}
}

func toMigrationExecution(exec bsonExecution) execution.MigrationExecution {
	return execution.MigrationExecution{
		Version:      exec.Version,
		ExecutedAtMs: exec.ExecutedAtMs,
		FinishedAtMs: exec.FinishedAtMs,
	}
}

func newMongoClient(dsn string, ctx context.Context) (*mongo.Client, error) {
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(dsn).SetServerAPIOptions(serverAPI)
	opts.SetMaxPoolSize(1)
	return mongo.Connect(ctx, opts)
}

type MongoHandler struct {
	client         *mongo.Client
	databaseName   string
	collectionName string
	ctx            context.Context
}

func NewMongoHandler(
	dsn string,
	databaseName string,
	collectionName string,
	ctx context.Context,
) (*MongoHandler, error) {
	db, err := newMongoClient(dsn, ctx)

	if err != nil {
		return nil, err
	}

	return &MongoHandler{db, databaseName, collectionName, ctx}, nil
}

func (h *MongoHandler) Context() context.Context {
	return h.ctx
}

func (h *MongoHandler) Init() error {
	names, err := h.client.Database(h.databaseName).ListCollectionNames(h.ctx, bson.D{})

	if err != nil {
		return err
	}

	for _, name := range names {
		if name == h.collectionName {
			return nil
		}
	}

	collectionOpts := options.CreateCollection()
	collectionOpts.SetValidator(
		bson.D{
			{
				"$jsonSchema", bson.D{
					{"bsonType", "object"},
					{"title", "migration execution object validation"},
					{
						"properties", bson.D{
							{
								"_id", bson.D{
									{"bsonType", "long"},
									{"minimum", 0},
									{
										"description",
										"_id (executed version) must be greater than 0",
									},
								},
							},
							{
								"executedAtMs", bson.D{
									{"bsonType", "long"},
									{"minimum", 0},
									{"description", "executed at must be greater than 0"},
								},
							},
							{
								"finishedAtMs", bson.D{
									{"bsonType", "long"},
									{"minimum", 0},
									{"description", "finished at must be greater than 0"},
								},
							},
						},
					},
				},
			},
		},
	)

	return h.client.Database(h.databaseName).CreateCollection(
		h.ctx, h.collectionName, collectionOpts,
	)
}

func (h *MongoHandler) LoadExecutions() (executions []execution.MigrationExecution, err error) {
	collection := h.client.Database(h.databaseName).Collection(h.collectionName)
	cursor, err := collection.Find(h.ctx, bson.D{})

	if err != nil {
		return nil, err
	}

	var bsonExecutions []bsonExecution
	if err = cursor.All(h.ctx, &bsonExecutions); err != nil {
		return nil, err
	}

	var migrationExecutions []execution.MigrationExecution
	for _, b := range bsonExecutions {
		migrationExecutions = append(migrationExecutions, toMigrationExecution(b))
	}

	return migrationExecutions, nil
}

func (h *MongoHandler) Save(exec execution.MigrationExecution) error {
	collection := h.client.Database(h.databaseName).Collection(h.collectionName)
	filter := bson.D{{"_id", exec.Version}}
	updateOpts := options.Update()
	updateOpts.SetUpsert(true)
	_, err := collection.UpdateOne(
		h.ctx, filter, bson.D{{"$set", toBsonExecution(exec)}}, updateOpts,
	)
	return err
}

func (h *MongoHandler) Remove(exec execution.MigrationExecution) error {
	collection := h.client.Database(h.databaseName).Collection(h.collectionName)
	filter := bson.D{{"_id", exec.Version}}
	_, err := collection.DeleteOne(h.ctx, filter)
	return err
}

func (h *MongoHandler) FindOne(version uint64) (*execution.MigrationExecution, error) {
	collection := h.client.Database(h.databaseName).Collection(h.collectionName)
	filter := bson.D{{"_id", version}}

	var result bsonExecution
	err := collection.FindOne(h.ctx, filter).Decode(&result)

	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	exec := toMigrationExecution(result)
	return &exec, err
}
