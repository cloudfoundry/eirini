package metrics

import (
	"os"
	"time"

	"code.cloudfoundry.org/bbs/db/sqldb/helpers/monitor"
	"code.cloudfoundry.org/clock"
	logging "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/ifrit"
)

const (
	DefaultEmitFrequency = 60 * time.Second

	dbOpenConnectionsMetric  = "DBOpenConnections"
	dbQueriesTotalMetric     = "DBQueriesTotal"
	dbQueriesSucceededMetric = "DBQueriesSucceeded"
	dbQueriesFailedMetric    = "DBQueriesFailed"
	dbQueriesInFlightMetric  = "DBQueriesInFlight"
	dbQueryDurationMaxMetric = "DBQueryDurationMax"
)

//go:generate counterfeiter . DBStats
type DBStats interface {
	OpenConnections() int
}

type dbStatMetronNotifier struct {
	logger       lager.Logger
	clock        clock.Clock
	dbStats      DBStats
	metronClient logging.IngressClient
	monitor      monitor.Monitor
}

func NewDBStatMetronNotifier(logger lager.Logger, clock clock.Clock, dbStats DBStats, metronClient logging.IngressClient, monitor monitor.Monitor) ifrit.Runner {
	return &dbStatMetronNotifier{
		logger:       logger,
		clock:        clock,
		dbStats:      dbStats,
		metronClient: metronClient,
		monitor:      monitor,
	}
}

func (notifier *dbStatMetronNotifier) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := notifier.logger.Session("db-stat-metron-notifier")
	logger.Info("starting", lager.Data{"interval": DefaultEmitFrequency})
	defer logger.Info("completed")

	ticker := notifier.clock.NewTicker(DefaultEmitFrequency)
	close(ready)

	for {
		select {
		case <-signals:
			return nil
		case <-ticker.C():
			logger.Debug("emitting-metrics")

			openConnections := notifier.dbStats.OpenConnections()
			err := notifier.metronClient.SendMetric(dbOpenConnectionsMetric, openConnections)
			if err != nil {
				logger.Error("failed-sending-db-open-connections-count", err)
			}

			total := notifier.monitor.Total()
			err = notifier.metronClient.SendMetric(dbQueriesTotalMetric, int(total))
			if err != nil {
				logger.Error("failed-sending-db-queries-total-count", err)
			}

			succeeded := notifier.monitor.Succeeded()
			err = notifier.metronClient.SendMetric(dbQueriesSucceededMetric, int(succeeded))
			if err != nil {
				logger.Error("failed-sending-db-queries-succeeded-count", err)
			}

			failed := notifier.monitor.Failed()
			err = notifier.metronClient.SendMetric(dbQueriesFailedMetric, int(failed))
			if err != nil {
				logger.Error("failed-sending-db-queries-failed-count", err)
			}

			inFlightMax := notifier.monitor.ReadAndResetInFlightMax()
			err = notifier.metronClient.SendMetric(dbQueriesInFlightMetric, int(inFlightMax))
			if err != nil {
				logger.Error("failed-sending-db-queries-in-flight-count", err)
			}

			durationMax := notifier.monitor.ReadAndResetDurationMax()
			err = notifier.metronClient.SendDuration(dbQueryDurationMaxMetric, durationMax)
			if err != nil {
				logger.Error("failed-sending-db-query-duration-max", err)
			}

			logger.Debug("done-emitting-metrics")
		}
	}
}
