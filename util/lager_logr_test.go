package util_test

import (
	"errors"
	"fmt"

	"code.cloudfoundry.org/eirini/util"
	"code.cloudfoundry.org/eirini/util/utilfakes"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

//counterfeiter:generate -o utilfakes/fake_logger.go code.cloudfoundry.org/lager.Logger

var _ = Describe("LagerLogr", func() {
	var (
		fakeLogger *utilfakes.FakeLogger
		lagerLogr  logr.Logger
	)

	BeforeEach(func() {
		fakeLogger = new(utilfakes.FakeLogger)
		lagerLogr = util.NewLagerLogr(fakeLogger)
	})

	It("is always enabled", func() {
		Expect(lagerLogr.Enabled()).To(BeTrue())
	})

	Describe("Info", func() {
		JustBeforeEach(func() {
			lagerLogr.Info("some-message", "some-data")
		})

		It("delegates to lagger's Info", func() {
			Expect(fakeLogger.InfoCallCount()).To(Equal(1))
			actualMsg, actualLagerData := fakeLogger.InfoArgsForCall(0)
			Expect(actualMsg).To(Equal("some-message"))
			Expect(actualLagerData).To(HaveLen(1))
			Expect(fmt.Sprintf("%s", actualLagerData[0]["data"])).To(ContainSubstring("some-data"))
		})
	})

	Describe("Error", func() {
		JustBeforeEach(func() {
			lagerLogr.Error(errors.New("some-error"), "some-message", "some-data")
		})

		It("delegates to lagger's Error", func() {
			Expect(fakeLogger.ErrorCallCount()).To(Equal(1))
			actualMsg, actualErr, actualLagerData := fakeLogger.ErrorArgsForCall(0)
			Expect(actualErr).To(MatchError("some-error"))
			Expect(actualMsg).To(Equal("some-message"))
			Expect(actualLagerData).To(HaveLen(1))
			Expect(fmt.Sprintf("%s", actualLagerData[0]["data"])).To(ContainSubstring("some-data"))
		})
	})

	Describe("V", func() {
		JustBeforeEach(func() {
			lagerLogr.V(0).Info("some-message", "some-data")
		})

		It("delegates to lagger's Info", func() {
			Expect(fakeLogger.InfoCallCount()).To(Equal(1))
			actualMsg, actualLagerData := fakeLogger.InfoArgsForCall(0)
			Expect(actualMsg).To(Equal("some-message"))
			Expect(actualLagerData).To(HaveLen(1))
			Expect(fmt.Sprintf("%s", actualLagerData[0]["data"])).To(ContainSubstring("some-data"))
		})
	})

	Describe("WithValues", func() {
		It("returns the same logger", func() {
			Expect(lagerLogr.WithValues()).To(BeIdenticalTo(lagerLogr))
		})
	})

	Describe("WithName", func() {
		It("returns the same logger", func() {
			Expect(lagerLogr.WithName("foo")).To(BeIdenticalTo(lagerLogr))
		})
	})
})
