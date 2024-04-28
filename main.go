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
	dirPath, err := migration.NewMigrationsDirPath(filepath.Dir(filename))

	if err != nil {
		fmt.Println("Invalid migrations path", err)
		os.Exit(1)
	}

	db, err := newDb()

	if err != nil {
		fmt.Println("Failed to connect to db", err)
		os.Exit(1)
	}

	migRegistry := migration.NewDirMigrationsRegistry(dirPath)
	populateRegistry(migRegistry, db, ctx)
	repo := repository.NewMysqlHandler(db, "migration_executions", context.Background())
	handler, err := NewExecutionsHandler(migRegistry, repo)

	if err != nil {
		fmt.Println("Failed to build executions handler\n", err)
		os.Exit(1)
	}

	Bootstrap(os.Args[1:], handler)
	// // Generates a blank migration file
	// dirPath, _ := migration.NewMigrationsDirPath(basepath + string(os.PathSeparator) + "tmp")
	// err := migration.GenerateBlankMigration(dirPath)
	// fmt.Println(err)
}

func newDb() (*sql.DB, error) {
	dbName := os.Getenv("MYSQL_DATABASE")
	dsn := os.Getenv("MYSQL_DSN")

	if dbName == "" {
		// Needed if tests are ran on the host not docker
		dbName = "migrations"
	}

	if dsn == "" {
		// Needed if tests are ran on the host not docker
		dsn = "root:123456789@tcp(localhost:3306)/" + dbName
	}

	return sql.Open("mysql", dsn)
}

func populateRegistry(
	migRegistry *migration.DirMigrationsRegistry,
	db *sql.DB,
	ctx context.Context,
) {
	migRegistry.Register(&tmp.Migration1712953077{Db: db})
	migRegistry.Register(&tmp.Migration1712953080{Db: db})
	migRegistry.Register(&tmp.Migration1712953083{Db: db, Ctx: ctx})
}
