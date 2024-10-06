package main

import (
	"context"
	"fmt"
	"github.com/rsgcata/go-migrations/_examples/mongo/migrations"
	"github.com/rsgcata/go-migrations/cli"
	"github.com/rsgcata/go-migrations/execution/repository"
	"github.com/rsgcata/go-migrations/migration"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"os"
	"path/filepath"
)

func main() {
	ctx := context.Background()
	dirPath := createMigrationsDirPath()
	dbDsn := getDbDsn()
	cli.Bootstrap(
		os.Args[1:],
		buildRegistry(dirPath, ctx, dbDsn, getDbName()),
		createMongoRepository(dbDsn, ctx),
		dirPath,
		nil,
	)
}

func createMigrationsDirPath() migration.MigrationsDirPath {
	appBaseDir := os.Getenv("APP_BASE_DIR")

	if appBaseDir == "" {
		appBaseDir = "/go/src/migrations"
	}

	dirPath, err := migration.NewMigrationsDirPath(
		filepath.Join(appBaseDir, "_examples/mysql/migrations"),
	)

	if err != nil {
		panic(fmt.Errorf("invalid migrations path: %w", err))
	}

	return dirPath
}

func createMongoRepository(
	dbDsn string,
	ctx context.Context,
) *repository.MongoHandler {
	repo, err := repository.NewMongoHandler(
		dbDsn,
		getDbName(),
		getCollectionName(),
		ctx,
		nil,
	)

	if err != nil {
		panic(fmt.Errorf("failed to build executions repository: %w", err))
	}

	return repo
}

func getDbName() string {
	dbName := os.Getenv("MONGO_DATABASE")

	if dbName == "" {
		dbName = "migrations"
	}

	return dbName
}

func getCollectionName() string {
	collectionName := os.Getenv("MONGO_MIGRATIONS_COLLECTION")

	if collectionName == "" {
		collectionName = "migrations"
	}

	return collectionName
}

// getDbDsn Prepare the Mongo DSN
func getDbDsn() string {
	dsn := os.Getenv("MONGO_DSN")

	if dsn == "" {
		// Needed if ran from host machine because we are missing the env variables
		// See pass and port in .env file
		dsn = "mongodb://localhost:27017"
	}

	return dsn
}

// buildRegistry This will create a new registry and register all migrations
func buildRegistry(
	dirPath migration.MigrationsDirPath,
	ctx context.Context,
	dbDsn string,
	dbName string,
) *migration.DirMigrationsRegistry {
	// New db needed to not conflict with executions repository connections
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(dbDsn).SetServerAPIOptions(serverAPI)
	opts.SetMaxPoolSize(1)
	client, err := mongo.Connect(ctx, opts)

	if err != nil {
		panic(fmt.Errorf("failed to connect to migrations db: %w", err))
	}

	// It's not necessary to add them in order, the tool will handle ordering based on
	// their version number
	allMigrations := []migration.Migration{
		&migrations.Migration1712953077{Client: client, DbName: dbName, Ctx: ctx},
		&migrations.Migration1712953080{Client: client, DbName: dbName, Ctx: ctx},
		&migrations.Migration1712953083{Client: client, DbName: dbName, Ctx: ctx},
	}

	return migration.NewDirMigrationsRegistry(dirPath, allMigrations)
}
