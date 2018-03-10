package logger

import "github.com/Sirupsen/logrus"

var (
	logger = logrus.New()
)

// Level log level
type Level uint32

const (
	// PanicLevel level
	PanicLevel Level = iota
	// FatalLevel level
	FatalLevel
	// ErrorLevel level
	ErrorLevel
	// WarnLevel level
	WarnLevel
	// InfoLevel level
	InfoLevel
	// DebugLevel level
	DebugLevel
)

// Logger logger
type Logger struct {
	log *logrus.Entry
}

// Fields map of key value fields
type Fields map[string]interface{}

// Init global setup
func Init(level Level) {
	logger.SetLevel(logrus.Level(level))
}

// NewLogger logger ctor
func NewLogger(fields Fields) *Logger {
	return &Logger{
		log: logger.WithFields(toLogrusFields(fields)),
	}
}

// LogInitializer config type
type LogInitializer func(l *Logger)

// SetCorrelationToken set correlation token
func SetCorrelationToken(correlationToken string) LogInitializer {
	return func(l *Logger) {
		l.log.Data["CorrelationToken"] = correlationToken
	}
}

// SetRequestInfo set request info
func SetRequestInfo(method string, path string) LogInitializer {
	return func(l *Logger) {
		l.log.Data["Method"] = method
		l.log.Data["path"] = path
	}
}

// TODO - instance per request, thread safe logger
// NewRequestLogger new instance per request logger
func NewRequestLogger(config ...LogInitializer) *Logger {
	log := &Logger{
		log: logger.WithField("test", "test"),
	}
	for _, fn := range config {
		fn(log)
	}
	return log
}

// FatalMsg fatal msg
func (l *Logger) FatalMsg(msg string) {
	l.log.Fatal(msg)
}

// Fatal fatal with error
func (l *Logger) Fatal(msg string, err error) {
	l.log.WithField("Error", err).Fatal(msg)
}

// FatalFields error with fields
func (l *Logger) FatalFields(msg string, fields Fields) {
	l.log.WithFields(toLogrusFields(fields)).Fatal(msg)
}

// ErrorMsg error msg
func (l *Logger) ErrorMsg(msg string) {
	l.log.Error(msg)
}

// Error error with error
func (l *Logger) Error(msg string, err error) {
	l.log.WithField("Error", err).Error(msg)
}

// ErrorFields error with fields
func (l *Logger) ErrorFields(msg string, fields Fields) {
	l.log.WithFields(toLogrusFields(fields)).Error(msg)
}

// InfoMsg info msg
func (l *Logger) InfoMsg(msg string) {
	l.log.Info(msg)
}

// Info info with fields
func (l *Logger) Info(msg string, fields Fields) {
	l.log.WithFields(toLogrusFields(fields)).Info(msg)
}

// DebugMsg debug msg
func (l *Logger) DebugMsg(msg string) {
	l.log.Debug(msg)
}

// Debug debug with fields
func (l *Logger) Debug(msg string, fields Fields) {
	l.log.WithFields(toLogrusFields(fields)).Debug(msg)
}

func toLogrusFields(fields Fields) logrus.Fields {
	var f = logrus.Fields{}
	for k, v := range fields {
		f[k] = v
	}
	return f
}
