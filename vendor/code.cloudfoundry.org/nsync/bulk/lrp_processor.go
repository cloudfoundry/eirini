package bulk

import (
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/cfhttp"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/nsync/helpers"
	"code.cloudfoundry.org/nsync/recipebuilder"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"code.cloudfoundry.org/runtimeschema/metric"
	"code.cloudfoundry.org/workpool"
	"github.com/cloudfoundry-incubator/routing-info/cfroutes"
	"github.com/cloudfoundry-incubator/routing-info/tcp_routes"
)

const (
	syncDesiredLRPsDuration = metric.Duration("DesiredLRPSyncDuration")
	invalidLRPsFound        = metric.Metric("NsyncInvalidDesiredLRPsFound")
)

type LRPProcessor struct {
	bbsClient             bbs.Client
	pollingInterval       time.Duration
	domainTTL             time.Duration
	bulkBatchSize         uint
	updateLRPWorkPoolSize int
	httpClient            *http.Client
	logger                lager.Logger
	fetcher               Fetcher
	builders              map[string]recipebuilder.RecipeBuilder
	clock                 clock.Clock
}

func NewLRPProcessor(
	logger lager.Logger,
	bbsClient bbs.Client,
	pollingInterval time.Duration,
	domainTTL time.Duration,
	bulkBatchSize uint,
	updateLRPWorkPoolSize int,
	skipCertVerify bool,
	fetcher Fetcher,
	builders map[string]recipebuilder.RecipeBuilder,
	clock clock.Clock,
) *LRPProcessor {
	return &LRPProcessor{
		bbsClient:             bbsClient,
		pollingInterval:       pollingInterval,
		domainTTL:             domainTTL,
		bulkBatchSize:         bulkBatchSize,
		updateLRPWorkPoolSize: updateLRPWorkPoolSize,
		httpClient:            initializeHttpClient(skipCertVerify),
		logger:                logger,
		fetcher:               fetcher,
		builders:              builders,
		clock:                 clock,
	}
}

func (l *LRPProcessor) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)

	timer := l.clock.NewTimer(l.pollingInterval)
	stop := l.sync(signals)

	for {
		if stop {
			return nil
		}

		select {
		case <-signals:
			return nil
		case <-timer.C():
			stop = l.sync(signals)
			timer.Reset(l.pollingInterval)
		}
	}
}

