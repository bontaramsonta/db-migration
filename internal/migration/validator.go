package migration

import (
	"fmt"
	"path/filepath"

	"github.com/bontaramsonta/db-migration/internal/console"
	"github.com/bontaramsonta/db-migration/internal/git"
)

// Validator handles modification checks for scripts
type Validator struct {
	git     *git.Git
	console *console.Console
}

// NewValidator creates a new Validator instance
func NewValidator(g *git.Git, c *console.Console) *Validator {
	return &Validator{
		git:     g,
		console: c,
	}
}

// CheckFileModifications checks if any previously executed scripts have been modified or deleted
// Returns an error if modifications are detected (which should fail the migration)
func (v *Validator) CheckFileModifications(fromCommit, toCommit string, executedScripts map[string]bool) error {
	if fromCommit == "" {
		// No previous commit, nothing to check
		return nil
	}

	// Get all file changes between commits
	statusMap, err := v.git.DiffFileStatus(fromCommit, toCommit)
	if err != nil {
		return fmt.Errorf("failed to get file status: %w", err)
	}

	var modified []string
	var deleted []string

	for file, status := range statusMap {
		// Check if this file was previously executed
		// Compare both full path and base filename since tracking table may store either
		baseName := filepath.Base(file)
		if !executedScripts[file] && !executedScripts[baseName] {
			continue
		}

		switch status {
		case "M":
			modified = append(modified, file)
		case "D":
			deleted = append(deleted, file)
		}
	}

	if len(modified) > 0 {
		v.console.Error("The following previously executed scripts have been MODIFIED:")
		for _, f := range modified {
			v.console.Failure("  - %s", f)
		}
	}

	if len(deleted) > 0 {
		v.console.Error("The following previously executed scripts have been DELETED:")
		for _, f := range deleted {
			v.console.Failure("  - %s", f)
		}
	}

	if len(modified) > 0 || len(deleted) > 0 {
		return fmt.Errorf("detected %d modified and %d deleted scripts that were previously executed - migration aborted", len(modified), len(deleted))
	}

	return nil
}

// CheckHalfCommittedFiles validates partial deployment state
// If there are scripts executed after the last successful batch, they need special handling
func (v *Validator) CheckHalfCommittedFiles(halfCommitted []ScriptRecord) error {
	if len(halfCommitted) == 0 {
		return nil
	}

	v.console.Warn("Detected %d scripts from incomplete previous batch:", len(halfCommitted))
	for _, rec := range halfCommitted {
		status := "completed"
		if !rec.Completed {
			status = "FAILED"
		}
		v.console.Info("  - %s (%s)", rec.ScriptName, status)
	}

	// Check if any failed scripts exist
	for _, rec := range halfCommitted {
		if !rec.Completed {
			return fmt.Errorf("previous migration batch has failed script: %s - manual intervention required", rec.ScriptName)
		}
	}

	v.console.Info("All scripts from incomplete batch were successful, continuing...")
	return nil
}

// ValidateScriptsDirectory checks if the scripts directory is within a git repository
func (v *Validator) ValidateScriptsDirectory() error {
	if !v.git.IsGitRepository() {
		return fmt.Errorf("scripts directory is not within a git repository")
	}
	return nil
}

