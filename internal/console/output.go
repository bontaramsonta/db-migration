package console

import (
	"fmt"
	"os"
	"time"
)

// ANSI color codes
const (
	Reset   = "\033[0m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"
	Bold    = "\033[1m"
)

// Console provides colored output with logging
type Console struct {
	verbose bool
}

// New creates a new Console instance
func New(verbose bool) *Console {
	return &Console{verbose: verbose}
}

// timestamp returns current timestamp string
func timestamp() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// Success prints a success message in green
func (c *Console) Success(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s[%s]%s %s✓%s %s\n", Cyan, timestamp(), Reset, Green, Reset, msg)
}

// Failure prints a failure message in red
func (c *Console) Failure(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s[%s]%s %s✗%s %s\n", Cyan, timestamp(), Reset, Red, Reset, msg)
}

// Info prints an info message in blue
func (c *Console) Info(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s[%s]%s %sℹ%s %s\n", Cyan, timestamp(), Reset, Blue, Reset, msg)
}

// Warn prints a warning message in yellow
func (c *Console) Warn(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s[%s]%s %s⚠%s %s\n", Cyan, timestamp(), Reset, Yellow, Reset, msg)
}

// Error prints an error message in red and bold
func (c *Console) Error(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s[%s]%s %s%s✗ ERROR:%s %s\n", Cyan, timestamp(), Reset, Bold, Red, Reset, msg)
}

// Header prints a section header
func (c *Console) Header(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("\n%s%s═══ %s ═══%s\n\n", Bold, Cyan, msg, Reset)
}

// Script prints script execution info
func (c *Console) Script(name string, status string) {
	var statusColor string
	var symbol string

	switch status {
	case "executing":
		statusColor = Yellow
		symbol = "▶"
	case "success":
		statusColor = Green
		symbol = "✓"
	case "failed":
		statusColor = Red
		symbol = "✗"
	case "skipped":
		statusColor = Blue
		symbol = "○"
	default:
		statusColor = White
		symbol = "•"
	}

	fmt.Printf("%s[%s]%s %s%s%s %s\n", Cyan, timestamp(), Reset, statusColor, symbol, Reset, name)
}

// Summary prints final execution summary
func (c *Console) Summary(total, success, failed, skipped int) {
	c.Header("Migration Summary")
	fmt.Printf("  Total scripts:   %s%d%s\n", Bold, total, Reset)
	fmt.Printf("  Successful:      %s%s%d%s\n", Green, Bold, success, Reset)
	if failed > 0 {
		fmt.Printf("  Failed:          %s%s%d%s\n", Red, Bold, failed, Reset)
	} else {
		fmt.Printf("  Failed:          %d\n", failed)
	}
	fmt.Printf("  Skipped:         %s%d%s\n", Blue, skipped, Reset)
	fmt.Println()
}
