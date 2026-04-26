package logger

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected LogLevel
	}{
		{"debug", DEBUG},
		{"DEBUG", DEBUG},
		{"Debug", DEBUG},
		{"info", INFO},
		{"INFO", INFO},
		{"Info", INFO},
		{"warn", WARN},
		{"WARN", WARN},
		{"warning", WARN},
		{"WARNing", WARN},
		{"error", ERROR},
		{"ERROR", ERROR},
		{"unknown", INFO},
		{"", INFO},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseLogLevel(tt.input)
			if got != tt.expected {
				t.Errorf("ParseLogLevel(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestLogLevel_String(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{DEBUG, "debug"},
		{INFO, "info"},
		{WARN, "warn"},
		{ERROR, "error"},
		{LogLevel(100), "info"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := tt.level.String()
			if got != tt.expected {
				t.Errorf("LogLevel(%d).String() = %q, want %q", tt.level, got, tt.expected)
			}
		})
	}
}

func TestSetLevel(t *testing.T) {
	originalLevel := currentLevel
	defer func() { currentLevel = originalLevel }()

	tests := []struct {
		input    string
		expected LogLevel
	}{
		{"debug", DEBUG},
		{"info", INFO},
		{"warn", WARN},
		{"error", ERROR},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			SetLevel(tt.input)
			if currentLevel != tt.expected {
				t.Errorf("SetLevel(%q) set level to %v, want %v", tt.input, currentLevel, tt.expected)
			}
		})
	}
}

func TestIsDebug(t *testing.T) {
	originalLevel := currentLevel
	defer func() { currentLevel = originalLevel }()

	currentLevel = DEBUG
	if !IsDebug() {
		t.Error("IsDebug() should return true when currentLevel is DEBUG")
	}

	currentLevel = INFO
	if IsDebug() {
		t.Error("IsDebug() should return false when currentLevel is INFO")
	}
}

func TestDebug_Output(t *testing.T) {
	originalLevel := currentLevel
	defer func() { currentLevel = originalLevel }()

	var buf bytes.Buffer
	logger.SetOutput(&buf)

	currentLevel = DEBUG
	Debug("test message %s", "arg")
	output := buf.String()
	if output == "" {
		t.Error("Debug() should output when level is DEBUG")
	}
	if !strings.Contains(output, "[DEBUG] test message arg") {
		t.Errorf("Debug() output = %q, want contains %q", output, "[DEBUG] test message arg")
	}

	buf.Reset()
	currentLevel = INFO
	Debug("should not output")
	if buf.Len() != 0 {
		t.Error("Debug() should not output when level is INFO")
	}

	logger.SetOutput(os.Stdout)
}

func TestInfo_Output(t *testing.T) {
	originalLevel := currentLevel
	defer func() { currentLevel = originalLevel }()

	var buf bytes.Buffer
	logger.SetOutput(&buf)

	currentLevel = INFO
	Info("info message")
	output := buf.String()
	if output == "" {
		t.Error("Info() should output when level is INFO")
	}

	buf.Reset()
	currentLevel = WARN
	Info("should not output")
	if buf.Len() != 0 {
		t.Error("Info() should not output when level is WARN")
	}

	logger.SetOutput(os.Stdout)
}

func TestWarn_Output(t *testing.T) {
	originalLevel := currentLevel
	defer func() { currentLevel = originalLevel }()

	var buf bytes.Buffer
	logger.SetOutput(&buf)

	currentLevel = WARN
	Warn("warn message")
	output := buf.String()
	if output == "" {
		t.Error("Warn() should output when level is WARN")
	}

	buf.Reset()
	currentLevel = ERROR
	Warn("should not output")
	if buf.Len() != 0 {
		t.Error("Warn() should not output when level is ERROR")
	}

	logger.SetOutput(os.Stdout)
}

func TestError_Output(t *testing.T) {
	originalLevel := currentLevel
	defer func() { currentLevel = originalLevel }()

	var buf bytes.Buffer
	logger.SetOutput(&buf)

	currentLevel = ERROR
	Error("error message")
	output := buf.String()
	if output == "" {
		t.Error("Error() should output when level is ERROR")
	}

	buf.Reset()
	currentLevel = 100
	Error("should not output")
	if buf.Len() != 0 {
		t.Error("Error() should not output when level is unknown")
	}

	logger.SetOutput(os.Stdout)
}