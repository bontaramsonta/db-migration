package migration

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/bontaramsonta/db-migration/internal/config"
	"github.com/bontaramsonta/db-migration/internal/console"
	"github.com/bontaramsonta/db-migration/internal/testhelpers"
)

// TestMigrator_FreshMigration tests migration on a fresh database with no prior executions
func TestMigrator_FreshMigration(t *testing.T) {
	// Skip if Docker is not available
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// 1. Setup MySQL container
	testDB := testhelpers.SetupTestDB(t)

	// 2. Setup git repository
	repo := testhelpers.SetupGitRepo(t)

	// 3. Create scripts directory
	scriptsDir := repo.CreateScriptsDir("Automated_Change_Scripts")

	// 4. Create SQL scripts and commit them
	scripts := testhelpers.StandardScripts()
	for filename, content := range scripts {
		repo.AddSQLScript(scriptsDir, filename, content)
	}
	commitHash := repo.CommitScripts("Add initial migration scripts")

	// 5. Create config and migrator
	cfg := &config.Config{
		Host:       testDB.Host,
		User:       testDB.User,
		Password:   testDB.Password,
		DBName:     testDB.DBName,
		Port:       mustParsePort(testDB.Port),
		ScriptsDir: scriptsDir,
	}

	cons := console.New(false)
	migrator := NewMigrator(cfg, testDB.DB, cons)

	// 6. Run migration
	err := migrator.Run()
	if err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// 7. Verify tracking table exists and has correct records
	records, err := testDB.GetTrackingRecords()
	if err != nil {
		t.Fatalf("failed to get tracking records: %v", err)
	}

	if len(records) != 3 {
		t.Errorf("expected 3 tracking records, got %d", len(records))
	}

	// Verify all scripts are marked as completed
	for _, rec := range records {
		if !rec.Completed {
			t.Errorf("script %s should be marked as completed", rec.ScriptName)
		}
	}

	// Verify last script has endofbatch = 1
	if len(records) > 0 {
		lastRecord := records[len(records)-1]
		if !lastRecord.EndOfBatch {
			t.Error("last script should have endofbatch = true")
		}
		if lastRecord.LastGitID != commitHash {
			t.Errorf("last script should have lastgitid = %s, got %s", commitHash, lastRecord.LastGitID)
		}
	}

	// 8. Verify database tables were created
	usersExists, err := testDB.TableExists("users")
	if err != nil {
		t.Fatalf("failed to check users table: %v", err)
	}
	if !usersExists {
		t.Error("users table should exist")
	}

	postsExists, err := testDB.TableExists("posts")
	if err != nil {
		t.Fatalf("failed to check posts table: %v", err)
	}
	if !postsExists {
		t.Error("posts table should exist")
	}

	// Verify indexes were created
	indexExists, err := testDB.IndexExists("posts", "idx_posts_user_id")
	if err != nil {
		t.Fatalf("failed to check index: %v", err)
	}
	if !indexExists {
		t.Error("idx_posts_user_id index should exist")
	}
}

// TestMigrator_IncrementalMigration tests migration with existing executed scripts
func TestMigrator_IncrementalMigration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// 1. Setup MySQL container
	testDB := testhelpers.SetupTestDB(t)

	// 2. Setup git repository
	repo := testhelpers.SetupGitRepo(t)
	scriptsDir := repo.CreateScriptsDir("Automated_Change_Scripts")

	// 3. Create initial scripts and commit (commit A)
	initialScripts := testhelpers.StandardScripts()
	for filename, content := range initialScripts {
		repo.AddSQLScript(scriptsDir, filename, content)
	}
	commitA := repo.CommitScripts("Initial migration scripts")

	// 4. Run initial migration
	cfg := &config.Config{
		Host:       testDB.Host,
		User:       testDB.User,
		Password:   testDB.Password,
		DBName:     testDB.DBName,
		Port:       mustParsePort(testDB.Port),
		ScriptsDir: scriptsDir,
	}
	cons := console.New(false)
	migrator := NewMigrator(cfg, testDB.DB, cons)

	if err := migrator.Run(); err != nil {
		t.Fatalf("initial migration failed: %v", err)
	}

	// Verify initial state
	records, _ := testDB.GetTrackingRecords()
	if len(records) != 3 {
		t.Fatalf("expected 3 initial records, got %d", len(records))
	}

	// 5. Add new scripts and commit (commit B)
	newScripts := testhelpers.IncrementalScripts()
	for filename, content := range newScripts {
		repo.AddSQLScript(scriptsDir, filename, content)
	}
	commitB := repo.CommitScripts("Add new migration scripts")

	// 6. Run incremental migration
	migrator2 := NewMigrator(cfg, testDB.DB, cons)
	if err := migrator2.Run(); err != nil {
		t.Fatalf("incremental migration failed: %v", err)
	}

	// 7. Verify tracking table has all records
	records, err := testDB.GetTrackingRecords()
	if err != nil {
		t.Fatalf("failed to get tracking records: %v", err)
	}

	if len(records) != 5 {
		t.Errorf("expected 5 tracking records, got %d", len(records))
	}

	// Verify all scripts are completed
	for _, rec := range records {
		if !rec.Completed {
			t.Errorf("script %s should be marked as completed", rec.ScriptName)
		}
	}

	// Verify last record has correct commit hash
	if len(records) > 0 {
		lastRecord := records[len(records)-1]
		if !lastRecord.EndOfBatch {
			t.Error("last script should have endofbatch = true")
		}
		if lastRecord.LastGitID != commitB {
			t.Errorf("last script should have lastgitid = %s (commit B), got %s", commitB, lastRecord.LastGitID)
		}
	}

	// First batch should still have commit A
	if len(records) >= 3 {
		thirdRecord := records[2]
		if thirdRecord.LastGitID != commitA {
			t.Errorf("third script should have lastgitid = %s (commit A), got %s", commitA, thirdRecord.LastGitID)
		}
	}

	// 8. Verify new tables were created
	commentsExists, err := testDB.TableExists("comments")
	if err != nil {
		t.Fatalf("failed to check comments table: %v", err)
	}
	if !commentsExists {
		t.Error("comments table should exist")
	}

	tagsExists, err := testDB.TableExists("tags")
	if err != nil {
		t.Fatalf("failed to check tags table: %v", err)
	}
	if !tagsExists {
		t.Error("tags table should exist")
	}
}

