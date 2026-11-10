package wazemmes

import (
	"context"

	"github.com/http-wasm/http-wasm-host-go/api"
)

type Logger interface {
	Debug(args ...any)
	Info(args ...any)
	Warn(args ...any)
	Error(args ...any)
}

type logger struct {
	Logger
}

func NewLogger(l Logger) *logger {
	return &logger{
		Logger: l,
	}
}

func (l logger) IsEnabled(level api.LogLevel) bool {
	return true
}

func (l logger) Log(_ context.Context, level api.LogLevel, message string) {
	switch level {
	case api.LogLevelDebug:
		l.Debug(message)
	case api.LogLevelInfo:
		l.Info(message)
	case api.LogLevelWarn:
		l.Warn(message)
	case api.LogLevelError:
		l.Error(message)
	}
}
