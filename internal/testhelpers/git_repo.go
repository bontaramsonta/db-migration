package testhelpers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// GitRepo provides test helpers for creating and managing git repositories
type GitRepo struct {
	Dir string
	t   *testing.T
}

// SetupGitRepo creates a new git repository in a temporary directory
func SetupGitRepo(t *testing.T) *GitRepo {
	t.Helper()

	dir := t.TempDir()
	repo := &GitRepo{Dir: dir, t: t}

	// Initialize git repository
	repo.runGit("init")

	// Configure git user (required for commits)
	repo.runGit("config", "user.name", "Test User")
	repo.runGit("config", "user.email", "test@test.com")

	return repo
}

// runGit executes a git command in the repository directory
func (r *GitRepo) runGit(args ...string) string {
	r.t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v failed: %v\nOutput: %s", args, err, string(output))
	}
	return string(output)
}

// CreateCommit creates a new commit with the given files
// files is a map of relative filepath -> content
// Returns the commit hash
func (r *GitRepo) CreateCommit(files map[string]string, message string) string {
	r.t.Helper()

	for relPath, content := range files {
		fullPath := filepath.Join(r.Dir, relPath)

		// Ensure directory exists
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			r.t.Fatalf("failed to create directory %s: %v", dir, err)
		}

		// Write file
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			r.t.Fatalf("failed to write file %s: %v", fullPath, err)
		}

		// Stage file
		r.runGit("add", relPath)
	}

	// Create commit
	r.runGit("commit", "-m", message)

	// Get commit hash
	return r.GetCurrentCommit()
}

// ModifyFile modifies an existing file (stages but doesn't commit)
func (r *GitRepo) ModifyFile(relPath, content string) {
	r.t.Helper()

	fullPath := filepath.Join(r.Dir, relPath)
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		r.t.Fatalf("failed to modify file %s: %v", fullPath, err)
	}

	r.runGit("add", relPath)
}

// DeleteFile deletes a file and stages the deletion (doesn't commit)
func (r *GitRepo) DeleteFile(relPath string) {
	r.t.Helper()

	r.runGit("rm", relPath)
}

// CommitChanges commits all staged changes
func (r *GitRepo) CommitChanges(message string) string {
	r.t.Helper()

	r.runGit("commit", "-m", message)
	return r.GetCurrentCommit()
}

// GetCurrentCommit returns the current HEAD commit hash
func (r *GitRepo) GetCurrentCommit() string {
	r.t.Helper()

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = r.Dir
	output, err := cmd.Output()
	if err != nil {
		r.t.Fatalf("failed to get current commit: %v", err)
	}
	// Trim newline
	hash := string(output)
	if len(hash) > 0 && hash[len(hash)-1] == '\n' {
		hash = hash[:len(hash)-1]
	}
	return hash
}

// CreateScriptsDir creates a subdirectory for SQL scripts and returns the path
func (r *GitRepo) CreateScriptsDir(dirName string) string {
	r.t.Helper()

	scriptsDir := filepath.Join(r.Dir, dirName)
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		r.t.Fatalf("failed to create scripts directory: %v", err)
	}
	return scriptsDir
}

// AddSQLScript adds a SQL script file to the scripts directory
// Returns the relative path to the script
func (r *GitRepo) AddSQLScript(scriptsDir, filename, content string) string {
	r.t.Helper()

	// Calculate relative path from repo root
	relScriptsDir, err := filepath.Rel(r.Dir, scriptsDir)
	if err != nil {
		r.t.Fatalf("failed to get relative path: %v", err)
	}

	relPath := filepath.Join(relScriptsDir, filename)
	fullPath := filepath.Join(r.Dir, relPath)

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		r.t.Fatalf("failed to write script %s: %v", fullPath, err)
	}

	r.runGit("add", relPath)
	return relPath
}

// CommitScripts commits all staged script files
func (r *GitRepo) CommitScripts(message string) string {
	r.t.Helper()

	return r.CommitChanges(message)
}

// GetScriptPath returns the full path to a script file
func (r *GitRepo) GetScriptPath(scriptsDir, filename string) string {
	return filepath.Join(scriptsDir, filename)
}

// MustReadFile reads a file and fails the test if it can't be read
func (r *GitRepo) MustReadFile(path string) string {
	r.t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		r.t.Fatalf("failed to read file %s: %v", path, err)
	}
	return string(content)
}

// String returns a string representation of the repo for debugging
func (r *GitRepo) String() string {
	return fmt.Sprintf("GitRepo{Dir: %s}", r.Dir)
}

