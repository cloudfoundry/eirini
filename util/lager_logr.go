package util

import (
	"code.cloudfoundry.org/lager"
	"github.com/go-logr/logr"
)

// LagerLogr is a logr (https://github.com/go-logr/logr) implementation over lager.Logger
type LagerLogr struct {
	logger lager.Logger
}

func (l LagerLogr) Info(msg string, kvs ...interface{}) {
	l.logger.Info(msg, toLagerData(kvs))
}

func (l LagerLogr) Enabled() bool {
	return true
}

func NewLagerLogr(logger lager.Logger) logr.Logger {
	return LagerLogr{
		logger: logger,
	}
}

func (l LagerLogr) Error(err error, msg string, kvs ...interface{}) {
	l.logger.Error(msg, err, toLagerData(kvs))
}

func (l LagerLogr) V(level int) logr.Logger {
	return l
}

func (l LagerLogr) WithValues(kvs ...interface{}) logr.Logger {
	return l
}

func (l LagerLogr) WithName(name string) logr.Logger {
	return l
}

func toLagerData(kvs ...interface{}) lager.Data {
	return lager.Data{"data": kvs}
}
