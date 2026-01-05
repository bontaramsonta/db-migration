package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Git provides Git CLI operations
type Git struct {
	workDir string
}

// New creates a new Git instance for the given working directory
func New(workDir string) *Git {
	return &Git{workDir: workDir}
}

// run executes a git command and returns the output
func (g *Git) run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.workDir
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s failed: %s", strings.Join(args, " "), string(exitErr.Stderr))
		}
		return "", fmt.Errorf("git %s failed: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(output)), nil
}

// GetCurrentCommit returns the current HEAD commit hash
func (g *Git) GetCurrentCommit() (string, error) {
	return g.run("rev-parse", "HEAD")
}

// GetEmptyTreeHash returns the hash of an empty tree (for initial comparison)
func (g *Git) GetEmptyTreeHash() (string, error) {
	return g.run("hash-object", "-t", "tree", "/dev/null")
}

// DiffFileNames returns files changed between two commits
// If fromCommit is empty, uses empty tree hash (for initial migration)
func (g *Git) DiffFileNames(fromCommit, toCommit string) ([]string, error) {
	var output string
	var err error

	if fromCommit == "" {
		// Get empty tree hash for initial comparison
		emptyTree, err := g.GetEmptyTreeHash()
		if err != nil {
			return nil, err
		}
		fromCommit = emptyTree
	}

	output, err = g.run("diff", "--name-only", fromCommit, toCommit)
	if err != nil {
		return nil, err
	}

	if output == "" {
		return []string{}, nil
	}

	files := strings.Split(output, "\n")
	return files, nil
}

// DiffFileStatus returns files with their status (A/M/D) between two commits
func (g *Git) DiffFileStatus(fromCommit, toCommit string) (map[string]string, error) {
	if fromCommit == "" {
		emptyTree, err := g.GetEmptyTreeHash()
		if err != nil {
			return nil, err
		}
		fromCommit = emptyTree
	}

	output, err := g.run("diff", "--name-status", fromCommit, toCommit)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	if output == "" {
		return result, nil
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			status := parts[0]
			filename := parts[1]
			result[filename] = status
		}
	}

	return result, nil
}

// ScriptInfo holds information about a script file
type ScriptInfo struct {
	Name      string
	Path      string
	Timestamp time.Time
}

// GetFileCommitTimestamp returns the commit timestamp for a file
func (g *Git) GetFileCommitTimestamp(filepath string) (time.Time, error) {
	// Get the first commit that added this file (--follow --diff-filter=A)
	output, err := g.run("log", "--follow", "--diff-filter=A", "--format=%ct", "-1", "--", filepath)
	if err != nil {
		// If file was never committed, use current time
		return time.Now(), nil
	}

	if output == "" {
		return time.Now(), nil
	}

	timestamp, err := strconv.ParseInt(output, 10, 64)
	if err != nil {
		return time.Now(), nil
	}

	return time.Unix(timestamp, 0), nil
}

// GetChangedScripts returns SQL scripts changed between commits, sorted by commit timestamp
func (g *Git) GetChangedScripts(fromCommit, toCommit, scriptsDir string) ([]ScriptInfo, error) {
	files, err := g.DiffFileNames(fromCommit, toCommit)
	if err != nil {
		return nil, err
	}

	var scripts []ScriptInfo

	for _, file := range files {
		// Only include SQL files from the scripts directory
		if !strings.HasSuffix(file, ".sql") {
			continue
		}

		// Check if file is in the scripts directory
		relDir := filepath.Dir(file)
		scriptsBase := filepath.Base(scriptsDir)
		if !strings.Contains(relDir, scriptsBase) && relDir != scriptsBase && file[:len(scriptsBase)] != scriptsBase {
			// More permissive check - just verify it ends with .sql
			if !strings.HasSuffix(file, ".sql") {
				continue
			}
		}

		timestamp, err := g.GetFileCommitTimestamp(file)
		if err != nil {
			timestamp = time.Now()
		}

		scripts = append(scripts, ScriptInfo{
			Name:      filepath.Base(file),
			Path:      file,
			Timestamp: timestamp,
		})
	}

	// Sort by commit timestamp (oldest first)
	sort.Slice(scripts, func(i, j int) bool {
		return scripts[i].Timestamp.Before(scripts[j].Timestamp)
	})

	return scripts, nil
}

// CheckModifications detects M (modified) or D (deleted) changes for given files
func (g *Git) CheckModifications(fromCommit, toCommit string, files []string) (modified, deleted []string, err error) {
	if fromCommit == "" {
		// No previous commit, so no modifications possible
		return nil, nil, nil
	}

	statusMap, err := g.DiffFileStatus(fromCommit, toCommit)
	if err != nil {
		return nil, nil, err
	}

	fileSet := make(map[string]bool)
	for _, f := range files {
		fileSet[f] = true
	}

	for file, status := range statusMap {
		if !fileSet[file] {
			continue
		}

		switch status {
		case "M":
			modified = append(modified, file)
		case "D":
			deleted = append(deleted, file)
		}
	}

	return modified, deleted, nil
}

// IsGitRepository checks if the working directory is a git repository
func (g *Git) IsGitRepository() bool {
	_, err := g.run("rev-parse", "--git-dir")
	return err == nil
}