// TestMigrator_ScriptFailure tests rollback on script failure
func TestMigrator_ScriptFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// 1. Setup MySQL container
	testDB := testhelpers.SetupTestDB(t)

	// 2. Setup git repository
	repo := testhelpers.SetupGitRepo(t)
	scriptsDir := repo.CreateScriptsDir("Automated_Change_Scripts")

	// 3. Create scripts with one invalid script
	failingScripts := testhelpers.FailingScripts()
	for filename, content := range failingScripts {
		repo.AddSQLScript(scriptsDir, filename, content)
	}
	repo.CommitScripts("Add scripts with invalid SQL")

	// 4. Run migration - should fail
	cfg := &config.Config{
		Host:       testDB.Host,
		User:       testDB.User,
		Password:   testDB.Password,
		DBName:     testDB.DBName,
		Port:       mustParsePort(testDB.Port),
		ScriptsDir: scriptsDir,
	}
	cons := console.New(false)
	migrator := NewMigrator(cfg, testDB.DB, cons)

	err := migrator.Run()
	if err == nil {
		t.Fatal("migration should have failed due to invalid SQL")
	}

	// Verify error message mentions the failed script
	if !strings.Contains(err.Error(), "002_invalid.sql") {
		t.Errorf("error should mention failed script, got: %v", err)
	}

	// 5. Verify tracking table state
	records, err := testDB.GetTrackingRecords()
	if err != nil {
		t.Fatalf("failed to get tracking records: %v", err)
	}

	// Should have 2 records: one successful, one failed
	if len(records) != 2 {
		t.Errorf("expected 2 tracking records, got %d", len(records))
	}

	// First script should be completed
	if len(records) > 0 {
		if !records[0].Completed {
			t.Error("first script (001_create_users.sql) should be completed")
		}
	}

	// Second script should be marked as failed (completed=0)
	if len(records) > 1 {
		if records[1].Completed {
			t.Error("second script (002_invalid.sql) should NOT be completed")
		}
	}

	// 6. Verify database state
	// Users table should exist (from script 1)
	usersExists, err := testDB.TableExists("users")
	if err != nil {
		t.Fatalf("failed to check users table: %v", err)
	}
	if !usersExists {
		t.Error("users table should exist (from successful first script)")
	}

	// Posts table should NOT exist (script 3 was never executed)
	postsExists, err := testDB.TableExists("posts")
	if err != nil {
		t.Fatalf("failed to check posts table: %v", err)
	}
	if postsExists {
		t.Error("posts table should NOT exist (third script should not have run)")
	}
}

