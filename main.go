package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/rsgcata/go-migrations/migrations"
)

func main() {
	_, b, _, _ := runtime.Caller(0)
	basepath := filepath.Dir(b)

	// Generates a blank migration file
	err := migrations.GenerateBlankMigration(basepath + string(os.PathSeparator) + "tmp")
	fmt.Println(err)
}