func (l *LRPProcessor) sync(signals <-chan os.Signal) bool {
	start := l.clock.Now()
	invalidsFound := int32(0)
	logger := l.logger.Session("sync-lrps")
	logger.Info("starting")

	defer func() {
		duration := l.clock.Now().Sub(start)
		err := syncDesiredLRPsDuration.Send(duration)
		if err != nil {
			logger.Error("failed-to-send-sync-desired-lrps-duration-metric", err)
		}
		err = invalidLRPsFound.Send(int(invalidsFound))
		if err != nil {
			logger.Error("failed-to-send-sync-invalid-lrps-found-metric", err)
		}
	}()

	defer logger.Info("done")

	existing, err := l.getSchedulingInfos(logger)
	if err != nil {
		return false
	}

	existingSchedulingInfoMap := organizeSchedulingInfosByProcessGuid(existing)
	appDiffer := NewAppDiffer(existingSchedulingInfoMap)

	cancelCh := make(chan struct{})

	// from here on out, the fetcher, differ, and processor work across channels in a pipeline
	fingerprintCh, fingerprintErrorCh := l.fetcher.FetchFingerprints(
		logger,
		cancelCh,
		l.httpClient,
	)

	diffErrorCh := appDiffer.Diff(
		logger,
		cancelCh,
		fingerprintCh,
	)

	missingAppCh, missingAppsErrorCh := l.fetcher.FetchDesiredApps(
		logger.Session("fetch-missing-desired-lrps-from-cc"),
		cancelCh,
		l.httpClient,
		appDiffer.Missing(),
	)

	createErrorCh := l.createMissingDesiredLRPs(logger, cancelCh, missingAppCh, &invalidsFound)

	staleAppCh, staleAppErrorCh := l.fetcher.FetchDesiredApps(
		logger.Session("fetch-stale-desired-lrps-from-cc"),
		cancelCh,
		l.httpClient,
		appDiffer.Stale(),
	)

	updateErrorCh := l.updateStaleDesiredLRPs(logger, cancelCh, staleAppCh, existingSchedulingInfoMap, &invalidsFound)

	bumpFreshness := true
	success := true

	fingerprintErrorCh, fingerprintErrorCount := countErrors(fingerprintErrorCh)

	// closes errors when all error channels have been closed.
	// below, we rely on this behavior to break the process_loop.
	errors := mergeErrors(
		fingerprintErrorCh,
		diffErrorCh,
		missingAppsErrorCh,
		staleAppErrorCh,
		createErrorCh,
		updateErrorCh,
	)

	logger.Info("processing-updates-and-creates")
process_loop:
	for {
		select {
		case err, open := <-errors:
			if err != nil {
				logger.Error("not-bumping-freshness-because-of", err)
				bumpFreshness = false
			}
			if !open {
				break process_loop
			}
		case sig := <-signals:
			logger.Info("exiting", lager.Data{"received-signal": sig})
			close(cancelCh)
			return true
		}
	}
	logger.Info("done-processing-updates-and-creates")

	if <-fingerprintErrorCount != 0 {
		logger.Error("failed-to-fetch-all-cc-fingerprints", nil)
		success = false
	}

	if success {
		deleteList := <-appDiffer.Deleted()
		l.deleteExcess(logger, cancelCh, deleteList)
	}

	if bumpFreshness && success {
		logger.Info("bumping-freshness")

		err = l.bbsClient.UpsertDomain(logger, cc_messages.AppLRPDomain, l.domainTTL)
		if err != nil {
			logger.Error("failed-to-upsert-domain", err)
		}
	}

	return false
}

func (l *LRPProcessor) createMissingDesiredLRPs(
	logger lager.Logger,
	cancel <-chan struct{},
	missing <-chan []cc_messages.DesireAppRequestFromCC,
	invalidCount *int32,
) <-chan error {
	logger = logger.Session("create-missing-desired-lrps")

	errc := make(chan error, 1)

	go func() {
		defer close(errc)

		for {
			var desireAppRequests []cc_messages.DesireAppRequestFromCC

			select {
			case <-cancel:
				return

			case selected, open := <-missing:
				if !open {
					return
				}

				desireAppRequests = selected
			}

			works := make([]func(), len(desireAppRequests))

			for i, desireAppRequest := range desireAppRequests {
				desireAppRequest := desireAppRequest
				var builder recipebuilder.RecipeBuilder = l.builders["buildpack"]
				if desireAppRequest.DockerImageUrl != "" {
					builder = l.builders["docker"]
				}

				works[i] = func() {
					logger.Debug("building-create-desired-lrp-request", desireAppRequestDebugData(&desireAppRequest))
					desired, err := builder.Build(&desireAppRequest)
					if err != nil {
						logger.Error("failed-building-create-desired-lrp-request", err, lager.Data{"process-guid": desireAppRequest.ProcessGuid})
						errc <- err
						return
					}
					logger.Debug("succeeded-building-create-desired-lrp-request", desireAppRequestDebugData(&desireAppRequest))

					logger.Debug("creating-desired-lrp", createDesiredReqDebugData(desired))
					err = l.bbsClient.DesireLRP(logger, desired)
					if err != nil {
						logger.Error("failed-creating-desired-lrp", err, lager.Data{"process-guid": desired.ProcessGuid})
						if models.ConvertError(err).Type == models.Error_InvalidRequest {
							atomic.AddInt32(invalidCount, int32(1))
						} else {
							errc <- err
						}
						return
					}
					logger.Debug("succeeded-creating-desired-lrp", createDesiredReqDebugData(desired))
				}
			}

			throttler, err := workpool.NewThrottler(l.updateLRPWorkPoolSize, works)
			if err != nil {
				errc <- err
				return
			}

			logger.Info("processing-batch", lager.Data{"size": len(desireAppRequests)})
			throttler.Work()
			logger.Info("done-processing-batch", lager.Data{"size": len(desireAppRequests)})
		}
	}()

	return errc
}

