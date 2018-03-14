package logger

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/Sirupsen/logrus"
)

// TODO move this to base package
// ContextKey context key
type ContextKey string

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
	logrus *logrus.Logger
	log    *logrus.Entry
	mu     sync.Mutex
}

// Fields map of key value fields
type Fields map[string]interface{}

// Formatter format logs
type Formatter logrus.Formatter

// NewLogger logger ctor
func NewLogger(fields Fields) *Logger {
	base := logrus.New()

	return &Logger{
		logrus: base,
		log:    base.WithFields(toLogrusFields(fields)),
		mu:     sync.Mutex{},
	}
}

// SetLevel global setup
func (l *Logger) SetLevel(level Level) {
	l.logrus.SetLevel(logrus.Level(level))
}

// SetFormatter sets the formatter of the logger
func (l *Logger) SetFormatter(f Formatter) {
	l.logrus.Formatter = f
}

// SetOutput sets the output of the logger
func (l *Logger) SetOutput(o io.Writer) {
	l.logrus.Out = o
}

// NewContext gets a context baked logger
func (l *Logger) NewContext(ctx context.Context) *Logger {
	correlationToken := ctx.Value(ContextKey("CorrelationToken")).(string)
	request := ctx.Value(ContextKey("Request")).(string)
	return &Logger{
		log: l.log.WithFields(toLogrusFields(Fields{
			"CorrelationToken": correlationToken,
			"Request":          request,
		})),
		logrus: l.logrus,
		mu:     sync.Mutex{},
	}
}

// FatalMsg fatal msg
func (l *Logger) FatalMsg(msg string) {
	l.safeLog(func(log *logrus.Entry) {
		log.Fatal(msg)
	})
}

// Fatal fatal with error
func (l *Logger) Fatal(msg string, err error) {
	l.safeLog(func(log *logrus.Entry) {
		log.WithField("Error", err).Fatal(msg)
	})
}

// FatalFields error with fields
func (l *Logger) FatalFields(msg string, fields Fields) {
	l.safeLog(func(log *logrus.Entry) {
		log.WithFields(toLogrusFields(fields)).Fatal(msg)
	})
}

// ErrorMsg error msg
func (l *Logger) ErrorMsg(msg string) {
	l.safeLog(func(log *logrus.Entry) {
		log.Error(msg)
	})
}

// Error error with error
func (l *Logger) Error(msg string, err error) {
	l.safeLog(func(log *logrus.Entry) {
		log.WithField("Error", err).Error(msg)
	})
}

// ErrorFields error with fields
func (l *Logger) ErrorFields(msg string, fields Fields) {
	l.safeLog(func(log *logrus.Entry) {
		log.WithFields(toLogrusFields(fields)).Error(msg)
	})
}

// InfoMsg info msg
func (l *Logger) InfoMsg(msg string) {
	l.safeLog(func(log *logrus.Entry) {
		log.Info(msg)
	})
}

// Info info with fields
func (l *Logger) Info(msg string, fields Fields) {
	l.safeLog(func(log *logrus.Entry) {
		log.WithFields(toLogrusFields(fields)).Info(msg)
	})
}

// DebugMsg debug msg
func (l *Logger) DebugMsg(msg string) {
	l.safeLog(func(log *logrus.Entry) {
		log.Debug(msg)
	})
}

// Debug debug with fields
func (l *Logger) Debug(msg string, fields Fields) {
	l.safeLog(func(log *logrus.Entry) {
		log.WithFields(toLogrusFields(fields)).Debug(msg)
	})
}

func toLogrusFields(fields Fields) logrus.Fields {
	var f = logrus.Fields{}
	for k, v := range fields {
		f[k] = v
	}
	return f
}

func (l *Logger) safeLog(logCall func(entry *logrus.Entry)) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered from logging panic", r)
		}
	}()

	l.mu.Lock()
	defer l.mu.Unlock()

	logCall(l.log)
}
