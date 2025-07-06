package utils

import (
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"swagger-docs-mcp/pkg/types"
)

// Logger provides structured logging functionality
type Logger struct {
	zapLogger *zap.Logger
	config    types.LoggingConfig
}

// NewLogger creates a new logger with the given configuration
func NewLogger(config types.LoggingConfig) *Logger {
	zapConfig := buildZapConfig(config)

	logger, err := zapConfig.Build()
	if err != nil {
		// Fallback to a basic logger if config fails
		logger = zap.NewNop()
	}

	return &Logger{
		zapLogger: logger,
		config:    config,
	}
}

// Child creates a child logger with a namespace prefix
func (l *Logger) Child(namespace string) *Logger {
	return &Logger{
		zapLogger: l.zapLogger.Named(namespace),
		config:    l.config,
	}
}

// Debug logs a debug message
func (l *Logger) Debug(message string, fields ...interface{}) {
	if !l.config.Enabled {
		return
	}

	if len(fields) == 0 {
		l.zapLogger.Debug(message)
	} else {
		zapFields := l.convertToZapFields(fields...)
		l.zapLogger.Debug(message, zapFields...)
	}
}

// Info logs an info message
func (l *Logger) Info(message string, fields ...interface{}) {
	if !l.config.Enabled {
		return
	}

	if len(fields) == 0 {
		l.zapLogger.Info(message)
	} else {
		zapFields := l.convertToZapFields(fields...)
		l.zapLogger.Info(message, zapFields...)
	}
}

// Warn logs a warning message
func (l *Logger) Warn(message string, fields ...interface{}) {
	if !l.config.Enabled {
		return
	}

	if len(fields) == 0 {
		l.zapLogger.Warn(message)
	} else {
		zapFields := l.convertToZapFields(fields...)
		l.zapLogger.Warn(message, zapFields...)
	}
}

// Error logs an error message
func (l *Logger) Error(message string, fields ...interface{}) {
	if !l.config.Enabled {
		return
	}

	if len(fields) == 0 {
		l.zapLogger.Error(message)
	} else {
		zapFields := l.convertToZapFields(fields...)
		l.zapLogger.Error(message, zapFields...)
	}
}

// convertToZapFields converts interface{} fields to zap fields
func (l *Logger) convertToZapFields(fields ...interface{}) []zap.Field {
	var zapFields []zap.Field

	for i := 0; i < len(fields); i++ {
		switch field := fields[i].(type) {
		case zap.Field:
			zapFields = append(zapFields, field)
		case map[string]interface{}:
			for key, value := range field {
				zapFields = append(zapFields, zap.Any(key, value))
			}
		case error:
			zapFields = append(zapFields, zap.Error(field))
		default:
			// Try to handle as key-value pairs
			if i+1 < len(fields) {
				key := fmt.Sprintf("%v", field)
				value := fields[i+1]
				zapFields = append(zapFields, zap.Any(key, value))
				i++ // Skip the next field as it's the value
			} else {
				zapFields = append(zapFields, zap.Any("field", field))
			}
		}
	}

	return zapFields
}

// UpdateConfig updates the logger configuration
func (l *Logger) UpdateConfig(config types.LoggingConfig) {
	l.config = config

	// Rebuild logger with new config
	zapConfig := buildZapConfig(config)
	newLogger, err := zapConfig.Build()
	if err != nil {
		l.Error("Failed to update logger config", zap.Error(err))
		return
	}

	// Replace logger instance
	l.zapLogger = newLogger
}

// Close flushes any buffered log entries
func (l *Logger) Close() error {
	if l.zapLogger != nil {
		// Ignore sync errors for stderr as they're common and harmless
		_ = l.zapLogger.Sync()
	}
	return nil
}

