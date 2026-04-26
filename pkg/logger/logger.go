package logger

import (
	"log"
	"os"
	"strings"
)

// LogLevel represents the logging level
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "debug"
	case INFO:
		return "info"
	case WARN:
		return "warn"
	case ERROR:
		return "error"
	default:
		return "info"
	}
}

// ParseLogLevel parses a string to LogLevel
func ParseLogLevel(s string) LogLevel {
	switch strings.ToLower(s) {
	case "debug":
		return DEBUG
	case "info":
		return INFO
	case "warn", "warning":
		return WARN
	case "error":
		return ERROR
	default:
		return INFO
	}
}

var (
	currentLevel = INFO
	logger       = log.New(os.Stdout, "", log.LstdFlags)
)

// SetLevel sets the global log level
func SetLevel(level string) {
	currentLevel = ParseLogLevel(level)
}

// Debug logs at DEBUG level
func Debug(format string, v ...interface{}) {
	if currentLevel <= DEBUG {
		logger.Printf("[DEBUG] "+format, v...)
	}
}

// Info logs at INFO level
func Info(format string, v ...interface{}) {
	if currentLevel <= INFO {
		logger.Printf("[INFO] "+format, v...)
	}
}

// Warn logs at WARN level
func Warn(format string, v ...interface{}) {
	if currentLevel <= WARN {
		logger.Printf("[WARN] "+format, v...)
	}
}

// Error logs at ERROR level
func Error(format string, v ...interface{}) {
	if currentLevel <= ERROR {
		logger.Printf("[ERROR] "+format, v...)
	}
}

// IsDebug returns true if current level is DEBUG
func IsDebug() bool {
	return currentLevel == DEBUG
}