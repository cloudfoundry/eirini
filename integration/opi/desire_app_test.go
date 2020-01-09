package opi_test

import (
	"bytes"
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Desire App", func() {
	var body string

	BeforeEach(func() {
		body = `{
			"guid": "the-app-guid",
			"version": "0.0.0",
			"ports" : [8080],
		  "lifecycle": {
				"docker_lifecycle": {
				  "image": "foo",
					"command": ["bar", "baz"]
				}
			}
		}`
	})

	JustBeforeEach(func() {
		desireAppReq, err := http.NewRequest("PUT", fmt.Sprintf("%s/apps/the-app-guid", url), bytes.NewReader([]byte(body)))
		Expect(err).NotTo(HaveOccurred())
		resp, err := httpClient.Do(desireAppReq)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusAccepted))
	})

	It("should create a stateful set for the app", func() {
		statefulsets, err := clientset.AppsV1().StatefulSets(namespace).List(metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())

		Expect(statefulsets.Items).To(HaveLen(1))
		Expect(statefulsets.Items[0].Name).To(ContainSubstring("the-app-guid"))
	})

	Context("when the app has user defined annotations", func() {
		BeforeEach(func() {
			body = `{
			"guid": "the-app-guid",
			"version": "0.0.0",
			"ports" : [8080],
		  "lifecycle": {
				"docker_lifecycle": {
				  "image": "foo",
					"command": ["bar", "baz"]
				}
			},
			"user_defined_annotations": {
			  "prometheus.io/scrape": "yes, please"
			}
		}`
		})

		It("should set the annotations to the pod template", func() {
			statefulsets, err := clientset.AppsV1().StatefulSets(namespace).List(metav1.ListOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(statefulsets.Items[0].Spec.Template.Annotations).To(HaveKeyWithValue("prometheus.io/scrape", "yes, please"))
		})
	})
})
