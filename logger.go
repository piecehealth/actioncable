package actioncable

import (
	"log"
)

type LogLevel int

const (
	silent LogLevel = iota + 1
	err
	warn
	info
	debug
)

type Logger interface {
	Debug(string)
	Info(string)
	Error(string)
}

type defaultLogger struct {
	Level LogLevel
}

var _ Logger = (*defaultLogger)(nil)

func (l *defaultLogger) Debug(msg string) {
	if l.Level >= debug {
		log.Println("[ActionCable][Debug] " + msg)
	}
}

func (l *defaultLogger) Info(msg string) {
	if l.Level >= info {
		log.Println("[ActionCable][Info] " + msg)
	}
}

func (l *defaultLogger) Warn(msg string) {
	if l.Level >= warn {
		log.Println("[ActionCable][Warn]" + msg)
	}
}

func (l *defaultLogger) Error(msg string) {
	if l.Level >= err {
		log.Println("[ActionCable][Error] " + msg)
	}
}
