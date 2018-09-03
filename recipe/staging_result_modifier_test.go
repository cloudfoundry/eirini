package recipe_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	bap "code.cloudfoundry.org/buildpackapplifecycle"
	. "code.cloudfoundry.org/eirini/recipe"
)

var _ = Describe("BuildpacksKeyModifier", func() {

	var (
		modifier       BuildpacksKeyModifier
		modifiedResult bap.StagingResult
		providedResult bap.StagingResult
		err            error
	)

	Context("When modifying the staging result", func() {

		BeforeEach(func() {
			providedBuildpacks := `
			[{
				"name":"java_buildpack",
				"key":"java-buildpack-key-420"
			},
			{
				"name":"ruby_buildpack",
				"key":"ruby-buildpack-key-42"
			}]`

			providedResult = bap.StagingResult{
				LifecycleMetadata: bap.LifecycleMetadata{
					BuildpackKey: "ruby_buildpack",
					Buildpacks: []bap.BuildpackMetadata{
						{Key: "ruby_buildpack"},
						{Key: "java_buildpack"},
					},
				},
			}

			modifier = BuildpacksKeyModifier{
				CCBuildpacksJSON: providedBuildpacks,
			}
		})

		JustBeforeEach(func() {
			modifiedResult, err = modifier.Modify(providedResult)
		})

		It("should not return an error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should replace the buildpack keys with the ones provided by CC", func() {
			Expect(modifiedResult).To(Equal(bap.StagingResult{
				LifecycleMetadata: bap.LifecycleMetadata{
					BuildpackKey: "ruby-buildpack-key-42",
					Buildpacks: []bap.BuildpackMetadata{
						{Key: "ruby-buildpack-key-42"},
						{Key: "java-buildpack-key-420"},
					},
				},
			}))

		})

		Context("When the CCBuildpacksJSON is an invalid json", func() {
			BeforeEach(func() {
				invalidJSON := "{very invalid json"
				modifier = BuildpacksKeyModifier{
					CCBuildpacksJSON: invalidJSON,
				}
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		Context("When staging result's buildpack_key is not available in CC", func() {

			BeforeEach(func() {
				providedResult = bap.StagingResult{
					LifecycleMetadata: bap.LifecycleMetadata{
						BuildpackKey: "wat is this",
						Buildpacks: []bap.BuildpackMetadata{
							{Key: "ruby_buildpack"},
							{Key: "java_buildpack"},
						},
					},
				}
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		Context("When staging result's buildpacks metadata key  is not available in CC", func() {

			BeforeEach(func() {
				providedResult = bap.StagingResult{
					LifecycleMetadata: bap.LifecycleMetadata{
						BuildpackKey: "ruby_buildpack",
						Buildpacks: []bap.BuildpackMetadata{
							{Key: "ruby_buildpack"},
							{Key: "build wat"},
						},
					},
				}
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})

})
