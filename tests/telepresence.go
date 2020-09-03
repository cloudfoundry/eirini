package tests

import (
	"fmt"
	"io"
	"os/exec"
	"strconv"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

type TelepresenceRunner struct {
	session *gexec.Session
	stdin   io.WriteCloser
}

// StartTelepresence creates a deployment and a service in the default namespace
// forwarding the defined remote ports in kubernetes to the local ports in the test machine.
// The number of exposed ports are defined by the totalPorts. The actual exported ports are
// startingPort, startingPort + 1, ..., startingPort + totalPorts - 1 .
func StartTelepresence(serviceName string, startingPort int, totalPorts int) (*TelepresenceRunner, error) {
	args := []string{
		"--new-deployment", serviceName,
		"--method", "vpn-tcp",
		"--logfile", "-",
	}
	for i := 0; i < totalPorts; i++ {
		args = append(args, "--expose", strconv.Itoa(startingPort+i))
	}

	cmd := exec.Command("telepresence", args...)

	// Telepresence needs something to run, and will run a shell if nothing specified.
	// We need to have an open stdin to stop the shell exiting
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	session, err := gexec.Start(cmd, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	if err != nil {
		stdin.Close()

		return nil, err
	}

	// Once the shell is responding, the tunnel is open
	fmt.Fprintln(stdin, "echo ready")
	gomega.Eventually(session, "10s").Should(gbytes.Say("ready"))
	gomega.Consistently(session.Exited).ShouldNot(gomega.Receive())

	return &TelepresenceRunner{
		session: session,
		stdin:   stdin,
	}, nil
}

// Stop closes the Telepresence tunnel (by closing the stdin to the shell)
func (t *TelepresenceRunner) Stop() {
	t.stdin.Close()
	gomega.Eventually(t.session, "5s").Should(gexec.Exit())
}
