package ctxlog

import (
	"context"
	"os"
	"path"

	"go.uber.org/zap"
)

type correlationIDType int

const (
	requestIDKey correlationIDType = iota
	sessionIDKey
)

var logger *zap.Logger

// Config defineds logger configuration parameters.
type Config struct {
	Level            string   `json:"level"`
	Format           string   `json:"encoding"`
	OutputPaths      []string `json:"outputPaths"`
	ErrorOutputPaths []string `json:"errorOutputPaths"`
}

// BuildLogger builds the `logger` with the given configurations.
func BuildLogger(c Config) (err error) {

	cfg := zap.NewProductionConfig()
	cfg.Encoding = c.Format
	cfg.OutputPaths = c.OutputPaths
	cfg.ErrorOutputPaths = c.ErrorOutputPaths

	switch c.Level {
	case "debug":
		cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	default:
		cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	if logger, err = cfg.Build(); err != nil {
		return
	}
	return
}

func init() {
	// a fallback/root logger for events without context
	logger, _ = zap.NewProduction(
		zap.Fields(zap.Int("pid", os.Getpid()), zap.String("exe", path.Base(os.Args[0]))),
	)
}

// WithRqID returns a context which knows its request ID
func WithRqID(ctx context.Context, rqID string) context.Context {
	return context.WithValue(ctx, requestIDKey, rqID)
}

// WithSessionID returns a context which knows its session ID
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionIDKey, sessionID)
}

// Logger returns a zap logger with as much context as possible
func Logger(ctx context.Context) *zap.Logger {
	newLogger := logger
	if ctx != nil {
		if ctxRqID, ok := ctx.Value(requestIDKey).(string); ok {
			newLogger = newLogger.With(zap.String("requestID", ctxRqID))
		}
		if ctxSessionID, ok := ctx.Value(sessionIDKey).(string); ok {
			newLogger = newLogger.With(zap.String("sessionID", ctxSessionID))
		}
	}
	return newLogger
}
