package migration

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bontaramsonta/db-migration/internal/config"
	"github.com/bontaramsonta/db-migration/internal/console"
	"github.com/bontaramsonta/db-migration/internal/db"
	"github.com/bontaramsonta/db-migration/internal/git"
)

// Migrator orchestrates the migration process
type Migrator struct {
	config    *config.Config
	db        *db.DB
	git       *git.Git
	tracker   *Tracker
	validator *Validator
	console   *console.Console
}

// NewMigrator creates a new Migrator instance
func NewMigrator(cfg *config.Config, database *db.DB, console *console.Console) *Migrator {
	gitInstance := git.New(cfg.ScriptsDir)
	tracker := NewTracker(database)
	validator := NewValidator(gitInstance, console)

	return &Migrator{
		config:    cfg,
		db:        database,
		git:       gitInstance,
		tracker:   tracker,
		validator: validator,
		console:   console,
	}
}

// Run executes the migration process
func (m *Migrator) Run() error {
	m.console.Header("DB Migration Started")

	// 1. Validate git repository
	m.console.Info("Validating scripts directory...")
	if err := m.validator.ValidateScriptsDirectory(); err != nil {
		return err
	}

	// 2. Ensure tracking table exists
	m.console.Info("Ensuring tracking table exists...")
	if err := m.tracker.EnsureTable(); err != nil {
		return err
	}

	// 3. Get last successful git commit from DB
	lastGitID, err := m.tracker.GetLastSuccessfulCommit()
	if err != nil {
		return fmt.Errorf("failed to get last successful commit: %w", err)
	}

	if lastGitID == "" {
		m.console.Info("No previous migration found - this is a fresh migration")
	} else {
		m.console.Info("Last successful migration at commit: %s", lastGitID[:8])
	}

	// 4. Execute missed scripts if file provided
	if m.config.MissedScriptsFile != "" {
		if err := m.executeMissedScripts(); err != nil {
			return err
		}
	}

	// 5. Get executed scripts for modification check
	executedScripts, err := m.tracker.GetExecutedScriptNames()
	if err != nil {
		return fmt.Errorf("failed to get executed scripts: %w", err)
	}

	// 6. Get current commit
	currentCommit, err := m.git.GetCurrentCommit()
	if err != nil {
		return fmt.Errorf("failed to get current commit: %w", err)
	}
	m.console.Info("Current commit: %s", currentCommit[:8])

	// 7. Check file modifications (fail if executed scripts were modified/deleted)
	m.console.Info("Checking for modifications to executed scripts...")
	if err := m.validator.CheckFileModifications(lastGitID, currentCommit, executedScripts); err != nil {
		return err
	}

	// 8. Check half-committed files
	halfCommitted, err := m.tracker.GetHalfCommittedScripts()
	if err != nil {
		return fmt.Errorf("failed to get half-committed scripts: %w", err)
	}
	if err := m.validator.CheckHalfCommittedFiles(halfCommitted); err != nil {
		return err
	}

	// 9. Get changed files from git, sorted by commit time
	m.console.Info("Discovering new scripts...")
	scripts, err := m.git.GetChangedScripts(lastGitID, currentCommit, m.config.ScriptsDir)
	if err != nil {
		return fmt.Errorf("failed to get changed scripts: %w", err)
	}

	// 10. Filter out already-executed scripts
	var pendingScripts []git.ScriptInfo
	for _, script := range scripts {
		if !executedScripts[script.Name] {
			pendingScripts = append(pendingScripts, script)
		}
	}

	if len(pendingScripts) == 0 {
		m.console.Success("No new scripts to execute")
		return nil
	}

	m.console.Info("Found %d new scripts to execute", len(pendingScripts))

	// 11. Execute each script in its own transaction
	successCount := 0
	failedCount := 0
	skippedCount := len(scripts) - len(pendingScripts)

	for i, script := range pendingScripts {
		isLast := i == len(pendingScripts)-1

		m.console.Script(script.Name, "executing")

		if err := m.executeScript(script, currentCommit, isLast); err != nil {
			m.console.Script(script.Name, "failed")
			m.console.Error("Script execution failed: %v", err)
			failedCount++

			// Report summary and exit
			m.console.Summary(len(scripts), successCount, failedCount, skippedCount)
			return fmt.Errorf("migration failed at script: %s", script.Name)
		}

		m.console.Script(script.Name, "success")
		successCount++
	}

	// 12. Report final status
	m.console.Summary(len(scripts), successCount, failedCount, skippedCount)
	m.console.Success("Migration completed successfully!")

	return nil
}

