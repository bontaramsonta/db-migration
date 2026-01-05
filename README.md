# DB Migration CLI

A Go-based database migration tool that tracks and executes SQL scripts with git integration, transaction support, and savepoint-based rollback.

## Features

- **Git Integration**: Automatically detects new SQL scripts based on git history
- **Transaction Support**: Each script runs in its own transaction with savepoints
- **Modification Detection**: Fails if previously executed scripts have been modified or deleted
- **Tracking Table**: Maintains execution history in `sqlScriptExec` table
- **Colored Output**: Clear, timestamped console output with status indicators
- **Missed Scripts**: Support for executing scripts that were missed in previous runs

## Installation

```bash
cd db-migration
go build -o db-migration ./cmd/db-migration
```

## Usage

```bash
db-migration <host> <user> <password> <dbname> <port> <scripts_dir> [missed_scripts_file]
```

### Arguments

| Argument | Description |
|----------|-------------|
| `host` | MySQL host address |
| `user` | MySQL username |
| `password` | MySQL password |
| `dbname` | Database name |
| `port` | MySQL port number |
| `scripts_dir` | Directory containing SQL migration scripts |
| `missed_scripts_file` | (Optional) File containing list of missed scripts to execute |

### Examples

```bash
# Basic migration
db-migration localhost root password mydb 3306 ./migrations

# With missed scripts file
db-migration localhost root password mydb 3306 ./migrations missed.txt
```

## How It Works

### Migration Flow

1. **Validate Environment**: Ensures scripts directory is a git repository
2. **Ensure Tracking Table**: Creates `sqlScriptExec` table if it doesn't exist
3. **Get Last Migration State**: Retrieves the git commit of the last successful batch
4. **Process Missed Scripts**: If a missed scripts file is provided, executes those first
5. **Check Modifications**: Fails if previously executed scripts have been modified or deleted
6. **Check Incomplete Batches**: Validates any scripts from incomplete previous runs
7. **Discover New Scripts**: Finds SQL files changed since last migration, sorted by commit time
8. **Execute Scripts**: Runs each script in a transaction with savepoints
9. **Report Summary**: Shows final execution statistics

### Transaction Strategy

Each script execution is wrapped in its own transaction:

```
BEGIN TRANSACTION
  ├── Execute SQL content
  ├── If success: record success, COMMIT
  └── If failure: ROLLBACK, record failure, exit
```

### Tracking Table Schema

```sql
CREATE TABLE sqlScriptExec (
    sno INT(11) PRIMARY KEY AUTO_INCREMENT,
    scriptName VARCHAR(500) NOT NULL,
    completed BOOLEAN,
    endofbatch BOOLEAN,
    lastgitid VARCHAR(70),
    createddatetime DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modifieddatetime DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
```

### Missed Scripts File Format

Plain text file with one script name per line. Comments (lines starting with `#`) are ignored:

```
# Scripts that were missed
create_users_table.sql
add_indexes.sql
```

## Safety Features

1. **Modification Detection**: The tool will fail if any previously executed script has been modified or deleted
2. **Half-Committed Detection**: Detects and reports scripts from incomplete previous migrations
3. **Savepoint Rollback**: Failed scripts are rolled back to their savepoint, preserving successful scripts
4. **Execution Recording**: All executions (success or failure) are recorded in the tracking table

## Project Structure

```
db-migration/
├── cmd/
│   └── db-migration/
│       └── main.go           # Entry point, flag parsing
├── internal/
│   ├── config/
│   │   └── config.go         # Configuration struct
│   ├── db/
│   │   └── db.go             # database/sql wrapper with transactions
│   ├── git/
│   │   └── git.go            # Git CLI wrapper
│   ├── migration/
│   │   ├── migrator.go       # Main orchestration
│   │   ├── tracker.go        # Tracking table operations
│   │   └── validator.go      # Modification checks
│   └── console/
│       └── output.go         # Colored output with logging
├── docker-compose.yml        # MySQL for testing
├── go.mod
└── README.md
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success - all scripts executed successfully |
| 1 | Failure - migration failed (check output for details) |

## Dependencies

- `github.com/go-sql-driver/mysql` - MySQL driver for Go
- Git CLI (must be available in PATH)

## Testing

The project includes comprehensive end-to-end integration tests using real git repositories and a Docker MySQL container.

### Prerequisites

- **Docker**: Required for running the MySQL test container
- **Docker Compose**: For managing the test MySQL instance
- **Git**: Required for test repository operations

### Running Tests

```bash
# Start MySQL container
docker compose up -d

# Run tests (will wait for MySQL to be healthy automatically)
go test -v ./internal/migration/...

# Stop MySQL when done
docker compose down

# Clean up volumes (reset all data)
docker compose down -v
```

The test setup automatically waits for MySQL to become healthy with retries, so you can run `go test` immediately after starting the container.

### Test Database Configuration

The test database uses the following defaults (configurable via environment variables):

| Variable | Default | Description |
|----------|---------|-------------|
| `TEST_DB_HOST` | `127.0.0.1` | MySQL host |
| `TEST_DB_PORT` | `3307` | MySQL port |
| `TEST_DB_USER` | `testuser` | MySQL user |
| `TEST_DB_PASSWORD` | `testpassword` | MySQL password |
| `TEST_DB_NAME` | `testdb` | Database name |

### Test Scenarios

| Test | Description |
|------|-------------|
| `TestMigrator_FreshMigration` | Migration on a fresh database with no prior executions |
| `TestMigrator_IncrementalMigration` | Adding new scripts to an existing migration history |
| `TestMigrator_ScriptFailure` | Verifies rollback behavior when a script fails |
| `TestMigrator_ModifiedScriptDetection` | Detects and rejects modified previously-executed scripts |
| `TestMigrator_NoNewScripts` | Re-running migration when there are no new scripts |
| `TestMigrator_EmptyRepository` | Handles empty scripts directory gracefully |

### Test Infrastructure

```
db-migration/
├── docker-compose.yml        # MySQL 8.0 with healthcheck
└── internal/
    ├── testhelpers/
    │   ├── git_repo.go       # Git repository test helpers
    │   ├── database.go       # MySQL connection with retry and reset
    │   └── scripts.go        # SQL script templates
    └── migration/
        └── migrator_test.go  # Integration tests
```

#### Test Helpers

- **`testhelpers.SetupGitRepo(t)`**: Creates a temporary git repository with user config
- **`testhelpers.SetupTestDB(t)`**: Connects to MySQL (with retries) and resets the database to a clean state
- **`testhelpers.StandardScripts()`**: Returns common test SQL scripts (users, posts, indexes)

### Running a Specific Test

```bash
# Start MySQL first
docker compose up -d

# Run a specific test
go test -v -run TestMigrator_FreshMigration ./internal/migration/...

# Stop MySQL when done
docker compose down
```
