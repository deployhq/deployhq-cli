package output

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Logger provides structured logging to a debug file.
// The debug log is always written silently; its path is shown on error.
type Logger struct {
	file *os.File
	Path string
}

// NewLogger creates a debug log file under ~/.deployhq/logs/.
// Returns a no-op logger if the log directory can't be created.
func NewLogger() *Logger {
	home, err := os.UserHomeDir()
	if err != nil {
		return &Logger{}
	}

	dir := filepath.Join(home, ".deployhq", "logs")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return &Logger{}
	}

	name := fmt.Sprintf("deployhq-%s.log", time.Now().Format("2006-01-02T15-04-05"))
	path := filepath.Join(dir, name)

	f, err := os.Create(path)
	if err != nil {
		return &Logger{}
	}

	return &Logger{file: f, Path: path}
}

// Write writes a debug message to the log file.
func (l *Logger) Write(format string, args ...interface{}) {
	if l.file == nil {
		return
	}
	ts := time.Now().Format("15:04:05.000")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(l.file, "[%s] %s\n", ts, msg) //nolint:errcheck
}

// Writer returns the underlying io.Writer for the log file.
// Returns io.Discard if no file is available.
func (l *Logger) Writer() io.Writer {
	if l.file == nil {
		return io.Discard
	}
	return l.file
}

// Close closes the log file.
func (l *Logger) Close() {
	if l.file != nil {
		_ = l.file.Close()
	}
}