// executeScript runs a single script within a transaction
func (m *Migrator) executeScript(script git.ScriptInfo, gitID string, isLast bool) error {
	// Read script content
	scriptPath := filepath.Join(m.config.ScriptsDir, script.Name)
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		// Try the full path from git
		content, err = os.ReadFile(script.Path)
		if err != nil {
			return fmt.Errorf("failed to read script %s: %w", script.Name, err)
		}
	}

	// Start transaction
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute script
	if err := db.ExecuteSQL(tx, string(content)); err != nil {
		// Record failure (in a new transaction since this one is tainted)
		m.tracker.RecordExecutionDirect(script.Name, false, false, gitID)
		return fmt.Errorf("script execution error: %w", err)
	}

	// Record success
	if err := m.tracker.RecordExecution(tx, script.Name, true, isLast, gitID); err != nil {
		return fmt.Errorf("failed to record execution: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// executeMissedScripts processes scripts from the missed scripts file
func (m *Migrator) executeMissedScripts() error {
	m.console.Header("Processing Missed Scripts")

	file, err := os.Open(m.config.MissedScriptsFile)
	if err != nil {
		return fmt.Errorf("failed to open missed scripts file: %w", err)
	}
	defer file.Close()

	var missedScripts []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			missedScripts = append(missedScripts, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading missed scripts file: %w", err)
	}

	if len(missedScripts) == 0 {
		m.console.Info("No missed scripts to process")
		return nil
	}

	m.console.Info("Found %d missed scripts to process", len(missedScripts))

	// Get current commit for tracking
	currentCommit, err := m.git.GetCurrentCommit()
	if err != nil {
		return fmt.Errorf("failed to get current commit: %w", err)
	}

	// Get already executed scripts
	executedScripts, err := m.tracker.GetExecutedScriptNames()
	if err != nil {
		return fmt.Errorf("failed to get executed scripts: %w", err)
	}

	for i, scriptName := range missedScripts {
		// Skip if already executed
		if executedScripts[scriptName] {
			m.console.Script(scriptName, "skipped")
			continue
		}

		isLast := i == len(missedScripts)-1
		script := git.ScriptInfo{
			Name: scriptName,
			Path: filepath.Join(m.config.ScriptsDir, scriptName),
		}

		m.console.Script(scriptName, "executing")

		if err := m.executeScript(script, currentCommit, isLast); err != nil {
			m.console.Script(scriptName, "failed")
			return fmt.Errorf("failed to execute missed script %s: %w", scriptName, err)
		}

		m.console.Script(scriptName, "success")
	}

	m.console.Success("All missed scripts processed successfully")
	return nil
}

// ExecuteSingleScript executes a single script by name (for testing purposes)
func (m *Migrator) ExecuteSingleScript(scriptName string) error {
	currentCommit, err := m.git.GetCurrentCommit()
	if err != nil {
		currentCommit = "manual"
	}

	script := git.ScriptInfo{
		Name: scriptName,
		Path: filepath.Join(m.config.ScriptsDir, scriptName),
	}

	return m.executeScript(script, currentCommit, true)
}

// getTransaction is a helper to get a transaction from the tracker's db
// This is needed because RecordExecution expects a *sql.Tx
func (m *Migrator) beginTrackerTransaction() (*sql.Tx, error) {
	return m.db.Begin()
}
