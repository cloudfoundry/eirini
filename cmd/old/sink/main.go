package main

import (
	"crypto/tls"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/nsync/bulk"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"github.com/julz/cube/kubeconv"
	"k8s.io/api/apps/v1beta1"
	av1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// what this?
// little sync loop that create/updates kube deployments for any cf desired apps
// uses annotations to store the etag to know when to update stuff
// for droplets, converts them in to OCI images using `cubed` as a registry
func main() {
	kubeconfig := filepath.Join("/Users/julz", ".kube", "config")
	ccApi := flag.String("cc_api", "internal_user", "")
	ccUser := flag.String("cc_user", "internal_user", "")
	ccPass := flag.String("cc_pass", "", "")
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	batchSize := 50

	log := lager.NewLogger("sink")
	log.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	cancel := make(chan struct{})
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}

	fetcher := &bulk.CCFetcher{
		BaseURI:   *ccApi,
		BatchSize: batchSize,
		Username:  *ccUser,
		Password:  *ccPass,
	}

	ticker := time.NewTicker(15 * time.Second).C
	for range ticker {
		log.Info("tick", nil)

		existing, err := clientset.AppsV1beta1().Deployments("default").List(av1.ListOptions{
			LabelSelector: "cube",
		})
		if err != nil {
			log.Error("fetch-from-kube", err, nil)
			break
		}

		existingByGuid := make(map[string]string)
		for _, e := range existing.Items {
			existingByGuid[e.Name] = e.Labels["etag"]
		}

		log.Info("got-existing", lager.Data{"existing": existingByGuid})

		fingerprints, fingerprintErr := fetcher.FetchFingerprints(log, cancel, client)
		desired, desiredErr := fetcher.FetchDesiredApps(log, cancel, client, fingerprints)
		deployments := convert(log, cancel, desired)

		for d := range deployments {
			if _, ok := existingByGuid[d.Name]; !ok {
				_, err = clientset.AppsV1beta1().Deployments("default").Create(d)
				if err != nil {
					log.Error("created-deployment-failed", err, nil)
				}

				log.Info("created", lager.Data{"d": d, "e": err})
			} else if existingByGuid[d.Name] != d.Labels["etag"] {
				_, err = clientset.AppsV1beta1().Deployments("default").Update(d)
				if err != nil {
					log.Error("created-deployment-failed", err, nil)
				}

				log.Info("updated", lager.Data{"d": d, "e": err})
			} else {
				log.Info("skipped", lager.Data{"name": d.Name})
			}
		}

		wait(log, "fetch-fingerprints-error", fingerprintErr)
		wait(log, "fetch-desired-error", desiredErr)
	}
}

func convert(log lager.Logger, cancel chan (struct{}), desired <-chan []cc_messages.DesireAppRequestFromCC) chan *v1beta1.Deployment {
	converted := make(chan *v1beta1.Deployment)
	go func() {
		for fs := range desired {
			for _, f := range fs {
				converted <- kubeconv.Convert(f)
			}
		}
		log.Info("converted")
		close(converted)
	}()

	return converted
}

func wait(log lager.Logger, msg string, errorCh <-chan error) {
	err := <-errorCh
	if err != nil {
		log.Error(msg, err, nil)
	}
}
