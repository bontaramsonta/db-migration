package testhelpers

import (
	"os"
	"path/filepath"
	"testing"
)

// SQLScripts provides common SQL script templates for testing
var SQLScripts = struct {
	CreateUsers    string
	CreatePosts    string
	AddIndexes     string
	CreateComments string
	CreateTags     string
	InvalidSyntax  string
}{
	CreateUsers: `CREATE TABLE users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(255) UNIQUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);`,

	CreatePosts: `CREATE TABLE posts (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT NOT NULL,
    title VARCHAR(200) NOT NULL,
    body TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);`,

	AddIndexes: `CREATE INDEX idx_posts_user_id ON posts(user_id);
CREATE INDEX idx_users_email ON users(email);`,

	CreateComments: `CREATE TABLE comments (
    id INT AUTO_INCREMENT PRIMARY KEY,
    post_id INT NOT NULL,
    user_id INT NOT NULL,
    content TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (post_id) REFERENCES posts(id),
    FOREIGN KEY (user_id) REFERENCES users(id)
);`,

	CreateTags: `CREATE TABLE tags (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(50) UNIQUE NOT NULL
);

CREATE TABLE post_tags (
    post_id INT NOT NULL,
    tag_id INT NOT NULL,
    PRIMARY KEY (post_id, tag_id),
    FOREIGN KEY (post_id) REFERENCES posts(id),
    FOREIGN KEY (tag_id) REFERENCES tags(id)
);`,

	// Script with invalid SQL syntax for failure testing
	InvalidSyntax: `CREATE TABLE invalid_table;`,
}

// CreateSQLScript writes a SQL script file to the specified directory
func CreateSQLScript(t *testing.T, dir, filename, content string) string {
	t.Helper()

	fullPath := filepath.Join(dir, filename)

	// Ensure directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create directory %s: %v", dir, err)
	}

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write script %s: %v", fullPath, err)
	}

	return fullPath
}

// CreateTestScripts writes multiple SQL scripts to a directory
// scripts is a map of filename -> content
func CreateTestScripts(t *testing.T, dir string, scripts map[string]string) {
	t.Helper()

	for filename, content := range scripts {
		CreateSQLScript(t, dir, filename, content)
	}
}

// StandardScripts returns a map of standard test scripts with numbered prefixes
func StandardScripts() map[string]string {
	return map[string]string{
		"001_create_users.sql": SQLScripts.CreateUsers,
		"002_create_posts.sql": SQLScripts.CreatePosts,
		"003_add_indexes.sql":  SQLScripts.AddIndexes,
	}
}

// IncrementalScripts returns additional scripts for incremental testing
func IncrementalScripts() map[string]string {
	return map[string]string{
		"004_create_comments.sql": SQLScripts.CreateComments,
		"005_create_tags.sql":     SQLScripts.CreateTags,
	}
}

// FailingScripts returns scripts where the second one has invalid syntax
func FailingScripts() map[string]string {
	return map[string]string{
		"001_create_users.sql": SQLScripts.CreateUsers,
		"002_invalid.sql":      SQLScripts.InvalidSyntax,
		"003_create_posts.sql": SQLScripts.CreatePosts,
	}
}

// ModifiedCreateUsers returns a modified version of the create users script
func ModifiedCreateUsers() string {
	return `CREATE TABLE users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(255) UNIQUE,
    phone VARCHAR(20),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);`
}

// SimpleCreateTable returns a simple create table statement for quick tests
func SimpleCreateTable(tableName string) string {
	return `CREATE TABLE ` + tableName + ` (
    id INT AUTO_INCREMENT PRIMARY KEY,
    data VARCHAR(255)
);`
}

// SimpleInsert returns a simple insert statement
func SimpleInsert(tableName, value string) string {
	return `INSERT INTO ` + tableName + ` (data) VALUES ('` + value + `');`
}

