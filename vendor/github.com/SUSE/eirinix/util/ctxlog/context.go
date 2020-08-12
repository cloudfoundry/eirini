package ctxlog

import (
	"context"

	"go.uber.org/zap"
)

type ctxLogger struct{}

// key must be comparable and should not be of type string
var (
	ctxLoggerKey = &ctxLogger{}
	nopLogger    = zap.NewNop().Sugar()
)

// NewManagerContext returns a new context with a logger
func NewManagerContext(log *zap.SugaredLogger) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, ctxLoggerKey, log)
	return ctx
}

// NewReconcilerContext includes a named logger for the reconciler
func NewReconcilerContext(ctx context.Context, name string) context.Context {
	log := ExtractLogger(ctx)
	if log != nil {
		log = log.Named(name)
	}
	return context.WithValue(ctx, ctxLoggerKey, log)
}

// ExtractLogger returns the logger from the context
func ExtractLogger(ctx context.Context) *zap.SugaredLogger {
	log, ok := ctx.Value(ctxLoggerKey).(*zap.SugaredLogger)
	if !ok || log == nil {
		return nopLogger
	}
	return log
}

// Debug uses the stored zap logger
func Debug(ctx context.Context, v ...interface{}) {
	log := ExtractLogger(ctx)
	log.Debug(v...)
}

// Info uses the stored zap logger
func Info(ctx context.Context, v ...interface{}) {
	log := ExtractLogger(ctx)
	log.Info(v...)
}

// Error uses the stored zap logger
func Error(ctx context.Context, v ...interface{}) {
	log := ExtractLogger(ctx)
	log.Error(v...)
}

// Debugf uses the stored zap logger
func Debugf(ctx context.Context, format string, v ...interface{}) {
	log := ExtractLogger(ctx)
	log.Debugf(format, v...)
}

// Infof uses the stored zap logger
func Infof(ctx context.Context, format string, v ...interface{}) {
	log := ExtractLogger(ctx)
	log.Infof(format, v...)
}

// Errorf uses the stored zap logger
func Errorf(ctx context.Context, format string, v ...interface{}) {
	log := ExtractLogger(ctx)
	log.Errorf(format, v...)
}
