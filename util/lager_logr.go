package util

import (
	"code.cloudfoundry.org/lager"
	"github.com/go-logr/logr"
)

type LagerInfoLogr struct {
	logger lager.Logger
}

func (l LagerInfoLogr) Info(msg string, kvs ...interface{}) {
	l.logger.Info(msg, toLagerData(kvs))
}

func (l LagerInfoLogr) Enabled() bool {
	return true
}

func toLagerData(kvs ...interface{}) lager.Data {
	return lager.Data{"data": kvs}
}

// LagerLogr is a logr (https://github.com/go-logr/logr) implementation over lager.Logger
type LagerLogr struct {
	LagerInfoLogr
}

func NewLagerLogr(logger lager.Logger) logr.Logger {
	return LagerLogr{
		LagerInfoLogr: LagerInfoLogr{logger: logger},
	}
}

func (l LagerLogr) Error(err error, msg string, kvs ...interface{}) {
	l.logger.Error(msg, err, toLagerData(kvs))
}

func (l LagerLogr) V(level int) logr.InfoLogger {
	return &LagerInfoLogr{logger: l.logger}
}

func (l LagerLogr) WithValues(kvs ...interface{}) logr.Logger {
	return l
}

func (l LagerLogr) WithName(name string) logr.Logger {
	return l
}