func (l *LRPProcessor) updateStaleDesiredLRPs(
	logger lager.Logger,
	cancel <-chan struct{},
	stale <-chan []cc_messages.DesireAppRequestFromCC,
	existingSchedulingInfoMap map[string]*models.DesiredLRPSchedulingInfo,
	invalidCount *int32,
) <-chan error {
	logger = logger.Session("update-stale-desired-lrps")

	errc := make(chan error, 1)

	go func() {
		defer close(errc)

		for {
			var staleAppRequests []cc_messages.DesireAppRequestFromCC

			select {
			case <-cancel:
				return

			case selected, open := <-stale:
				if !open {
					return
				}

				staleAppRequests = selected
			}

			works := make([]func(), len(staleAppRequests))

			for i, desireAppRequest := range staleAppRequests {
				desireAppRequest := desireAppRequest
				var builder recipebuilder.RecipeBuilder = l.builders["buildpack"]
				if desireAppRequest.DockerImageUrl != "" {
					builder = l.builders["docker"]
				}

				works[i] = func() {
					processGuid := desireAppRequest.ProcessGuid
					existingSchedulingInfo := existingSchedulingInfoMap[desireAppRequest.ProcessGuid]

					updateReq := &models.DesiredLRPUpdate{}
					instances := int32(desireAppRequest.NumInstances)
					updateReq.Instances = &instances
					updateReq.Annotation = &desireAppRequest.ETag

					exposedPorts, err := builder.ExtractExposedPorts(&desireAppRequest)
					if err != nil {
						logger.Error("failed-updating-stale-lrp", err, lager.Data{
							"process-guid":       processGuid,
							"execution-metadata": desireAppRequest.ExecutionMetadata,
						})
						errc <- err
						return
					}

					routes, err := helpers.CCRouteInfoToRoutes(desireAppRequest.RoutingInfo, exposedPorts)
					if err != nil {
						logger.Error("failed-to-marshal-routes", err)
						errc <- err
						return
					}

					updateReq.Routes = &routes

					for k, v := range existingSchedulingInfo.Routes {
						if k != cfroutes.CF_ROUTER && k != tcp_routes.TCP_ROUTER {
							(*updateReq.Routes)[k] = v
						}
					}

					logger.Debug("updating-stale-lrp", updateDesiredRequestDebugData(processGuid, updateReq))
					err = l.bbsClient.UpdateDesiredLRP(logger, processGuid, updateReq)
					if err != nil {
						logger.Error("failed-updating-stale-lrp", err, lager.Data{
							"process-guid": processGuid,
						})

						if models.ConvertError(err).Type == models.Error_InvalidRequest {
							atomic.AddInt32(invalidCount, int32(1))
						} else {
							errc <- err
						}
						return
					}
					logger.Debug("succeeded-updating-stale-lrp", updateDesiredRequestDebugData(processGuid, updateReq))
				}
			}

			throttler, err := workpool.NewThrottler(l.updateLRPWorkPoolSize, works)
			if err != nil {
				errc <- err
				return
			}

			logger.Info("processing-batch", lager.Data{"size": len(staleAppRequests)})
			throttler.Work()
			logger.Info("done-processing-batch", lager.Data{"size": len(staleAppRequests)})
		}
	}()

	return errc
}

func (l *LRPProcessor) getSchedulingInfos(logger lager.Logger) ([]*models.DesiredLRPSchedulingInfo, error) {
	logger.Info("getting-desired-lrps-from-bbs")
	existing, err := l.bbsClient.DesiredLRPSchedulingInfos(logger, models.DesiredLRPFilter{Domain: cc_messages.AppLRPDomain})
	if err != nil {
		logger.Error("failed-getting-desired-lrps-from-bbs", err)
		return nil, err
	}
	logger.Info("succeeded-getting-desired-lrps-from-bbs", lager.Data{"count": len(existing)})

	return existing, nil
}

