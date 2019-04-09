package recipe_test

import (
	"crypto/md5"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"strconv"

	"code.cloudfoundry.org/eirini/recipe"
	"code.cloudfoundry.org/urljoiner"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/tlsconfig"
	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("StagingText", func() {

	const (
		stagingGUID        = "staging-guid"
		completionCallback = ""
		responseURL        = "/stage/staging-guid/completed"
	)

	var (
		err            error
		server         *ghttp.Server
		appbitBytes    []byte
		buildpackBytes []byte
		session        *gexec.Session
		buildpacks     []recipe.Buildpack
		buildpacksDir  string
		workspaceDir   string
		outputDir      string
		cacheDir       string
		certsPath      string
		tlsConfig      *tls.Config
		buildpackJSON  []byte
		actualBytes    []byte
		expectedBytes  []byte
	)

	BeforeEach(func() {

		workspaceDir, err = ioutil.TempDir("", "workspace")
		Expect(err).NotTo(HaveOccurred())
		err = os.Setenv(eirini.EnvWorkspaceDir, workspaceDir)
		Expect(err).NotTo(HaveOccurred())

		err = chownR(workspaceDir, "vcap", "vcap")
		Expect(err).NotTo(HaveOccurred())

		outputDir, err = ioutil.TempDir("", "out")
		Expect(err).NotTo(HaveOccurred())
		err = os.Setenv(eirini.EnvOutputDropletLocation, path.Join(outputDir, "droplet.tgz"))
		Expect(err).NotTo(HaveOccurred())
		err = os.Setenv(eirini.EnvOutputMetadataLocation, path.Join(outputDir, "result.json"))
		Expect(err).NotTo(HaveOccurred())

		err = chownR(outputDir, "vcap", "vcap")
		Expect(err).NotTo(HaveOccurred())

		cacheDir, err = ioutil.TempDir("", "cache")
		Expect(err).NotTo(HaveOccurred())
		err = os.Setenv(eirini.EnvOutputBuildArtifactsCache, path.Join(cacheDir, "cache.tgz"))
		Expect(err).NotTo(HaveOccurred())

		err = chownR(cacheDir, "vcap", "vcap")
		Expect(err).NotTo(HaveOccurred())

		buildpacksDir, err = ioutil.TempDir("", "buildpacks")
		Expect(err).NotTo(HaveOccurred())
		err = os.Setenv(eirini.EnvBuildpacksDir, buildpacksDir)
		Expect(err).NotTo(HaveOccurred())

		err = chownR(buildpacksDir, "vcap", "vcap")
		Expect(err).NotTo(HaveOccurred())

		err = os.Setenv(eirini.EnvStagingGUID, stagingGUID)
		Expect(err).NotTo(HaveOccurred())

		err = os.Setenv(eirini.EnvCompletionCallback, completionCallback)
		Expect(err).NotTo(HaveOccurred())

		certsPath, err = filepath.Abs("testdata/certs")
		Expect(err).NotTo(HaveOccurred())

		certPath := filepath.Join(certsPath, "cc-server-crt")
		keyPath := filepath.Join(certsPath, "cc-server-crt-key")
		caCertPath := filepath.Join(certsPath, "internal-ca-cert")

		tlsConfig, err = tlsconfig.Build(
			tlsconfig.WithInternalServiceDefaults(),
			tlsconfig.WithIdentityFromFile(certPath, keyPath),
		).Server(
			tlsconfig.WithClientAuthenticationFromFile(caCertPath),
		)
		Expect(err).NotTo(HaveOccurred())

		server = ghttp.NewUnstartedServer()
		server.HTTPTestServer.TLS = tlsConfig
	})

	AfterEach(func() {
		err = os.RemoveAll(buildpacksDir)
		Expect(err).ToNot(HaveOccurred())
		err = os.RemoveAll(workspaceDir)
		Expect(err).ToNot(HaveOccurred())
		err = os.RemoveAll(outputDir)
		Expect(err).ToNot(HaveOccurred())
		err = os.RemoveAll(cacheDir)
		Expect(err).ToNot(HaveOccurred())

		err = os.Unsetenv(eirini.EnvCertsPath)
		Expect(err).ToNot(HaveOccurred())
		err = os.Unsetenv(eirini.EnvStagingGUID)
		Expect(err).NotTo(HaveOccurred())
		err = os.Unsetenv(eirini.EnvCompletionCallback)
		Expect(err).NotTo(HaveOccurred())
		err = os.Unsetenv(eirini.EnvBuildpacksDir)
		Expect(err).NotTo(HaveOccurred())
		err = os.Unsetenv(eirini.EnvBuildpacks)
		Expect(err).ToNot(HaveOccurred())
		err = os.Unsetenv(eirini.EnvDownloadURL)
		Expect(err).ToNot(HaveOccurred())
		err = os.Unsetenv(eirini.EnvWorkspaceDir)
		Expect(err).ToNot(HaveOccurred())
		err = os.Unsetenv(eirini.EnvOutputDropletLocation)
		Expect(err).NotTo(HaveOccurred())
		err = os.Unsetenv(eirini.EnvOutputMetadataLocation)
		Expect(err).NotTo(HaveOccurred())
		err = os.Unsetenv(eirini.EnvOutputBuildArtifactsCache)
		Expect(err).NotTo(HaveOccurred())
		err = os.Unsetenv(eirini.EnvEiriniAddress)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when a droplet needs building...", func() {
		Context("download", func() {
			BeforeEach(func() {
				appbitBytes, err = ioutil.ReadFile("testdata/dora.zip")
				Expect(err).NotTo(HaveOccurred())

				buildpackBytes, err = ioutil.ReadFile("testdata/ruby-buildpack-cflinuxfs2-v1.7.35.zip")
				Expect(err).NotTo(HaveOccurred())

				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/my-buildpack"),
						ghttp.RespondWith(http.StatusOK, buildpackBytes),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/my-app-bits"),
						ghttp.RespondWith(http.StatusOK, appbitBytes),
					),
				)
				server.Start()

				err = os.Setenv(eirini.EnvEiriniAddress, server.URL())
				Expect(err).NotTo(HaveOccurred())

				err = os.Setenv(eirini.EnvDownloadURL, urljoiner.Join(server.URL(), "my-app-bits"))
				Expect(err).ToNot(HaveOccurred())

				buildpacks = []recipe.Buildpack{
					{
						Name: "app_buildpack",
						Key:  "app_buildpack",
						URL:  urljoiner.Join(server.URL(), "/my-buildpack"),
					},
				}

				buildpackJSON, err = json.Marshal(buildpacks)
				Expect(err).ToNot(HaveOccurred())

				err = os.Setenv(eirini.EnvBuildpacks, string(buildpackJSON))
				Expect(err).ToNot(HaveOccurred())

				err = os.Setenv(eirini.EnvCertsPath, certsPath)
				Expect(err).ToNot(HaveOccurred())
			})

			JustBeforeEach(func() {
				cmd := exec.Command(binaries.DownloaderPath)
				session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Eventually(session).Should(gexec.Exit())
			})

			It("runs successfully", func() {
				Expect(session.ExitCode()).To(BeZero())
			})

			It("installs the buildpack json", func() {
				expectedFile := filepath.Join(buildpacksDir, "config.json")
				Expect(expectedFile).To(BeARegularFile())

				actualBytes, err = ioutil.ReadFile(expectedFile)
				Expect(err).ToNot(HaveOccurred())

				var actualBuildpacks []recipe.Buildpack
				err = json.Unmarshal(actualBytes, &actualBuildpacks)
				Expect(err).ToNot(HaveOccurred())

				Expect(actualBuildpacks).To(Equal(buildpacks))
			})

			It("installs the binary buildpack", func() {
				md5Hash := fmt.Sprintf("%x", md5.Sum([]byte("app_buildpack")))
				expectedBuildpackPath := path.Join(buildpacksDir, md5Hash)
				Expect(expectedBuildpackPath).To(BeADirectory())
			})

			It("places the app bits in the workspace", func() {
				actualBytes, err = ioutil.ReadFile(path.Join(workspaceDir, eirini.AppBits))
				Expect(err).NotTo(HaveOccurred())
				expectedBytes, err = ioutil.ReadFile("testdata/dora.zip")
				Expect(err).NotTo(HaveOccurred())
				Expect(actualBytes).To(Equal(expectedBytes))
			})

			Context("fails", func() {
				BeforeEach(func() {
					buildpacks = []recipe.Buildpack{
						{
							Name: "app_buildpack",
							Key:  "app_buildpack",
							URL:  "bad-url",
						},
					}

					buildpackJSON, err = json.Marshal(buildpacks)
					Expect(err).ToNot(HaveOccurred())

					err = os.Setenv(eirini.EnvBuildpacks, string(buildpackJSON))
					Expect(err).ToNot(HaveOccurred())

					server.SetHandler(0,
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", responseURL),
							verifyResponse(true, "failed to request buildpack"),
						),
					)
				})

				It("should send completion response with a failure", func() {
					Expect(server.ReceivedRequests()).To(HaveLen(1))
				})

				It("should exit with non-zero exit code", func() {
					Expect(session.ExitCode).NotTo(BeZero())
				})
			})
		})

		Context("execute", func() {
			Context("when extract fails", func() {
				BeforeEach(func() {
					appbitBytes, err = ioutil.ReadFile("testdata/bad-dora.zip")
					Expect(err).NotTo(HaveOccurred())

					buildpackBytes, err = ioutil.ReadFile("testdata/ruby-buildpack-cflinuxfs2-v1.7.35.zip")
					Expect(err).NotTo(HaveOccurred())
					server.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/my-buildpack"),
							ghttp.RespondWith(http.StatusOK, buildpackBytes),
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/my-app-bits"),
							ghttp.RespondWith(http.StatusOK, appbitBytes),
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", responseURL),
							verifyResponse(true, "not a valid zip file"),
						),
					)
					server.Start()

					err = os.Setenv(eirini.EnvEiriniAddress, server.URL())
					Expect(err).NotTo(HaveOccurred())

					err = os.Setenv(eirini.EnvDownloadURL, urljoiner.Join(server.URL(), "my-app-bits"))
					Expect(err).ToNot(HaveOccurred())

					buildpacks = []recipe.Buildpack{
						{
							Name: "ruby_buildpack",
							Key:  "ruby_buildpack",
							URL:  urljoiner.Join(server.URL(), "/my-buildpack"),
						},
					}

					buildpackJSON, err = json.Marshal(buildpacks)
					Expect(err).ToNot(HaveOccurred())

					err = os.Setenv(eirini.EnvBuildpacks, string(buildpackJSON))
					Expect(err).ToNot(HaveOccurred())

					err = os.Setenv(eirini.EnvCertsPath, certsPath)
					Expect(err).ToNot(HaveOccurred())

					cmd := exec.Command(binaries.DownloaderPath)
					session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Eventually(session).Should(gexec.Exit())
					Expect(err).NotTo(HaveOccurred())

				})

				JustBeforeEach(func() {
					cmd := exec.Command(binaries.ExecutorPath)
					session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Eventually(session, 80).Should(gexec.Exit())
				})

				It("should send completion response with a failure", func() {
					Expect(session.ExitCode).NotTo(BeZero())
					Expect(server.ReceivedRequests()).To(HaveLen(3))
				})
			})

			Context("when extract succeeds", func() {
				BeforeEach(func() {
					appbitBytes, err = ioutil.ReadFile("testdata/dora.zip")
					Expect(err).NotTo(HaveOccurred())

					buildpackBytes, err = ioutil.ReadFile("testdata/ruby-buildpack-cflinuxfs2-v1.7.35.zip")
					Expect(err).NotTo(HaveOccurred())
					server.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/my-buildpack"),
							ghttp.RespondWith(http.StatusOK, buildpackBytes),
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/my-app-bits"),
							ghttp.RespondWith(http.StatusOK, appbitBytes),
						),
					)
					server.Start()

					err = os.Setenv(eirini.EnvEiriniAddress, server.URL())
					Expect(err).NotTo(HaveOccurred())

					err = os.Setenv(eirini.EnvDownloadURL, urljoiner.Join(server.URL(), "my-app-bits"))
					Expect(err).ToNot(HaveOccurred())

					buildpacks = []recipe.Buildpack{
						{
							Name: "ruby_buildpack",
							Key:  "ruby_buildpack",
							URL:  urljoiner.Join(server.URL(), "/my-buildpack"),
						},
					}

					buildpackJSON, err = json.Marshal(buildpacks)
					Expect(err).ToNot(HaveOccurred())

					err = os.Setenv(eirini.EnvBuildpacks, string(buildpackJSON))
					Expect(err).ToNot(HaveOccurred())

					err = os.Setenv(eirini.EnvCertsPath, certsPath)
					Expect(err).ToNot(HaveOccurred())

					cmd := exec.Command(binaries.DownloaderPath)
					session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Eventually(session).Should(gexec.Exit())
					Expect(err).NotTo(HaveOccurred())

				})

				JustBeforeEach(func() {
					cmd := exec.Command(binaries.ExecutorPath)
					session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Eventually(session, 80).Should(gexec.Exit())
				})

				It("should create the droplet and output metadata", func() {
					Expect(session.ExitCode()).To(BeZero())

					Expect(path.Join(outputDir, "droplet.tgz")).To(BeARegularFile())
					Expect(path.Join(outputDir, "result.json")).To(BeARegularFile())
				})

				Context("fails", func() {
					BeforeEach(func() {
						err = os.Setenv(eirini.EnvWorkspaceDir, filepath.Join(workspaceDir, "bad-workspace-dir"))
						Expect(err).NotTo(HaveOccurred())

						server.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("PUT", responseURL),
								verifyResponse(true, "failed to create droplet"),
							),
						)
					})

					It("should send completion response with a failure", func() {
						Expect(server.ReceivedRequests()).To(HaveLen(3))
					})

					It("should exit with non-zero exit code", func() {
						Expect(session.ExitCode).NotTo(BeZero())
					})
				})
			})
		})

		Context("upload", func() {
			BeforeEach(func() {
				appbitBytes, err = ioutil.ReadFile("testdata/dora.zip")
				Expect(err).NotTo(HaveOccurred())

				buildpackBytes, err = ioutil.ReadFile("testdata/ruby-buildpack-cflinuxfs2-v1.7.35.zip")
				Expect(err).NotTo(HaveOccurred())

				server.AppendHandlers(
					// Downloader
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/my-buildpack"),
						ghttp.RespondWith(http.StatusOK, buildpackBytes),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/my-app-bits"),
						ghttp.RespondWith(http.StatusOK, appbitBytes),
					),

					// Uploader
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/my-droplet"),
						ghttp.RespondWith(http.StatusOK, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", responseURL),
						ghttp.RespondWith(http.StatusOK, ""),
						ghttp.VerifyMimeType("application/json"),
						verifyResponse(false, "ruby"),
					),
				)

				server.Start()

				err = os.Setenv(eirini.EnvEiriniAddress, server.URL())
				Expect(err).NotTo(HaveOccurred())

				err = os.Setenv(eirini.EnvDropletUploadURL, urljoiner.Join(server.URL(), "my-droplet"))
				Expect(err).ToNot(HaveOccurred())

				err = os.Setenv(eirini.EnvDownloadURL, urljoiner.Join(server.URL(), "my-app-bits"))
				Expect(err).ToNot(HaveOccurred())

				buildpacks = []recipe.Buildpack{
					{
						Name: "ruby_buildpack",
						Key:  "ruby_buildpack",
						URL:  urljoiner.Join(server.URL(), "/my-buildpack"),
					},
				}

				buildpackJSON, err = json.Marshal(buildpacks)
				Expect(err).ToNot(HaveOccurred())

				err = os.Setenv(eirini.EnvBuildpacks, string(buildpackJSON))
				Expect(err).ToNot(HaveOccurred())

				err = os.Setenv(eirini.EnvCertsPath, certsPath)
				Expect(err).ToNot(HaveOccurred())

				cmd := exec.Command(binaries.DownloaderPath)
				session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Eventually(session).Should(gexec.Exit())
				Expect(err).NotTo(HaveOccurred())

				cmd = exec.Command(binaries.ExecutorPath)
				session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Eventually(session, 80).Should(gexec.Exit())
				Expect(err).NotTo(HaveOccurred())
			})

			JustBeforeEach(func() {
				cmd := exec.Command(binaries.UploaderPath)
				session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Eventually(session, 10).Should(gexec.Exit())
			})

			It("should successfully upload the droplet", func() {
				Expect(session.ExitCode()).To(BeZero())
			})

			Context("fails", func() {
				BeforeEach(func() {
					err = os.Setenv(eirini.EnvOutputDropletLocation, path.Join(outputDir, "bad-location.tgz"))
					Expect(err).NotTo(HaveOccurred())

					server.SetHandler(2,
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", responseURL),
							verifyResponse(true, "no such file"),
						),
					)
				})

				It("should send completion response with a failure", func() {
					Expect(server.ReceivedRequests()).To(HaveLen(3))
				})

				It("should return an error", func() {
					Expect(server.ReceivedRequests()).To(HaveLen(3))
					Expect(session.ExitCode).NotTo(BeZero())
				})
			})

			Context("and eirini returns response with failure status", func() {
				BeforeEach(func() {
					server.SetHandler(3,
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", responseURL),
							ghttp.RespondWith(http.StatusInternalServerError, ""),
						),
					)
				})

				It("should return an error", func() {
					Expect(server.ReceivedRequests()).To(HaveLen(4))
					Expect(session.ExitCode).NotTo(BeZero())
				})
			})
		})
	})
})

