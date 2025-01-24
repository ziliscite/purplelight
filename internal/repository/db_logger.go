package repository

import (
	"fmt"
	"log/slog"
	"runtime"
	"strings"
)

type dbLogger struct {
	sl *slog.Logger
}

func (l *dbLogger) Error(msg string, args ...any) {
	_, file, line, _ := runtime.Caller(2)
	shortFile := file
	if strings.Contains(file, "GolandProjects/purplelight") {
		shortFile = strings.Replace(file, "C:/Users/manzi/GolandProjects/purplelight", ".", 1)
	}
	trace := fmt.Sprintf("%s:%d", shortFile, line)
	args = append(args, "trace", trace)
	l.sl.Error(msg, args...)
}
