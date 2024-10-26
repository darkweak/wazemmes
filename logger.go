package wazemmes

import (
	"context"

	"github.com/http-wasm/http-wasm-host-go/api"
)

type Logger interface {
	Debug(args ...interface{})
	Info(args ...interface{})
	Warn(args ...interface{})
	Error(args ...interface{})
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
		l.Logger.Debug(message)
	case api.LogLevelInfo:
		l.Logger.Info(message)
	case api.LogLevelWarn:
		l.Logger.Warn(message)
	case api.LogLevelError:
		l.Logger.Error(message)
	}
}