// buildZapConfig creates a zap configuration from LoggingConfig
func buildZapConfig(config types.LoggingConfig) zap.Config {
	// Set log level
	var zapLevel zapcore.Level
	switch strings.ToLower(config.Level) {
	case "debug":
		zapLevel = zapcore.DebugLevel
	case "info":
		zapLevel = zapcore.InfoLevel
	case "warn", "warning":
		zapLevel = zapcore.WarnLevel
	case "error":
		zapLevel = zapcore.ErrorLevel
	default:
		zapLevel = zapcore.InfoLevel
	}

	// Create custom encoder config to match the desired format
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "name",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    customLevelEncoder,
		EncodeTime:     customTimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
		EncodeName:     customNameEncoder,
	}

	// Create config with custom encoder
	zapConfig := zap.Config{
		Level:       zap.NewAtomicLevelAt(zapLevel),
		Development: false,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding:         "console",
		EncoderConfig:    encoderConfig,
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}

	// Disable logging if not enabled
	if !config.Enabled {
		zapConfig.Level = zap.NewAtomicLevelAt(zapcore.PanicLevel + 1) // Disable all logging
	}

	return zapConfig
}

// customTimeEncoder formats time in ISO format
func customTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.UTC().Format("2006-01-02T15:04:05.000Z"))
}

// customLevelEncoder formats level in brackets
func customLevelEncoder(level zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString("[" + level.String() + "]")
}

// customNameEncoder formats logger name in brackets
func customNameEncoder(loggerName string, enc zapcore.PrimitiveArrayEncoder) {
	if loggerName == "" {
		enc.AppendString("[swagger-docs-go]")
	} else {
		enc.AppendString("[swagger-docs-go:" + loggerName + "]")
	}
}

// GetZapLogger returns the underlying zap logger
func (l *Logger) GetZapLogger() *zap.Logger {
	return l.zapLogger
}

// GetSugar returns the sugared logger for printf-style logging
func (l *Logger) GetSugar() *zap.SugaredLogger {
	return l.zapLogger.Sugar()
}

// Legacy methods for backwards compatibility

// LogInfo logs an informational message (legacy method)
func (l *Logger) LogInfo(message string, context ...map[string]interface{}) {
	fields := make([]interface{}, 0, len(context))
	for _, ctx := range context {
		fields = append(fields, ctx)
	}
	l.Info(message, fields...)
}

// LogError logs an error message (legacy method)
func (l *Logger) LogError(message string, err error, context ...map[string]interface{}) {
	fields := []interface{}{zap.Error(err)}
	for _, ctx := range context {
		fields = append(fields, ctx)
	}
	l.Error(message, fields...)
}

// LogDebug logs a debug message (legacy method)
func (l *Logger) LogDebug(message string, context ...map[string]interface{}) {
	fields := make([]interface{}, 0, len(context))
	for _, ctx := range context {
		fields = append(fields, ctx)
	}
	l.Debug(message, fields...)
}

// LogWarn logs a warning message (legacy method)
func (l *Logger) LogWarn(message string, context ...map[string]interface{}) {
	fields := make([]interface{}, 0, len(context))
	for _, ctx := range context {
		fields = append(fields, ctx)
	}
	l.Warn(message, fields...)
}

// Printf-style logging methods

// Debugf logs a debug message with printf-style formatting
func (l *Logger) Debugf(format string, args ...interface{}) {
	if !l.config.Enabled {
		return
	}
	l.zapLogger.Debug(fmt.Sprintf(format, args...))
}

// Infof logs an info message with printf-style formatting
func (l *Logger) Infof(format string, args ...interface{}) {
	if !l.config.Enabled {
		return
	}
	l.zapLogger.Info(fmt.Sprintf(format, args...))
}

// Warnf logs a warning message with printf-style formatting
func (l *Logger) Warnf(format string, args ...interface{}) {
	if !l.config.Enabled {
		return
	}
	l.zapLogger.Warn(fmt.Sprintf(format, args...))
}

// Errorf logs an error message with printf-style formatting
func (l *Logger) Errorf(format string, args ...interface{}) {
	if !l.config.Enabled {
		return
	}
	l.zapLogger.Error(fmt.Sprintf(format, args...))
}
