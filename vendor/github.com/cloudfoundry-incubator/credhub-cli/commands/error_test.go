package commands_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Errors", func() {
	It("prints newline after error messages", func() {
		session := runCommand("bogus-command")

		Eventually(session).Should(Exit(1))
		Eventually(session.Err.Contents()).Should(MatchRegexp("\n$"))
	})
})
