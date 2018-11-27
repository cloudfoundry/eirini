package img_test

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/buildpack/packs"
	"github.com/buildpack/packs/img"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/sclevine/spec"
)

func Test(t *testing.T) {
	spec.Run(t, "Actions", func(t *testing.T, when spec.G, it spec.S) {
		var (
			baseImage  v1.Image
			baseTag    string
			baseStore  img.Store
			imageTag   string
			imageStore img.Store
			assertNil  func(error)
		)
		it.Before(func() {
			assertNil = func(err error) {
				if err != nil {
					t.Helper()
					t.Fatal(err)
				}
			}
		})

		it.Before(func() {
			baseTag = randStringRunes(16)
			_, err := packs.Run("docker", "build", "-t", baseTag, "-f", "testdata/base.Dockerfile", "testdata")
			assertNil(err)
			baseStore, err = img.NewDaemon(baseTag)
			assertNil(err)
			baseImage, err = baseStore.Image()
			assertNil(err)

			imageTag = randStringRunes(16)
			imageStore, err = img.NewDaemon(imageTag)
			assertNil(err)
		})

		it.After(func() {
			_, err := packs.Run("docker", "rmi", "-f", baseTag, imageTag)
			assertNil(err)
		})

		when(".Append", func() {
			it("appends a tarball on top of a base image", func() {
				image, err := img.Append(baseImage, "testdata/some-appended-layer.tgz")
				assertNil(err)
				err = imageStore.Write(image)
				assertNil(err)
				output, err := packs.Run("docker", "run", "--rm", imageTag, "ls", "/layers/")
				layers := strings.Fields(output)
				if !reflect.DeepEqual(layers, []string{
					"some-appended-layer.txt",
					"some-layer-1.txt",
					"some-layer-2.txt",
					"some-layer-3.txt",
				}) {
					t.Fatalf(`Unexpected file contents "%s"`, layers)
				}
			})

			when("layer doesn't exist", func() {
				it("errors", func() {
					_, err := img.Append(baseImage, "testdata/some-missing-layer.tgz")
					if err == nil {
						t.Fatal("Expected an error")
					} else if !strings.Contains(err.Error(), "get layer from file:") {
						t.Fatalf(`Expected error to contain: "get layer from file:": got "%s"`, err)
					}
				})
			})

			when("append fails", func() {
				it.Before(func() {
					_, err := packs.Run("docker", "rmi", "-f", baseTag)
					assertNil(err)
				})
				it("errors", func() {
					_, err := img.Append(baseImage, "testdata/some-appended-layer.tgz")
					if err == nil {
						t.Fatal("Expected an error")
					} else if !strings.Contains(err.Error(), "append layer:") {
						t.Fatalf(`Expected error to container: "append layer": got "%s"`, err)
					}
				})
			})
		}, spec.Parallel())

		when(".Rebase", func() {
			var newBaseImage v1.Image
			var newBaseTag string
			var upperImage v1.Image
			var upperTag string

			it.Before(func() {
				newBaseTag = randStringRunes(16)
				_, err := packs.Run("docker", "build", "-t", newBaseTag, "-f", "testdata/newbase.Dockerfile", "testdata")
				assertNil(err)
				newBaseStore, err := img.NewDaemon(newBaseTag)
				assertNil(err)
				newBaseImage, err = newBaseStore.Image()
				assertNil(err)

				upperTag = randStringRunes(16)
				_, err = packs.Run("docker", "build", "--build-arg=base="+baseTag, "-t", upperTag, "-f", "testdata/upper.Dockerfile", "testdata")
				assertNil(err)
				upperStore, err := img.NewDaemon(upperTag)
				assertNil(err)
				upperImage, err = upperStore.Image()
				assertNil(err)
			})

			it("rebases image using labels", func() {
				image, err := img.Rebase(upperImage, newBaseImage, func(labels map[string]string) (v1.Image, error) {
					//TODO test labels
					return baseImage, nil
				})
				assertNil(err)

				if err := imageStore.Write(image); err != nil {
					t.Fatal(err)
				}
				output, err := packs.Run("docker", "run", "--rm", imageTag, "ls", "/layers/")
				layers := strings.Fields(output)
				if !reflect.DeepEqual(layers, []string{
					"some-new-base-layer.txt",
					"upper-layer-1.txt",
					"upper-layer-2.txt",
				}) {
					t.Fatalf(`Unexpected file contents "%s"`, layers)
				}
			})

			when("old base finder func errors", func() {
				it("errors", func() {
					_, err := img.Rebase(upperImage, newBaseImage, func(labels map[string]string) (v1.Image, error) {
						return nil, errors.New("old base finder error")
					})
					if err == nil {
						t.Fatal("Expected an error")
					} else if err.Error() != "find old base: old base finder error" {
						t.Fatalf(`Expected error to eqal: "find old base: old base finder error": got "%s"`, err)
					}
				})
			})

			when("rebase fails", func() {
				it("errors", func() {
					_, err := img.Rebase(upperImage, newBaseImage, func(labels map[string]string) (v1.Image, error) {
						return newBaseImage, nil
					})
					if err == nil {
						t.Fatal("Expected an error")
					} else if !strings.HasPrefix(err.Error(), "rebase image:") {
						t.Fatalf(`Expected error to have prefix: "rebase image:": got "%s"`, err)
					}
				})
			})
		})

		when(".Label", func() {
			it("adds labels to image", func() {
				image, err := img.Label(baseImage, "label1", "val1")
				assertNil(err)
				image, err = img.Label(image, "label2", "val2")
				assertNil(err)

				if err := imageStore.Write(image); err != nil {
					t.Fatal(err)
				}

				output, err := packs.Run("docker", "inspect", "--format={{.Config.Labels.label1}}", imageTag)
				assertNil(err)
				if output != "val1" {
					t.Errorf(`expected label1 to equal "val1", got "%s"`, output)
				}

				output, err = packs.Run("docker", "inspect", "--format={{.Config.Labels.label2}}", imageTag)
				assertNil(err)
				if output != "val2" {
					t.Errorf(`expected label2 to equal "val2", got "%s"`, output)
				}
			})
		})

		when.Focus(".SetupCredHelpers", func() {
			var tmpDir, dockerConfig string

			it.Before(func() {
				var err error
				tmpDir, err = ioutil.TempDir("", "actions-test")
				assertNil(err)
				dockerConfig = filepath.Join(tmpDir, "docker-home", "docker-config.json")
			})

			it.After(func() {
				err := os.RemoveAll(tmpDir)
				assertNil(err)
			})

			when("config does NOT exist", func() {
				it("add appropriate cred helpers for any repo matching *gcr.io, *.amazonaws.*, or *azurecf.io", func() {
					gcrRef1 := "gcr.io/some-org/some-image:some-tag"
					gcrRef2 := "some.gcr.io/some-org/some-image:some-tag"
					ecrRef := "some.amazonaws.some-tld/some-org/some-image:sha256some-sha"
					azureRef1 := "azurecr.io/some-org/some-image:sha256some-sha"
					azureRef2 := "some.azurecr.io/some-org/some-image:sha256some-sha"
					otherRef := "some-repo/some-image:some-tag"

					err := img.SetupCredHelpers(dockerConfig, gcrRef1, gcrRef2, ecrRef, azureRef1, azureRef2, otherRef)
					assertNil(err)
					contents, err := ioutil.ReadFile(dockerConfig)
					assertNil(err)
					config := struct {
						CredHelpers map[string]string
					}{}
					err = json.Unmarshal(contents, &config)
					assertNil(err)
					if config.CredHelpers["gcr.io"] != "gcr" {
						t.Fatalf(`expected cred helpers to contain gcr.io:gcr, got %+v`, config.CredHelpers)
					}
					if config.CredHelpers["some.gcr.io"] != "gcr" {
						t.Fatalf(`expected cred helpers to contain some.gcr.io:gcr, got %+v`, config.CredHelpers)
					}
					if config.CredHelpers["azurecr.io"] != "acr" {
						t.Fatalf(`expected cred helpers to contain azurecr.io:acr, got %+v`, config.CredHelpers)
					}
					if config.CredHelpers["some.azurecr.io"] != "acr" {
						t.Fatalf(`expected cred helpers to contain some.azurecr.io:acr, got %+v`, config.CredHelpers)
					}
					if config.CredHelpers["some.amazonaws.some-tld"] != "ecr-login" {
						t.Fatalf(`expected cred helpers to contain some.amazonaws.some-tld:ecr-login, got %+v`, config.CredHelpers)
					}
					if len(config.CredHelpers) > 5 {
						t.Fatalf(`added unexpected cred helpers %+v`, config.CredHelpers)
					}
				})

				when("ref is bad", func() {
					it("errors", func() {
						err := img.SetupCredHelpers(dockerConfig, ":some-bad-ref")
						if err == nil {
							t.Fatalf(`expected an error because of bad ref`)
						}
					})
				})
			})

			when("config does exist", func() {
				it.Before(func() {
					b, err := json.Marshal(map[string]interface{}{
						"credHelpers": map[string]string{"domain.com": "helper"},
						"aKey":        "a_val",
					})
					assertNil(err)
					os.MkdirAll(filepath.Dir(dockerConfig), 0755)
					err = ioutil.WriteFile(dockerConfig, b, 0644)
					assertNil(err)
				})

				it("leaves other keys unchanged, and overwrites CredHelpers", func() {
					gcrRef := "gcr.io/some-org/some-image:some-tag"

					err := img.SetupCredHelpers(dockerConfig, gcrRef)
					assertNil(err)

					contents, err := ioutil.ReadFile(dockerConfig)
					assertNil(err)
					config := map[string]interface{}{}
					err = json.Unmarshal(contents, &config)
					assertNil(err)
					if !reflect.DeepEqual(config["credHelpers"], map[string]interface{}{"gcr.io": "gcr", "domain.com": "helper"}) {
						t.Fatalf(`expected cred helpers to contain gcr.io:gcr, got %+v`, config)
					}
					if config["aKey"] != "a_val" {
						t.Fatalf(`expected config to container a_key to have value a_val, got %+s`, config)
					}
				})
			})
		})
	}, spec.Parallel())
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz")

func randStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}