func verifyResponse(failed bool, reason string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {

		body, err := ioutil.ReadAll(req.Body)
		req.Body.Close()
		Expect(err).ShouldNot(HaveOccurred())

		var uploaderResponse models.TaskCallbackResponse
		err = json.Unmarshal(body, &uploaderResponse)
		Expect(err).NotTo(HaveOccurred())
		Expect(uploaderResponse.Failed).To(Equal(failed))
		if failed {
			Expect(uploaderResponse.FailureReason).To(ContainSubstring(reason))
		} else {
			Expect(uploaderResponse.Result).To(ContainSubstring(reason))
		}
	}
}

func chownR(path, username, group string) error {

	uid, gid, err := getIds(username, group)
	if err != nil {
		return err
	}

	return filepath.Walk(path, func(name string, info os.FileInfo, err error) error {
		if err == nil {
			err = os.Chown(name, uid, gid)
		}
		return err
	})
}

func getIds(username, group string) (uid int, gid int, err error) {

	var g *user.Group
	g, err = user.LookupGroup(group)
	if err != nil {
		return -1, -1, err
	}

	var u *user.User
	u, err = user.Lookup(username)
	if err != nil {
		return -1, -1, err
	}

	uid, err = strconv.Atoi(u.Uid)
	if err != nil {
		return -1, -1, err
	}

	gid, err = strconv.Atoi(g.Gid)
	if err != nil {
		return -1, -1, err
	}

	return uid, gid, nil
}