func (l *LRPProcessor) deleteExcess(logger lager.Logger, cancel <-chan struct{}, excess []string) {
	logger = logger.Session("delete-excess")

	logger.Info("processing-batch", lager.Data{"num-to-delete": len(excess), "guids-to-delete": excess})
	deletedGuids := make([]string, 0, len(excess))
	for _, deleteGuid := range excess {
		err := l.bbsClient.RemoveDesiredLRP(logger, deleteGuid)
		if err != nil {
			logger.Error("failed-processing-batch", err, lager.Data{"delete-request": deleteGuid})
		} else {
			deletedGuids = append(deletedGuids, deleteGuid)
		}
	}
	logger.Info("succeeded-processing-batch", lager.Data{"num-deleted": len(deletedGuids), "deleted-guids": deletedGuids})
}

func countErrors(source <-chan error) (<-chan error, <-chan int) {
	count := make(chan int, 1)
	dest := make(chan error, 1)
	var errorCount int

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		for e := range source {
			errorCount++
			dest <- e
		}

		close(dest)
		wg.Done()
	}()

	go func() {
		wg.Wait()

		count <- errorCount
		close(count)
	}()

	return dest, count
}

func mergeErrors(channels ...<-chan error) <-chan error {
	out := make(chan error)
	wg := sync.WaitGroup{}

	for _, ch := range channels {
		wg.Add(1)

		go func(c <-chan error) {
			for e := range c {
				out <- e
			}
			wg.Done()
		}(ch)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

func organizeSchedulingInfosByProcessGuid(list []*models.DesiredLRPSchedulingInfo) map[string]*models.DesiredLRPSchedulingInfo {
	result := make(map[string]*models.DesiredLRPSchedulingInfo)
	for _, l := range list {
		lrp := l
		result[lrp.ProcessGuid] = lrp
	}

	return result
}

func updateDesiredRequestDebugData(processGuid string, updateDesiredRequest *models.DesiredLRPUpdate) lager.Data {
	return lager.Data{
		"process-guid": processGuid,
		"instances":    updateDesiredRequest.Instances,
	}
}

func createDesiredReqDebugData(createDesiredRequest *models.DesiredLRP) lager.Data {
	return lager.Data{
		"process-guid": createDesiredRequest.ProcessGuid,
		"log-guid":     createDesiredRequest.LogGuid,
		"metric-guid":  createDesiredRequest.MetricsGuid,
		"root-fs":      createDesiredRequest.RootFs,
		"instances":    createDesiredRequest.Instances,
		"timeout":      createDesiredRequest.StartTimeoutMs,
		"disk":         createDesiredRequest.DiskMb,
		"memory":       createDesiredRequest.MemoryMb,
		"cpu":          createDesiredRequest.CpuWeight,
		"privileged":   createDesiredRequest.Privileged,
	}
}

func desireAppRequestDebugData(desireAppRequest *cc_messages.DesireAppRequestFromCC) lager.Data {
	return lager.Data{
		"process-guid": desireAppRequest.ProcessGuid,
		"log-guid":     desireAppRequest.LogGuid,
		"stack":        desireAppRequest.Stack,
		"memory":       desireAppRequest.MemoryMB,
		"disk":         desireAppRequest.DiskMB,
		"file":         desireAppRequest.FileDescriptors,
		"instances":    desireAppRequest.NumInstances,
		"allow-ssh":    desireAppRequest.AllowSSH,
		"etag":         desireAppRequest.ETag,
	}
}

func initializeHttpClient(skipCertVerify bool) *http.Client {
	httpClient := cfhttp.NewClient()
	httpClient.Transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: skipCertVerify,
			MinVersion:         tls.VersionTLS10,
		},
	}
	return httpClient
}
