package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration for the db-migration CLI
type Config struct {
	Host              string
	User              string
	Password          string
	DBName            string
	Port              int
	ScriptsDir        string
	MissedScriptsFile string // Optional
}

// ParseArgs parses command line arguments into Config
// Usage: db-migration <host> <user> <password> <dbname> <port> <scripts_dir> [missed_scripts_file]
func ParseArgs(args []string) (*Config, error) {
	if len(args) < 6 {
		return nil, fmt.Errorf("usage: db-migration <host> <user> <password> <dbname> <port> <scripts_dir> [missed_scripts_file]")
	}

	port, err := strconv.Atoi(args[4])
	if err != nil {
		return nil, fmt.Errorf("invalid port number: %s", args[4])
	}

	cfg := &Config{
		Host:       args[0],
		User:       args[1],
		Password:   args[2],
		DBName:     args[3],
		Port:       port,
		ScriptsDir: args[5],
	}

	if len(args) >= 7 {
		cfg.MissedScriptsFile = args[6]
	}

	// Validate scripts directory exists
	if _, err := os.Stat(cfg.ScriptsDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("scripts directory does not exist: %s", cfg.ScriptsDir)
	}

	// Validate missed scripts file exists if provided
	if cfg.MissedScriptsFile != "" {
		if _, err := os.Stat(cfg.MissedScriptsFile); os.IsNotExist(err) {
			return nil, fmt.Errorf("missed scripts file does not exist: %s", cfg.MissedScriptsFile)
		}
	}

	return cfg, nil
}

// DSN returns the MySQL Data Source Name connection string
func (c *Config) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&multiStatements=true",
		c.User, c.Password, c.Host, c.Port, c.DBName)
}

