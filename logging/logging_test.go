package logging

import (
	"testing"
	"bytes"
	"log"
)

func TestLevel(t *testing.T) {
	if DEBUG != -1 {
		t.Errorf("DEBUG is %d", DEBUG)
	}
	if DEBUG.String() != "DEBUG" {
		t.Errorf("DEBUG.String() is %#v", DEBUG.String())
	}
	if INFO != 0 {
		t.Errorf("INFO is %d", INFO)
	}
	if INFO.String() != "INFO" {
		t.Errorf("INFO.String() is %#v", INFO.String())
	}
	if WARN != 1 {
		t.Errorf("WARN is %d", WARN)
	}
	if WARN.String() != "WARN" {
		t.Errorf("WARN.String() is %#v", WARN.String())
	}
	if ERROR != 2 {
		t.Errorf("ERROR is %d", ERROR)
	}
	if ERROR.String() != "ERROR" {
		t.Errorf("ERROR.String() is %#v", ERROR.String())
	}
}

func TestSimpleLogger(t *testing.T) {
	debugOut := bytes.NewBuffer([]byte{})
	infoOut := bytes.NewBuffer([]byte{})
	warnOut := bytes.NewBuffer([]byte{})
	errorOut := bytes.NewBuffer([]byte{})

	l := &SimpleLogger{
		DebugOut: log.New(debugOut, "", 0),
		InfoOut:  log.New(infoOut, "", 0),
		WarnOut:  log.New(warnOut, "", 0),
		ErrorOut: log.New(errorOut, "", 0),
	}

	if l.Level != INFO {
		t.Errorf("expected default log level is INFO but got %s", l.Level)
		l.Level = INFO
	}

	l.Debugf("hello world")
	if debugOut.Len() != 0 {
		t.Errorf("unexpected debug log: %s", string(debugOut.Bytes()))
	}

	l.Level = DEBUG
	l.Debugf("hello %d", 42)
	if string(debugOut.Bytes()) != "hello 42\n" {
		t.Errorf("unexpected debug log: %s", string(debugOut.Bytes()))
	}

	l.Level = WARN
	l.Infof("hello world")
	if infoOut.Len() != 0 {
		t.Errorf("unexpected info log: %s", string(infoOut.Bytes()))
	}

	l.Level = INFO
	l.Infof("hello %s", "world")
	if string(infoOut.Bytes()) != "hello world\n" {
		t.Errorf("unexpected info log: %s", string(infoOut.Bytes()))
	}

	l.Level = ERROR
	l.Warnf("hello world")
	if warnOut.Len() != 0 {
		t.Errorf("unexpected info log: %s", string(warnOut.Bytes()))
	}

	l.Level = WARN
	l.Warnf("hello %#v", "world")
	if string(warnOut.Bytes()) != "hello \"world\"\n" {
		t.Errorf("unexpected info log: %s", string(warnOut.Bytes()))
	}

	l.Level = ERROR
	l.Errorf("hello %d", 123)
	if string(errorOut.Bytes()) != "hello 123\n" {
		t.Errorf("unexpected info log: %s", string(errorOut.Bytes()))
	}
}
