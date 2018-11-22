package tpsrunner

import (
	"encoding/json"
	"io/ioutil"
	"os/exec"

	"code.cloudfoundry.org/tps/config"
	. "github.com/onsi/gomega"

	"github.com/tedsuo/ifrit/ginkgomon"
)

func NewWatcher(bin string, watcherConfig config.WatcherConfig) *ginkgomon.Runner {
	configFile, err := ioutil.TempFile("", "listener_config")
	Expect(err).NotTo(HaveOccurred())

	watcherJSON, err := json.Marshal(watcherConfig)
	Expect(err).NotTo(HaveOccurred())
	err = ioutil.WriteFile(configFile.Name(), watcherJSON, 0644)
	Expect(err).NotTo(HaveOccurred())

	return ginkgomon.New(ginkgomon.Config{
		Name: "tps-watcher",
		Command: exec.Command(
			bin,
			"-configPath", configFile.Name(),
		),
		StartCheck: "tps-watcher.started",
	})
}
