package tests

import (
	"fmt"
	"io"
	"os/exec"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

type TelepresenceRunner struct {
	session *gexec.Session
	stdin   io.WriteCloser
}

// StartTelepresence creates a deployment and a service in the given namespace
// forwarding the remote port in kubernetes to the local port in the test machine.
// Port can be either 'LOCAL_PORT:REMOTE_PORT' or 'LOCAL_PORT'. In the latter case
// REMOTE_PORT is set to be the same as LOCAL_PORT
func StartTelepresence(namespace, serviceName, port string) (*TelepresenceRunner, error) {
	cmd := exec.Command("telepresence",
		"--namespace", namespace,
		"--new-deployment", serviceName,
		"--expose", port,
		"--method", "vpn-tcp",
		"--logfile", "-",
	)

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
