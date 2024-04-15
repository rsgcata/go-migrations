package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/rsgcata/go-migrations/migration"
)

func main() {
	_, b, _, _ := runtime.Caller(0)
	basepath := filepath.Dir(b)

	// Generates a blank migration file
	dirPath, _ := migration.NewMigrationsDirPath(basepath + string(os.PathSeparator) + "tmp")
	err := migration.GenerateBlankMigration(dirPath)
	fmt.Println(err)
}
