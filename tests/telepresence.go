package tests

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"

	// nolint:golint,stylecheck
	. "github.com/onsi/ginkgo"

	// nolint:golint,stylecheck
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

const (
	// Think if we need to somehow merge with NextAvailablePort. For now make sure
	// they don't collide.
	startPort = 10000
)

type TelepresenceRunner struct {
	session *gexec.Session
	stdin   io.WriteCloser
}

// StartTelepresence creates a deployment and a service in the default namespace
// forwarding the defined remote ports in kubernetes to the local ports in the test machine.
// The number of exposed ports are defined by the totalPorts. The actual exported ports are
// startingPort, startingPort + 1, ..., startingPort + totalPorts - 1 .
func StartTelepresence(serviceName string, totalPorts int) (*TelepresenceRunner, error) {
	args := []string{
		"--new-deployment", serviceName,
		"--method", "vpn-tcp",
		"--logfile", "-",
	}
	for i := 0; i < totalPorts; i++ {
		args = append(args, "--expose", strconv.Itoa(startPort+i))
	}

	cmd := exec.Command("telepresence", args...)

	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", GetKubeconfig()))

	// Telepresence needs something to run, and will run a shell if nothing specified.
	// We need to have an open stdin to stop the shell exiting
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	if err != nil {
		stdin.Close()

		return nil, err
	}

	RetryResolveHost("kubernetes.default.svc.cluster.local",
		"Looks like telepresence is not running!")

	return &TelepresenceRunner{
		session: session,
		stdin:   stdin,
	}, nil
}

// Stop closes the Telepresence tunnel (by closing the stdin to the shell)
func (t *TelepresenceRunner) Stop() {
	t.stdin.Close()
	Eventually(t.session, "60s").Should(gexec.Exit())
}

func GetTelepresencePort() int {
	return startPort + GinkgoParallelNode() - 1
}
