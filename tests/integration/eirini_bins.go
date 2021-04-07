package integration

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/tests"
	"github.com/gofrs/flock"

	// nolint:golint,stylecheck,revive
	. "github.com/onsi/ginkgo"

	// nolint:golint,stylecheck,revive
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"gopkg.in/yaml.v2"
)

type EiriniBinaries struct {
	OPI                      Binary `json:"opi"`
	RouteCollector           Binary `json:"route_collector"`
	RouteStatefulsetInformer Binary `json:"route_stateful_set_informer"`
	RoutePodInformer         Binary `json:"route_pod_informer"`
	EventsReporter           Binary `json:"events_reporter"`
	TaskReporter             Binary `json:"task_reporter"`
	EiriniController         Binary `json:"eirini_controller"`
	InstanceIndexEnvInjector Binary `json:"instance_index_env_injector"`
	Migration                Binary `json:"migration"`
	ResourceValidator        Binary `json:"resource_validator"`
	ExternalBinsPath         bool
	BinsPath                 string
	CertsPath                string
}

func NewEiriniBinaries() EiriniBinaries {
	bins := EiriniBinaries{}

	bins.CertsPath, _ = tests.GenerateKeyPairDir("tls", "localhost")

	bins.setBinsPath()
	bins.OPI = NewBinary("code.cloudfoundry.org/eirini/cmd/opi", bins.BinsPath, []string{"connect"}, bins.CertsPath)
	bins.RouteCollector = NewBinary("code.cloudfoundry.org/eirini/cmd/route-collector", bins.BinsPath, []string{}, bins.CertsPath)
	bins.RouteStatefulsetInformer = NewBinary("code.cloudfoundry.org/eirini/cmd/route-statefulset-informer", bins.BinsPath, []string{}, bins.CertsPath)
	bins.RoutePodInformer = NewBinary("code.cloudfoundry.org/eirini/cmd/route-pod-informer", bins.BinsPath, []string{}, bins.CertsPath)
	bins.EventsReporter = NewBinary("code.cloudfoundry.org/eirini/cmd/event-reporter", bins.BinsPath, []string{}, bins.CertsPath)
	bins.TaskReporter = NewBinary("code.cloudfoundry.org/eirini/cmd/task-reporter", bins.BinsPath, []string{}, bins.CertsPath)
	bins.EiriniController = NewBinary("code.cloudfoundry.org/eirini/cmd/eirini-controller", bins.BinsPath, []string{}, bins.CertsPath)
	bins.InstanceIndexEnvInjector = NewBinary("code.cloudfoundry.org/eirini/cmd/instance-index-env-injector", bins.BinsPath, []string{}, bins.CertsPath)
	bins.Migration = NewBinary("code.cloudfoundry.org/eirini/cmd/migration", bins.BinsPath, []string{}, bins.CertsPath)
	bins.ResourceValidator = NewBinary("code.cloudfoundry.org/eirini/cmd/resource-validator", bins.BinsPath, []string{}, bins.CertsPath)

	return bins
}

func (b *EiriniBinaries) TearDown() {
	gexec.CleanupBuildArtifacts()

	if !b.ExternalBinsPath {
		os.RemoveAll(b.BinsPath)
	}

	os.RemoveAll(b.CertsPath)
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
	CertsPath   string   `json:"cert_path"`
}

func NewBinary(packagePath, binsPath string, extraArgs []string, certsPath string) Binary {
	paths := strings.Split(packagePath, "/")
	binName := paths[len(paths)-1]

	return Binary{
		PackagePath: packagePath,
		BinPath:     filepath.Join(binsPath, binName),
		ExtraArgs:   extraArgs,
		LocksDir:    filepath.Join(binsPath, ".locks"),
		CertsPath:   certsPath,
	}
}

func (b *Binary) Run(config interface{}, envVars ...string) (*gexec.Session, string) {
	configBytes, err := yaml.Marshal(config)
	Expect(err).NotTo(HaveOccurred())

	var configFile string
	if config != nil {
		configFile = tests.WriteTempFile(configBytes, filepath.Base(b.BinPath)+"-config.yaml")
	}

	defaultEnv := []string{
		fmt.Sprintf("%s=%s", eirini.EnvCCCertDir, b.CertsPath),
		fmt.Sprintf("%s=%s", eirini.EnvServerCertDir, b.CertsPath),
	}
	env := append(defaultEnv, envVars...)

	return b.RunWithConfig(configFile, env...), configFile
}

func (b *Binary) RunWithConfig(configFilePath string, envVars ...string) *gexec.Session {
	b.buildIfNecessary()

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

	return b.RunWithConfig(configFilePath, envVars...)
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
