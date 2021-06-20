package vlog

import (
	"log"
	"os"
)

type VLogger struct {
	*log.Logger
	verbose bool
}

func New() *VLogger {
	return &VLogger{
		Logger: log.New(os.Stdout, "", log.LstdFlags),
	}
}

func (l *VLogger) Verbose(v ...interface{}) {
	if l.verbose {
		l.Print(v...)
	}
}

func (l *VLogger) Verbosef(format string, v ...interface{}) {
	if l.verbose {
		l.Printf(format, v...)
	}
}

func (l *VLogger) SetVerbose(verbose bool) {
	l.verbose = verbose
}
