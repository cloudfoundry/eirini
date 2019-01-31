package lager_test

import (
	"encoding/json"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RedactingSink", func() {
	var (
		sink     lager.Sink
		testSink *lagertest.TestSink
	)

	BeforeEach(func() {
		testSink = lagertest.NewTestSink()

		var err error
		sink, err = lager.NewRedactingSink(testSink, nil, nil)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when given a valid set of data", func() {
		BeforeEach(func() {
			sink.Log(lager.LogFormat{
				LogLevel: lager.INFO,
				Message:  "hello world",
				Data:     lager.Data{"password": "abcd"},
			})
		})

		It("writes to the given sink", func() {
			Expect(testSink.Buffer().Contents()).To(MatchJSON(`{"timestamp":"","log_level":1,"source":"","message":"hello world","data":{"password":"*REDACTED*"}}`))
		})
	})

	Context("when an unserializable data object is passed in", func() {
		BeforeEach(func() {
			sink.Log(lager.LogFormat{
				LogLevel: lager.INFO,
				Message:  "hello world", Data: map[string]interface{}{
					"some_key": func() {},
				},
			})
		})

		It("logs the serialization error", func() {
			message := map[string]interface{}{}

			err := json.Unmarshal(testSink.Buffer().Contents(), &message)
			Expect(err).NotTo(HaveOccurred())

			Expect(message["message"]).To(Equal("hello world"))
			Expect(message["log_level"]).To(Equal(float64(1)))
			Expect(message["data"].(map[string]interface{})["lager serialisation error"]).To(Equal("json: unsupported type: func()"))
			Expect(message["data"].(map[string]interface{})["data_dump"]).ToNot(BeEmpty())
		})
	})
})
