package sink_test

import (
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"github.com/julz/cube/sink"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Convert CC DesiredApp into an opi LRP", func() {
	It("Directly converts DockerImageURL, Instances fields", func() {
		lrp := sink.Convert(cc_messages.DesireAppRequestFromCC{
			DockerImageUrl: "the-image-url",
			NumInstances:   3,
		})

		Expect(lrp.Image).To(Equal("the-image-url"))
		Expect(lrp.TargetInstances).To(Equal(3))
	})

	// a processGuid is <app-guid>-<version-guid>
	It("truncates the ProcessGuid so it only contain the app guid", func() {
		lrp := sink.Convert(cc_messages.DesireAppRequestFromCC{
			ProcessGuid: "b194809b-88c0-49af-b8aa-69da097fc360-2fdc448f-6bac-4085-9426-87d0124c433a",
		})

		Expect(lrp.Name).To(Equal("b194809b-88c0-49af-b8aa-69da097fc360"))
	})

	PIt("Converts droplet apps via the special registry URL", func() {
		lrp := sink.Convert(cc_messages.DesireAppRequestFromCC{
			ProcessGuid: "the-process-guid",
		})

		Expect(lrp.Image).To(Equal("cube-registry.service.cf.internal/cloudfoundry/app-guid:the-process-guid"))
	})
})
