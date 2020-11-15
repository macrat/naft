package logging

import (
	"log"
	"os"
)

type Level int

const (
	DEBUG Level = iota - 1
	INFO
	WARN
	ERROR
)

func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

var (
	DefaultLogger = NewLogger(INFO)
)

type Logger interface {
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

type SimpleLogger struct {
	DebugOut *log.Logger
	InfoOut  *log.Logger
	WarnOut  *log.Logger
	ErrorOut *log.Logger
	Level    Level
}

func NewLogger(level Level) *SimpleLogger {
	return &SimpleLogger{
		DebugOut: log.New(os.Stdout, "[DEBUG] ", log.Ldate | log.Lmicroseconds | log.Lmsgprefix),
		InfoOut:  log.New(os.Stdout, "[INFO ] ", log.Ldate | log.Lmicroseconds | log.Lmsgprefix),
		WarnOut:  log.New(os.Stderr, "[WARN ] ", log.Ldate | log.Lmicroseconds | log.Lmsgprefix),
		ErrorOut: log.New(os.Stderr, "[ERROR] ", log.Ldate | log.Lmicroseconds | log.Lmsgprefix),
		Level:    level,
	}
}

func (l *SimpleLogger) Debugf(format string, args ...interface{}) {
	if l.Level <= DEBUG {
		l.DebugOut.Printf(format, args...)
	}
}

func (l *SimpleLogger) Infof(format string, args ...interface{}) {
	if l.Level <= INFO {
		l.InfoOut.Printf(format, args...)
	}
}

func (l *SimpleLogger) Warnf(format string, args ...interface{}) {
	if l.Level <= WARN {
		l.WarnOut.Printf(format, args...)
	}
}

func (l *SimpleLogger) Errorf(format string, args ...interface{}) {
	if l.Level <= ERROR {
		l.ErrorOut.Printf(format, args...)
	}
}

type DummyLogger struct {}

func (l DummyLogger) Debugf(format string, args ...interface{}) {}

func (l DummyLogger) Infof(format string, args ...interface{}) {}

func (l DummyLogger) Warnf(format string, args ...interface{}) {}

func (l DummyLogger) Errorf(format string, args ...interface{}) {}
