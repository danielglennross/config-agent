package logger

import "github.com/Sirupsen/logrus"

var (
	logger = logrus.New()
)

func init() {
	logger.SetLevel(logrus.DebugLevel)
}

// Logger logger
type Logger struct {
	log *logrus.Entry
}

// Fields map of key value fields
type Fields map[string]interface{}

// NewLogger logger ctor
func NewLogger(fields Fields) *Logger {
	return &Logger{
		log: logger.WithFields(toLogrusFields(fields)),
	}
}

// FatalMsg fatal msg
func (l Logger) FatalMsg(msg string) {
	l.log.Fatal(msg)
}

// Fatal fatal with error
func (l Logger) Fatal(msg string, err error) {
	l.log.WithField("error", err).Fatal(msg)
}

// ErrorMsg error msg
func (l Logger) ErrorMsg(msg string) {
	l.log.Error(msg)
}

// Error error with error
func (l Logger) Error(msg string, err error) {
	l.log.WithField("error", err).Error(msg)
}

// InfoMsg info msg
func (l Logger) InfoMsg(msg string) {
	l.log.Info(msg)
}

// Info info with fields
func (l Logger) Info(msg string, fields Fields) {
	l.log.WithFields(toLogrusFields(fields)).Info(msg)
}

// DebugMsg debug msg
func (l Logger) DebugMsg(msg string) {
	l.log.Debug(msg)
}

// Debug debug with fields
func (l Logger) Debug(msg string, fields Fields) {
	l.log.WithFields(toLogrusFields(fields)).Debug(msg)
}

func toLogrusFields(fields Fields) logrus.Fields {
	var f = logrus.Fields{}
	for k, v := range fields {
		f[k] = v
	}
	return f
}
