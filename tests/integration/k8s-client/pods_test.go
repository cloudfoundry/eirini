package integration_test

import (
	"code.cloudfoundry.org/eirini/k8s/client"
	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Pod", func() {
	var podClient *client.Pod

	BeforeEach(func() {
		podClient = client.NewPod(fixture.Clientset, "")
	})

	Describe("GetAll", func() {
		var extraNs string

		BeforeEach(func() {
			extraNs = fixture.CreateExtraNamespace()

			createLrpPods(fixture.Namespace, "one", "two", "three")
			createTaskPods(extraNs, "four", "five", "six")
			createPod(extraNs, "sadpod", map[string]string{})
		})

		It("lists all eirini pods across all namespaces", func() {
			Eventually(func() []string {
				pods, err := podClient.GetAll(ctx)
				Expect(err).NotTo(HaveOccurred())

				return podNames(pods)
			}).Should(SatisfyAll(
				ContainElements("one", "two", "three", "four", "five", "six"),
				Not(ContainElement("sadpod")),
			))
		})

		When("the workloads namespace is set", func() {
			BeforeEach(func() {
				podClient = client.NewPod(fixture.Clientset, fixture.Namespace)
			})

			It("lists eirini pods from the configured namespace only", func() {
				Eventually(func() []string {
					pods, err := podClient.GetAll(ctx)
					Expect(err).NotTo(HaveOccurred())

					return podNames(pods)
				}).Should(SatisfyAll(
					ContainElements("one", "two", "three"),
					Not(ContainElements("four", "five", "six", "sadpod")),
				))
			})
		})
	})

	Describe("GetByLRPIdentifier", func() {
		var guid, extraNs string

		BeforeEach(func() {
			createLrpPods(fixture.Namespace, "one", "two", "three")

			guid = tests.GenerateGUID()

			createPod(fixture.Namespace, "four", map[string]string{
				stset.LabelGUID:    guid,
				stset.LabelVersion: "42",
			})
			createPod(fixture.Namespace, "five", map[string]string{
				stset.LabelGUID:    guid,
				stset.LabelVersion: "42",
			})

			extraNs = fixture.CreateExtraNamespace()

			createPod(extraNs, "six", map[string]string{
				stset.LabelGUID:    guid,
				stset.LabelVersion: "42",
			})
		})

		It("lists all pods matching the specified LRP identifier", func() {
			pods, err := podClient.GetByLRPIdentifier(ctx, opi.LRPIdentifier{GUID: guid, Version: "42"})

			Expect(err).NotTo(HaveOccurred())
			Eventually(func() []string { return podNames(pods) }).Should(ConsistOf("four", "five", "six"))
		})
	})

	Describe("Delete", func() {
		BeforeEach(func() {
			createLrpPods(fixture.Namespace, "foo")
		})

		It("deletes a pod", func() {
			Eventually(func() []string { return podNames(listAllPods()) }).Should(ContainElement("foo"))

			err := podClient.Delete(ctx, fixture.Namespace, "foo")

			Expect(err).NotTo(HaveOccurred())
			Eventually(func() []string { return podNames(listAllPods()) }).ShouldNot(ContainElement("foo"))
		})

		Context("when it fails", func() {
			It("returns the error", func() {
				err := podClient.Delete(ctx, fixture.Namespace, "bar")

				Expect(err).To(MatchError(ContainSubstring(`"bar" not found`)))
			})
		})
	})

	Describe("SetAnnotation", func() {
		var (
			key            string
			value          string
			oldPod, newPod *corev1.Pod
			err            error
		)

		BeforeEach(func() {
			key = "foo"
			value = "bar"

			createLrpPods(fixture.Namespace, "foo")
			oldPod = getPod(fixture.Namespace, "foo")
		})

		JustBeforeEach(func() {
			newPod, err = podClient.SetAnnotation(ctx, oldPod, key, value)
		})

		It("succeeds", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("sets a pod annotation", func() {
			Expect(newPod.Annotations["foo"]).To(Equal("bar"))
		})

		It("preserves existing annotations", func() {
			Expect(newPod.Annotations["some"]).To(Equal("annotation"))
		})

		When("setting an existing annotation", func() {
			BeforeEach(func() {
				key = "some"
			})

			It("overrides that annotation", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(newPod.Annotations["some"]).To(Equal("bar"))
			})
		})

		When("pod was updated since being read", func() {
			BeforeEach(func() {
				_, err = podClient.SetAnnotation(ctx, oldPod, "foo", "something-else")
				Expect(err).NotTo(HaveOccurred())
			})

			It("overwrites the change without failing", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(newPod.Annotations["foo"]).To(Equal("bar"))
			})
		})
	})

	Describe("SetAndTestAnnotation", func() {
		var (
			key            string
			value          string
			prevValue      *string
			oldPod, newPod *corev1.Pod
			err            error
		)

		BeforeEach(func() {
			key = "foo"
			value = "bar"
			prevValue = nil

			createLrpPods(fixture.Namespace, "foo")
			oldPod = getPod(fixture.Namespace, "foo")
		})

		JustBeforeEach(func() {
			newPod, err = podClient.SetAndTestAnnotation(ctx, oldPod, key, value, prevValue)
		})

		It("succeeds", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("sets a pod annotation", func() {
			Expect(newPod.Annotations["foo"]).To(Equal("bar"))
		})

		It("preserves existing annotations", func() {
			Expect(newPod.Annotations["some"]).To(Equal("annotation"))
		})

		When("setting an existing annotation", func() {
			BeforeEach(func() {
				key = "some"
				prevValueStr := "annotation"
				prevValue = &prevValueStr
			})

			It("overrides that annotation", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(newPod.Annotations["some"]).To(Equal("bar"))
			})
		})

		When("the previous value doesn't match that supplied", func() {
			BeforeEach(func() {
				key = "some"
				prevValueStr := "notTheValue"
				prevValue = &prevValueStr
			})

			It("fails", func() {
				Expect(err).To(MatchError(ContainSubstring("the server rejected")))
			})
		})
	})
})
