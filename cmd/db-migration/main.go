package main

import (
	"fmt"
	"os"

	"github.com/bontaramsonta/db-migration/internal/config"
	"github.com/bontaramsonta/db-migration/internal/console"
	"github.com/bontaramsonta/db-migration/internal/db"
	"github.com/bontaramsonta/db-migration/internal/migration"
)

func main() {
	// Initialize console for output
	cons := console.New(true) // verbose mode

	// Parse command line arguments
	cfg, err := config.ParseArgs(os.Args[1:])
	if err != nil {
		cons.Error("%v", err)
		printUsage()
		os.Exit(1)
	}

	// Connect to database
	cons.Info("Connecting to database %s@%s:%d/%s...", cfg.User, cfg.Host, cfg.Port, cfg.DBName)
	database, err := db.Connect(cfg.DSN())
	if err != nil {
		cons.Error("Database connection failed: %v", err)
		os.Exit(1)
	}
	defer database.Close()
	cons.Success("Database connection established")

	// Create and run migrator
	migrator := migration.NewMigrator(cfg, database, cons)
	if err := migrator.Run(); err != nil {
		cons.Error("Migration failed: %v", err)
		os.Exit(1)
	}

	os.Exit(0)
}

func printUsage() {
	fmt.Println()
	fmt.Println("Usage: db-migration <host> <user> <password> <dbname> <port> <scripts_dir> [missed_scripts_file]")
	fmt.Println()
	fmt.Println("Arguments:")
	fmt.Println("  host               MySQL host address")
	fmt.Println("  user               MySQL username")
	fmt.Println("  password           MySQL password")
	fmt.Println("  dbname             Database name")
	fmt.Println("  port               MySQL port number")
	fmt.Println("  scripts_dir        Directory containing SQL migration scripts")
	fmt.Println("  missed_scripts_file (optional) File containing list of missed scripts to execute")
	fmt.Println()
	fmt.Println("Example:")
	fmt.Println("  db-migration localhost root password mydb 3306 ./migrations")
	fmt.Println("  db-migration localhost root password mydb 3306 ./migrations missed.txt")
	fmt.Println()
}

