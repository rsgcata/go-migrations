package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

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
		fmt.Println("Invalid migrations path", err)
		os.Exit(1)
	}

	db := newDb()

	migRegistry := migration.NewDirMigrationsRegistry(dirPath)
	populateRegistry(migRegistry, ctx)
	repo := repository.NewMysqlHandler(db, "migration_executions", context.Background())

	Bootstrap(os.Args[1:], migRegistry, repo, dirPath)
}

func newDb() *sql.DB {
	dbName := os.Getenv("MYSQL_DATABASE")
	dsn := os.Getenv("MYSQL_DSN")

	if dbName == "" {
		dbName = "migrations"
	}

	if dsn == "" {
		// Needed if ran from host machine
		dsn = "root:123456789@tcp(localhost:3306)/" + dbName
	}

	// DB creation should run only once. This is added only for dev purpose
	db, _ := sql.Open("mysql", strings.TrimRight(dsn, dbName))
	db.Query("CREATE DATABASE IF NOT EXISTS " + dbName)
	db.Close()

	db, err := sql.Open("mysql", dsn)

	// Adjust this as needed, depending how much the migrations will last
	// Max open connections in some scenarios should be 1 because of repository lock/unlock
	// behaviour
	// db.SetMaxOpenConns(1)
	// db.SetMaxIdleConns(1)
	// db.SetConnMaxIdleTime(time.Hour)
	// db.SetConnMaxLifetime(time.Hour)

	if err != nil {
		panic(err)
	}

	return db
}

func populateRegistry(
	migRegistry *migration.DirMigrationsRegistry,
	ctx context.Context,
) {
	// New db needed to overcome the lock tables limitation where a session
	// needs to specify all tables it reads from when using LOCK TABLES
	db := newDb()

	migRegistry.Register(&tmp.Migration1712953077{Db: db})
	migRegistry.Register(&tmp.Migration1712953080{Db: db})
	migRegistry.Register(&tmp.Migration1712953083{Db: db, Ctx: ctx})
}
