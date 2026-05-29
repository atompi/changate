package logger

import (
	"fmt"
	"log/slog"

	"github.com/atompi/goutil/log"
)

func Init(opts ...log.Options) *slog.Logger {
	l := log.NewLoggerOptions(opts...)
	return log.NewSlogLogger(l)
}

func LogFormatter(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}
