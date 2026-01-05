package testhelpers

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/bontaramsonta/db-migration/internal/db"
)

// TestDatabase wraps a MySQL database connection for testing
type TestDatabase struct {
	DB       *db.DB
	DSN      string
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

// getEnvOrDefault returns the environment variable value or the default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// SetupTestDB connects to the docker-compose MySQL and returns a test database instance.
// It waits for MySQL to become healthy with retries, then resets the database to ensure a clean state.
func SetupTestDB(t *testing.T) *TestDatabase {
	t.Helper()

	host := getEnvOrDefault("TEST_DB_HOST", "127.0.0.1")
	port := getEnvOrDefault("TEST_DB_PORT", "3307")
	user := getEnvOrDefault("TEST_DB_USER", "testuser")
	password := getEnvOrDefault("TEST_DB_PASSWORD", "testpassword")
	dbName := getEnvOrDefault("TEST_DB_NAME", "testdb")

	// Build DSN
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&multiStatements=true", user, password, host, port, dbName)

	// Connect to database with retries (wait for MySQL to be healthy)
	var database *db.DB
	var err error
	maxRetries := 10
	retryInterval := 2 * time.Second

	for i := 0; i < maxRetries; i++ {
		database, err = db.Connect(dsn)
		if err == nil {
			break
		}
		if i < maxRetries-1 {
			t.Logf("Waiting for MySQL... attempt %d/%d: %v", i+1, maxRetries, err)
			time.Sleep(retryInterval)
		}
	}
	if err != nil {
		t.Fatalf("failed to connect to test database after %d attempts: %v\nMake sure MySQL is running with: docker compose up -d", maxRetries, err)
	}

	testDB := &TestDatabase{
		DB:       database,
		DSN:      dsn,
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		DBName:   dbName,
	}

	// Reset database to clean state before each test
	if err := testDB.ResetDatabase(); err != nil {
		database.Close()
		t.Fatalf("failed to reset test database: %v", err)
	}

	// Register cleanup to close connection
	t.Cleanup(func() {
		testDB.DB.Close()
	})

	return testDB
}

// Exec executes a SQL query on the test database
func (td *TestDatabase) Exec(query string, args ...interface{}) error {
	_, err := td.DB.Exec(query, args...)
	return err
}

// QueryRow executes a query and returns a single row
func (td *TestDatabase) QueryRow(query string, args ...interface{}) *SingleRow {
	return &SingleRow{row: td.DB.QueryRow(query, args...)}
}

// SingleRow wraps sql.Row for test assertions
type SingleRow struct {
	row interface {
		Scan(dest ...interface{}) error
	}
}

// Scan scans the row into destination variables
func (r *SingleRow) Scan(dest ...interface{}) error {
	return r.row.Scan(dest...)
}

// TableExists checks if a table exists in the database
func (td *TestDatabase) TableExists(tableName string) (bool, error) {
	var count int
	err := td.DB.QueryRow(
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = ? AND table_name = ?",
		td.DBName, tableName,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetTableRowCount returns the number of rows in a table
func (td *TestDatabase) GetTableRowCount(tableName string) (int, error) {
	var count int
	err := td.DB.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// InsertTrackingRecord inserts a record into the sqlScriptExec tracking table
func (td *TestDatabase) InsertTrackingRecord(scriptName string, completed bool, endOfBatch bool, lastGitID string) error {
	_, err := td.DB.Exec(
		"INSERT INTO sqlScriptExec (scriptName, completed, endofbatch, lastgitid) VALUES (?, ?, ?, ?)",
		scriptName, completed, endOfBatch, lastGitID,
	)
	return err
}

// GetTrackingRecords returns all records from the tracking table
func (td *TestDatabase) GetTrackingRecords() ([]TrackingRecord, error) {
	rows, err := td.DB.Query(
		"SELECT sno, scriptName, completed, endofbatch, COALESCE(lastgitid, '') FROM sqlScriptExec ORDER BY sno ASC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []TrackingRecord
	for rows.Next() {
		var rec TrackingRecord
		if err := rows.Scan(&rec.SNO, &rec.ScriptName, &rec.Completed, &rec.EndOfBatch, &rec.LastGitID); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, nil
}

// TrackingRecord represents a row in the sqlScriptExec table
type TrackingRecord struct {
	SNO        int
	ScriptName string
	Completed  bool
	EndOfBatch bool
	LastGitID  string
}

// ColumnExists checks if a column exists in a table
func (td *TestDatabase) ColumnExists(tableName, columnName string) (bool, error) {
	var count int
	err := td.DB.QueryRow(
		"SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = ? AND table_name = ? AND column_name = ?",
		td.DBName, tableName, columnName,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// IndexExists checks if an index exists on a table
func (td *TestDatabase) IndexExists(tableName, indexName string) (bool, error) {
	var count int
	err := td.DB.QueryRow(
		"SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = ? AND table_name = ? AND index_name = ?",
		td.DBName, tableName, indexName,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ResetDatabase drops all tables except system tables
func (td *TestDatabase) ResetDatabase() error {
	// Get all tables
	rows, err := td.DB.Query(
		"SELECT table_name FROM information_schema.tables WHERE table_schema = ?",
		td.DBName,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return err
		}
		tables = append(tables, tableName)
	}

	// Disable foreign key checks and drop tables
	if _, err := td.DB.Exec("SET FOREIGN_KEY_CHECKS = 0"); err != nil {
		return err
	}

	for _, table := range tables {
		if _, err := td.DB.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", table)); err != nil {
			return err
		}
	}

	if _, err := td.DB.Exec("SET FOREIGN_KEY_CHECKS = 1"); err != nil {
		return err
	}

	return nil
}
