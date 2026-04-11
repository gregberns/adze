// Package runner log file management.
//
// Log files are created only for steps/items that fail.
// Location: ~/.config/adze/logs/<step-name>.log (or <step-name>-<item>.log for batch items)
// Content: full stdout+stderr from the apply command.
// Lifecycle: overwritten on each run (not appended).
package runner

import (
	"fmt"
	"os"
	"path/filepath"
)

// EnsureLogDir creates the log directory if it does not exist.
func EnsureLogDir(logDir string) error {
	return os.MkdirAll(logDir, 0o755)
}

// LogFilePath returns the log file path for a step or batch item.
func LogFilePath(logDir, stepName, itemName string) string {
	var filename string
	if itemName != "" {
		filename = fmt.Sprintf("%s-%s.log", stepName, itemName)
	} else {
		filename = fmt.Sprintf("%s.log", stepName)
	}
	return filepath.Join(logDir, filename)
}

// WriteLog writes content to a log file, overwriting any existing file.
// Returns the full path on success, empty string on error.
func WriteLog(logDir, stepName, itemName, content string) string {
	if err := EnsureLogDir(logDir); err != nil {
		return ""
	}
	logPath := LogFilePath(logDir, stepName, itemName)
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		return ""
	}
	return logPath
}
