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

func Debugf(format string, args ...any) {
	slog.Debug(fmt.Sprintf(format, args...))
}

func Infof(format string, args ...any) {
	slog.Info(fmt.Sprintf(format, args...))
}

func Warnf(format string, args ...any) {
	slog.Warn(fmt.Sprintf(format, args...))
}

func Errorf(format string, args ...any) {
	slog.Error(fmt.Sprintf(format, args...))
}
