package sink_test

import (
	"context"
	"net/http"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"github.com/julz/cube"
	"github.com/julz/cube/opi"
	"github.com/julz/cube/sink"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Converge", func() {
	It("simply converts and desires every single CCRequest to a LRP", func() {
		converted := make([]cc_messages.DesireAppRequestFromCC, 0)
		desired := make([][]opi.LRP, 0)
		converger := sink.Converger{
			Converter: sink.ConvertFunc(func(
				msg cc_messages.DesireAppRequestFromCC,
				regUrl string,
				regIP string,
				cfClient cube.CfClient,
				client *http.Client,
				log lager.Logger,
			) opi.LRP {
				converted = append(converted, msg)
				return opi.LRP{Image: msg.DockerImageUrl}
			}),
			Desirer: opi.DesireFunc(func(_ context.Context, lrps []opi.LRP) error {
				desired = append(desired, lrps)
				return nil
			}),
		}

		err := converger.ConvergeOnce(context.Background(), []cc_messages.DesireAppRequestFromCC{
			{
				DockerImageUrl: "msg1",
			},
			{
				DockerImageUrl: "msg2",
			},
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(converted).To(HaveLen(2))
		Expect(desired).To(HaveLen(1))
		Expect(desired[0]).To(HaveLen(2))
		Expect(desired[0][0].Image).To(Equal("msg1"))
		Expect(desired[0][1].Image).To(Equal("msg2"))
	})
})
