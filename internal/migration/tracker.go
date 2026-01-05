package migration

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/bontaramsonta/db-migration/internal/db"
)

// Tracker handles tracking table operations
type Tracker struct {
	db        *db.DB
	tableName string
}

// ScriptRecord represents a record in the tracking table
type ScriptRecord struct {
	SNO              int
	ScriptName       string
	Completed        bool
	EndOfBatch       bool
	LastGitID        string
	CreatedDateTime  time.Time
	ModifiedDateTime time.Time
}

// NewTracker creates a new Tracker instance
func NewTracker(database *db.DB) *Tracker {
	return &Tracker{
		db:        database,
		tableName: "sqlScriptExec",
	}
}

// EnsureTable creates the tracking table if it doesn't exist
func (t *Tracker) EnsureTable() error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			sno INT(11) PRIMARY KEY AUTO_INCREMENT,
			scriptName VARCHAR(500) NOT NULL,
			completed BOOLEAN,
			endofbatch BOOLEAN,
			lastgitid VARCHAR(70),
			createddatetime DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			modifieddatetime DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		)
	`, t.tableName)

	_, err := t.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create tracking table: %w", err)
	}

	return nil
}

// GetLastSuccessfulCommit returns the git commit ID of the last successful batch
// (where endofbatch = 1)
func (t *Tracker) GetLastSuccessfulCommit() (string, error) {
	query := fmt.Sprintf(`
		SELECT lastgitid FROM %s 
		WHERE endofbatch = 1 
		ORDER BY sno DESC 
		LIMIT 1
	`, t.tableName)

	var lastGitID sql.NullString
	err := t.db.QueryRow(query).Scan(&lastGitID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get last successful commit: %w", err)
	}

	if !lastGitID.Valid {
		return "", nil
	}

	return lastGitID.String, nil
}

// GetExecutedScriptNames returns all script names that have been executed
func (t *Tracker) GetExecutedScriptNames() (map[string]bool, error) {
	query := fmt.Sprintf(`
		SELECT scriptName FROM %s WHERE completed = 1
	`, t.tableName)

	rows, err := t.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get executed scripts: %w", err)
	}
	defer rows.Close()

	executed := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to scan script name: %w", err)
		}
		executed[name] = true
	}

	return executed, nil
}

// RecordExecution inserts a record for script execution
func (t *Tracker) RecordExecution(tx *sql.Tx, scriptName string, completed bool, endOfBatch bool, gitID string) error {
	query := fmt.Sprintf(`
		INSERT INTO %s (scriptName, completed, endofbatch, lastgitid)
		VALUES (?, ?, ?, ?)
	`, t.tableName)

	_, err := tx.Exec(query, scriptName, completed, endOfBatch, gitID)
	if err != nil {
		return fmt.Errorf("failed to record execution for %s: %w", scriptName, err)
	}

	return nil
}

// RecordExecutionDirect inserts a record for script execution directly (no transaction)
func (t *Tracker) RecordExecutionDirect(scriptName string, completed bool, endOfBatch bool, gitID string) error {
	query := fmt.Sprintf(`
		INSERT INTO %s (scriptName, completed, endofbatch, lastgitid)
		VALUES (?, ?, ?, ?)
	`, t.tableName)

	_, err := t.db.Exec(query, scriptName, completed, endOfBatch, gitID)
	if err != nil {
		return fmt.Errorf("failed to record execution for %s: %w", scriptName, err)
	}

	return nil
}

// GetHalfCommittedScripts returns scripts executed after the last successful batch
// These are scripts that were started but the batch didn't complete
func (t *Tracker) GetHalfCommittedScripts() ([]ScriptRecord, error) {
	// Find the SNO of the last successful batch
	lastBatchQuery := fmt.Sprintf(`
		SELECT sno FROM %s 
		WHERE endofbatch = 1 
		ORDER BY sno DESC 
		LIMIT 1
	`, t.tableName)

	var lastBatchSNO int
	err := t.db.QueryRow(lastBatchQuery).Scan(&lastBatchSNO)
	if err == sql.ErrNoRows {
		// No successful batch found, check if there are any records at all
		lastBatchSNO = 0
	} else if err != nil {
		return nil, fmt.Errorf("failed to get last batch SNO: %w", err)
	}

	// Get all scripts after the last successful batch
	query := fmt.Sprintf(`
		SELECT sno, scriptName, completed, endofbatch, COALESCE(lastgitid, ''), createddatetime, modifieddatetime
		FROM %s 
		WHERE sno > ?
		ORDER BY sno ASC
	`, t.tableName)

	rows, err := t.db.Query(query, lastBatchSNO)
	if err != nil {
		return nil, fmt.Errorf("failed to get half-committed scripts: %w", err)
	}
	defer rows.Close()

	var scripts []ScriptRecord
	for rows.Next() {
		var rec ScriptRecord
		if err := rows.Scan(&rec.SNO, &rec.ScriptName, &rec.Completed, &rec.EndOfBatch, &rec.LastGitID, &rec.CreatedDateTime, &rec.ModifiedDateTime); err != nil {
			return nil, fmt.Errorf("failed to scan script record: %w", err)
		}
		scripts = append(scripts, rec)
	}

	return scripts, nil
}

// HasRecords checks if the tracking table has any records
func (t *Tracker) HasRecords() (bool, error) {
	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s`, t.tableName)

	var count int
	err := t.db.QueryRow(query).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to count records: %w", err)
	}

	return count > 0, nil
}

// GetAllScripts returns all script records
func (t *Tracker) GetAllScripts() ([]ScriptRecord, error) {
	query := fmt.Sprintf(`
		SELECT sno, scriptName, completed, endofbatch, COALESCE(lastgitid, ''), createddatetime, modifieddatetime
		FROM %s 
		ORDER BY sno ASC
	`, t.tableName)

	rows, err := t.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all scripts: %w", err)
	}
	defer rows.Close()

	var scripts []ScriptRecord
	for rows.Next() {
		var rec ScriptRecord
		if err := rows.Scan(&rec.SNO, &rec.ScriptName, &rec.Completed, &rec.EndOfBatch, &rec.LastGitID, &rec.CreatedDateTime, &rec.ModifiedDateTime); err != nil {
			return nil, fmt.Errorf("failed to scan script record: %w", err)
		}
		scripts = append(scripts, rec)
	}

	return scripts, nil
}

