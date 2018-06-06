package bifrost_test

import (
	"context"
	"errors"
	"net/http"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/bifrost"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/eirini/opi/opifakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Bifrost", func() {
	Context("Transfer", func() {
		It("simply converts and desires every single CCRequest to a LRP", func() {
			converted := make([]cc_messages.DesireAppRequestFromCC, 0)
			desired := make([][]opi.LRP, 0)
			bifrost := bifrost.Bifrost{
				Converter: bifrost.ConvertFunc(func(
					msg cc_messages.DesireAppRequestFromCC,
					regUrl string,
					regIP string,
					cfClient eirini.CfClient,
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

			err := bifrost.Transfer(context.Background(), []cc_messages.DesireAppRequestFromCC{
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

	Context("List", func() {
		var (
			opiClient *opifakes.FakeDesirer
			lager     lager.Logger
			bfrst     bifrost.Bifrost
		)

		BeforeEach(func() {
			opiClient = new(opifakes.FakeDesirer)
			lager = lagertest.NewTestLogger("bifrost-test")
			bfrst = bifrost.Bifrost{
				Desirer: opiClient,
				Logger:  lager,
			}
		})

		Context("When listing running LRPs", func() {

			JustBeforeEach(func() {
				opiClient.ListReturns([]opi.LRP{
					opi.LRP{Name: "1234"},
					opi.LRP{Name: "5678"},
					opi.LRP{Name: "0213"},
				}, nil)
			})

			It("should translate []LRPs to []DesiredLRPSchedulingInfo", func() {
				desiredLRPSchedulingInfos, err := bfrst.List(context.Background())
				Expect(err).ToNot(HaveOccurred())

				Expect(desiredLRPSchedulingInfos[0].ProcessGuid).To(Equal("1234"))
				Expect(desiredLRPSchedulingInfos[1].ProcessGuid).To(Equal("5678"))
				Expect(desiredLRPSchedulingInfos[2].ProcessGuid).To(Equal("0213"))
			})
		})

		Context("When an error occurs", func() {

			JustBeforeEach(func() {
				opiClient.ListReturns(nil, errors.New("arrgh"))
			})

			It("should return a meaningful errormessage", func() {
				_, err := bfrst.List(context.Background())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).Should(ContainSubstring("failed to list desired LRPs"))
			})
		})
	})
})
