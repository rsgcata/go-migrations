package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	_ "github.com/go-sql-driver/mysql"
	"github.com/rsgcata/go-migrations/migration"
	"github.com/rsgcata/go-migrations/repository"
	"github.com/rsgcata/go-migrations/tmp"
)

func main() {
	ctx := context.Background()
	_, filename, _, _ := runtime.Caller(0)
	dirPath, err := migration.NewMigrationsDirPath(
		filepath.Join(filepath.Dir(filename), "tmp"),
	)

	if err != nil {
		panic(fmt.Errorf("invalid migrations path: %w", err))
	}

	dbDsn := getDbDsn()
	repoDb, err := repository.NewDbHandle(dbDsn)

	if err != nil {
		panic(fmt.Errorf("failed to build executions repository db: %w", err))
	}

	migRegistry := migration.NewDirMigrationsRegistry(dirPath)
	populateRegistry(migRegistry, ctx, dbDsn)
	repo := repository.NewMysqlHandler(repoDb, "migration_executions", context.Background())

	Bootstrap(os.Args[1:], migRegistry, repo, dirPath)
}

func getDbDsn() string {
	dbName := os.Getenv("MYSQL_DATABASE")
	dsn := os.Getenv("MYSQL_DSN")

	if dbName == "" {
		dbName = "migrations"
	}

	if dsn == "" {
		// Needed if ran from host machine because we are missing the env variables
		// See pass and port in .env file
		dsn = "root:123456789@tcp(localhost:3306)/" + dbName
	}

	return dsn
}

func populateRegistry(
	migRegistry *migration.DirMigrationsRegistry,
	ctx context.Context,
	dbDsn string,
) {
	// New db needed to not conflict with executions repository connections
	db, err := sql.Open("mysql", dbDsn)

	if err != nil {
		panic(fmt.Errorf("failed to build migrations db: %w", err))
	}

	migRegistry.Register(&tmp.Migration1712953077{Db: db})
	migRegistry.Register(&tmp.Migration1712953080{Db: db})
	migRegistry.Register(&tmp.Migration1712953083{Db: db, Ctx: ctx})
}
