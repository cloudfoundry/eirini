package blobondemand_test

import (
	"bytes"
	"io"

	. "code.cloudfoundry.org/eirini/blobondemand"
	"code.cloudfoundry.org/eirini/registry"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Blonde", func() {

	var (
		store registry.BlobStore
	)

	BeforeEach(func() {
		store = NewInMemoryStore()
	})

	Context("When putting a blob", func() {
		var (
			buf  io.Reader
			id   string
			size int64
			err  error
		)

		BeforeEach(func() {
			buf = bytes.NewReader([]byte("here-is-some-content"))
			id, size, err = store.Put(buf)
		})

		It("should not return an error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return the id and size", func() {
			Expect(id).To(Equal("sha256:e0c6189f72b0e909e963116fb71625186098e75a843abffc6f7f5ab53df8cdd3"))
			Expect(size).To(Equal(int64(20)))
		})

		It("should store it", func() {
			Expect(store.Has(id)).To(Equal(true))
		})

	})

	Context("When checking if a blob is available", func() {

		Context("when it is in the store", func() {

			var id string

			BeforeEach(func() {
				buf := bytes.NewReader([]byte("here-is-some-content"))
				var err error
				id, _, err = store.Put(buf)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should report it correctly", func() {
				Expect(store.Has(id)).To(Equal(true))
			})
		})

		Context("when it is NOT in the store", func() {
			It("should report it correctly", func() {
				Expect(store.Has("water")).To(Equal(false))
			})
		})
	})

	Context("When getting a blob", func() {
		var (
			outputBuf *gbytes.Buffer
			id        string
			err       error
		)

		JustBeforeEach(func() {
			outputBuf = gbytes.NewBuffer()
			err = store.Get(id, outputBuf)
		})

		Context("and the blob exists", func() {
			BeforeEach(func() {
				buf := bytes.NewReader([]byte("here-is-some-content"))
				id, _, err = store.Put(buf)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should write the blob", func() {
				Eventually(outputBuf).Should(gbytes.Say("here-is-some-content"))
			})
		})

		Context("and the blob does not exist", func() {
			BeforeEach(func() {
				id = "mr.blobby"
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should not modify the buffer", func() {
				Eventually(outputBuf).Should(gbytes.Say(""))
			})
		})
	})
})
