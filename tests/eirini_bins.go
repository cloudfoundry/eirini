package tests

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gofrs/flock"

	// nolint:golint,stylecheck
	. "github.com/onsi/ginkgo"

	// nolint:golint,stylecheck
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"gopkg.in/yaml.v2"
)

type EiriniBinaries struct {
	OPI                      Binary `json:"opi"`
	RouteCollector           Binary `json:"route_collector"`
	MetricsCollector         Binary `json:"metrics_collector"`
	RouteStatefulsetInformer Binary `json:"route_stateful_set_informer"`
	RoutePodInformer         Binary `json:"route_pod_informer"`
	EventsReporter           Binary `json:"events_reporter"`
	TaskReporter             Binary `json:"task_reporter"`
	EiriniController         Binary `json:"eirini_controller"`
	InstanceIndexEnvInjector Binary `json:"instance_index_env_injector"`
	ExternalBinsPath         bool
	BinsPath                 string
}

func NewEiriniBinaries() EiriniBinaries {
	bins := EiriniBinaries{}
	bins.setBinsPath()
	bins.OPI = NewBinary("code.cloudfoundry.org/eirini/cmd/opi", bins.BinsPath, []string{"connect"})
	bins.RouteCollector = NewBinary("code.cloudfoundry.org/eirini/cmd/route-collector", bins.BinsPath, []string{})
	bins.MetricsCollector = NewBinary("code.cloudfoundry.org/eirini/cmd/metrics-collector", bins.BinsPath, []string{})
	bins.RouteStatefulsetInformer = NewBinary("code.cloudfoundry.org/eirini/cmd/route-statefulset-informer", bins.BinsPath, []string{})
	bins.RoutePodInformer = NewBinary("code.cloudfoundry.org/eirini/cmd/route-pod-informer", bins.BinsPath, []string{})
	bins.EventsReporter = NewBinary("code.cloudfoundry.org/eirini/cmd/event-reporter", bins.BinsPath, []string{})
	bins.TaskReporter = NewBinary("code.cloudfoundry.org/eirini/cmd/task-reporter", bins.BinsPath, []string{})
	bins.EiriniController = NewBinary("code.cloudfoundry.org/eirini/cmd/eirini-controller", bins.BinsPath, []string{})
	bins.InstanceIndexEnvInjector = NewBinary("code.cloudfoundry.org/eirini/cmd/instance-index-env-injector", bins.BinsPath, []string{})

	return bins
}

func (b *EiriniBinaries) TearDown() {
	gexec.CleanupBuildArtifacts()

	if !b.ExternalBinsPath {
		os.RemoveAll(b.BinsPath)
	}
}

func (b *EiriniBinaries) setBinsPath() {
	binsPath := os.Getenv("EIRINI_BINS_PATH")
	b.ExternalBinsPath = true

	if binsPath == "" {
		b.ExternalBinsPath = false

		var err error
		binsPath, err = ioutil.TempDir("", "bins")
		Expect(err).NotTo(HaveOccurred())
	}

	b.BinsPath = binsPath
}

type Binary struct {
	PackagePath string   `json:"src_path"`
	BinPath     string   `json:"bin_path"`
	LocksDir    string   `json:"locks_dir"`
	ExtraArgs   []string `json:"extra_args"`
}

func NewBinary(packagePath, binsPath string, extraArgs []string) Binary {
	paths := strings.Split(packagePath, "/")
	binName := paths[len(paths)-1]

	return Binary{
		PackagePath: packagePath,
		BinPath:     filepath.Join(binsPath, binName),
		ExtraArgs:   extraArgs,
		LocksDir:    filepath.Join(binsPath, ".locks"),
	}
}

func (b *Binary) Run(config interface{}, envVars ...string) (*gexec.Session, string) {
	b.buildIfNecessary()

	configBytes, err := yaml.Marshal(config)
	Expect(err).NotTo(HaveOccurred())

	var configFile string
	if config != nil {
		configFile = WriteTempFile(configBytes, filepath.Base(b.BinPath)+"-config.yaml")
	}

	return b.runWithConfig(configFile, envVars...), configFile
}

func (b *Binary) runWithConfig(configFilePath string, envVars ...string) *gexec.Session {
	args := b.ExtraArgs
	if configFilePath != "" {
		args = append(args, "-c", configFilePath)
	}

	command := exec.Command(b.BinPath, args...) //#nosec G204
	command.Env = envVars
	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())

	return session
}

func (b *Binary) Restart(configFilePath string, runningSession *gexec.Session) *gexec.Session {
	envVars := runningSession.Command.Env
	runningSession.Kill().Wait()

	return b.runWithConfig(configFilePath, envVars...)
}

// Build builds the binary. Normally, you should not use this function as it is
// built if needed upon first run anyway. However, sometimes it might make sense
// to explicitly build a common binary that is used across all the tests
// in SynchronizedBeforeSuite thus preventing running the build on concurrent nodes.
//
// For example, EATs tests will always run OPI, therefore it is a good idea to
// build it in advance.
func (b *Binary) Build() {
	b.buildIfNecessary()
}

func (b *Binary) buildIfNecessary() {
	if _, err := os.Stat(b.BinPath); err == nil {
		return
	}

	lock := flock.New(b.BinPath + ".lock")
	err := lock.Lock()
	Expect(err).NotTo(HaveOccurred())

	defer func() {
		Expect(lock.Unlock()).To(Succeed())
	}()

	_, err = os.Stat(b.BinPath)
	if os.IsNotExist(err) {
		b.build()
	}
}

func (b *Binary) build() {
	compiledPath, err := gexec.Build(b.PackagePath)
	Expect(err).NotTo(HaveOccurred())
	Expect(os.MkdirAll(filepath.Dir(b.BinPath), 0o755)).To(Succeed())

	Expect(os.Link(compiledPath, b.BinPath)).To(Succeed())
}
