package st8ger_test

import (
	"context"

	"github.com/cloudfoundry-incubator/eirini"
	"github.com/cloudfoundry-incubator/eirini/opi"
	. "github.com/cloudfoundry-incubator/eirini/st8ger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Stager", func() {
	Context("Run", func() {

		var (
			task   opi.Task
			st8ger eirini.St8ger
		)

		BeforeEach(func() {
			task = opi.Task{}
			st8ger = St8ger{
				Desirer: opi.DesireTaskFunc(func(_ context.Context, tasks []opi.Task) error {
					return nil
				}),
			}
		})

		It("converts and desires a staging request to a Task", func() {
			err := st8ger.Run(task)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