// TestMigrator_ModifiedScriptDetection tests that modified scripts are detected and rejected
func TestMigrator_ModifiedScriptDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// 1. Setup MySQL container
	testDB := testhelpers.SetupTestDB(t)

	// 2. Setup git repository
	repo := testhelpers.SetupGitRepo(t)
	scriptsDir := repo.CreateScriptsDir("Automated_Change_Scripts")

	// 3. Create initial script and commit
	repo.AddSQLScript(scriptsDir, "001_create_users.sql", testhelpers.SQLScripts.CreateUsers)
	repo.CommitScripts("Add initial script")

	// 4. Run initial migration
	cfg := &config.Config{
		Host:       testDB.Host,
		User:       testDB.User,
		Password:   testDB.Password,
		DBName:     testDB.DBName,
		Port:       mustParsePort(testDB.Port),
		ScriptsDir: scriptsDir,
	}
	cons := console.New(false)
	migrator := NewMigrator(cfg, testDB.DB, cons)

	if err := migrator.Run(); err != nil {
		t.Fatalf("initial migration failed: %v", err)
	}

	// Verify initial state
	records, _ := testDB.GetTrackingRecords()
	if len(records) != 1 {
		t.Fatalf("expected 1 initial record, got %d", len(records))
	}

	// 5. Modify the executed script
	modifiedContent := testhelpers.ModifiedCreateUsers()
	scriptPath := filepath.Join(scriptsDir, "001_create_users.sql")
	repo.ModifyFile(filepath.Join("Automated_Change_Scripts", "001_create_users.sql"), modifiedContent)
	repo.CommitChanges("Modify executed script")

	// Verify file was modified
	actualContent := repo.MustReadFile(scriptPath)
	if !strings.Contains(actualContent, "phone") {
		t.Fatal("script modification did not persist")
	}

	// 6. Try to run migration again - should fail
	migrator2 := NewMigrator(cfg, testDB.DB, cons)
	err := migrator2.Run()

	if err == nil {
		t.Fatal("migration should have failed due to modified script")
	}

	// Verify error message mentions modification
	if !strings.Contains(err.Error(), "modified") && !strings.Contains(err.Error(), "deleted") {
		t.Errorf("error should mention modified scripts, got: %v", err)
	}

	// 7. Verify tracking table is unchanged
	records, err = testDB.GetTrackingRecords()
	if err != nil {
		t.Fatalf("failed to get tracking records: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("expected 1 tracking record (unchanged), got %d", len(records))
	}

	// 8. Verify database state is unchanged
	// Users table should still exist without the phone column
	colExists, err := testDB.ColumnExists("users", "phone")
	if err != nil {
		t.Fatalf("failed to check column: %v", err)
	}
	if colExists {
		t.Error("phone column should NOT exist (migration should have been aborted)")
	}
}

// TestMigrator_NoNewScripts tests running migration when there are no new scripts
func TestMigrator_NoNewScripts(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// 1. Setup MySQL container
	testDB := testhelpers.SetupTestDB(t)

	// 2. Setup git repository
	repo := testhelpers.SetupGitRepo(t)
	scriptsDir := repo.CreateScriptsDir("Automated_Change_Scripts")

	// 3. Create scripts and commit
	scripts := testhelpers.StandardScripts()
	for filename, content := range scripts {
		repo.AddSQLScript(scriptsDir, filename, content)
	}
	repo.CommitScripts("Add migration scripts")

	// 4. Run initial migration
	cfg := &config.Config{
		Host:       testDB.Host,
		User:       testDB.User,
		Password:   testDB.Password,
		DBName:     testDB.DBName,
		Port:       mustParsePort(testDB.Port),
		ScriptsDir: scriptsDir,
	}
	cons := console.New(false)
	migrator := NewMigrator(cfg, testDB.DB, cons)

	if err := migrator.Run(); err != nil {
		t.Fatalf("initial migration failed: %v", err)
	}

	// 5. Run migration again - should succeed with no new scripts
	migrator2 := NewMigrator(cfg, testDB.DB, cons)
	if err := migrator2.Run(); err != nil {
		t.Fatalf("second migration should succeed even with no new scripts: %v", err)
	}

	// 6. Verify tracking table hasn't changed
	records, err := testDB.GetTrackingRecords()
	if err != nil {
		t.Fatalf("failed to get tracking records: %v", err)
	}

	if len(records) != 3 {
		t.Errorf("expected 3 tracking records (unchanged), got %d", len(records))
	}
}

// TestMigrator_EmptyRepository tests migration on an empty repository
func TestMigrator_EmptyRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// 1. Setup MySQL container
	testDB := testhelpers.SetupTestDB(t)

	// 2. Setup git repository with just an initial commit
	repo := testhelpers.SetupGitRepo(t)
	scriptsDir := repo.CreateScriptsDir("Automated_Change_Scripts")

	// Create an empty file to make the directory tracked
	repo.CreateCommit(map[string]string{
		"Automated_Change_Scripts/.gitkeep": "",
	}, "Initialize empty scripts directory")

	// 3. Run migration - should succeed with nothing to do
	cfg := &config.Config{
		Host:       testDB.Host,
		User:       testDB.User,
		Password:   testDB.Password,
		DBName:     testDB.DBName,
		Port:       mustParsePort(testDB.Port),
		ScriptsDir: scriptsDir,
	}
	cons := console.New(false)
	migrator := NewMigrator(cfg, testDB.DB, cons)

	if err := migrator.Run(); err != nil {
		t.Fatalf("migration should succeed on empty repo: %v", err)
	}

	// 4. Verify tracking table exists but is empty
	records, err := testDB.GetTrackingRecords()
	if err != nil {
		t.Fatalf("failed to get tracking records: %v", err)
	}

	if len(records) != 0 {
		t.Errorf("expected 0 tracking records, got %d", len(records))
	}
}

// mustParsePort converts port string to int
func mustParsePort(port string) int {
	var result int
	for _, c := range port {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}
	return result
}

