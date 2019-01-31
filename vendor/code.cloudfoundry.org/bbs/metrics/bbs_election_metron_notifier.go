package metrics

import (
	"os"

	"github.com/tedsuo/ifrit"

	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/lager"
)

const bbsMasterElectedMetric = "BBSMasterElected"

type BBSElectionMetronNotifier struct {
	Logger       lager.Logger
	metronClient loggingclient.IngressClient
}

func NewBBSElectionMetronNotifier(logger lager.Logger, metronClient loggingclient.IngressClient) ifrit.Runner {
	return &BBSElectionMetronNotifier{
		Logger:       logger,
		metronClient: metronClient,
	}
}

func (notifier BBSElectionMetronNotifier) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := notifier.Logger.Session("metrics-notifier")
	logger.Info("starting")

	close(ready)

	logger.Info("started")
	defer logger.Info("finished")

	notifier.metronClient.SendMetric(bbsMasterElectedMetric, 1)

	<-signals
	return nil
}
