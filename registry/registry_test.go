package registry_test

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/julz/cube/blobondemand"
	"github.com/julz/cube/registry"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Registry", func() {
	var (
		ts           *httptest.Server
		dropletStore registry.InMemoryDropletStore
		blobStore    *blobondemand.InMemoryStore
	)

	BeforeEach(func() {
		blobStore = blobondemand.NewInMemoryStore()
		dropletStore = make(registry.InMemoryDropletStore)
	})

	JustBeforeEach(func() {
		ts = httptest.NewServer(registry.NewHandler(registry.BlobRef{Digest: "the-rootfs-digest"}, dropletStore, blobStore))
	})

	AfterEach(func() {
		ts.Close()
	})

	It("Serves the /v2 endpoint so that the client skips authentication", func() {
		// the docker client hits /v2 to see if it needs to authenticate, so
		// we need to reply 200 to tell it to continue
		res, err := http.Get(ts.URL + "/v2")
		Expect(err).NotTo(HaveOccurred())
		Expect(res.StatusCode).To(Equal(200))
	})

	// Right now the droplet in CC isn't quite what we want from a layer.
	// Firstly, it's not content-addressed, secondly it's rooted at /home/vcap
	// rather than the root of the filesystem. Ideally we'd fix the existing CC
	// stuff around this, but for now we just have this endpoint which takes a droplet
	// and creates a nice content-addressed layer in our local blobstore.
	Context("TEMPORARY: staging a droplet", func() {
		It("stores a proper layer blob based on a given droplet", func() {
			droplet, err := os.Open("testdata/droplet.tar.gz")
			Expect(err).NotTo(HaveOccurred())

			res := post(ts.URL+"/v2/a-space/an-app/blobs/?guid=the-droplet-guid", "application/cf.droplet+tar", droplet)
			Expect(res.StatusCode).To(Equal(201))

			digest := dropletStore.Get("the-droplet-guid").Digest
			Expect(digest).NotTo(BeEmpty(), "should have stored the droplet digest by its guid")

			// this sha happens to be the content-address of a properly translated droplet layer based on the given tar contents
			res = fetch(ts.URL + "/v2/a-space/an-app/blobs/" + digest)
			Expect(res.StatusCode).To(Equal(200))

			tmp, err := ioutil.TempDir("", "droplet2layertest")
			Expect(err).NotTo(HaveOccurred())

			cmd := exec.Command("tar", "xvf", "-", "-C", tmp)
			cmd.Stdin = res.Body
			Expect(cmd.Run()).To(Succeed())

			// Expect to download with the files under /home/vcap
			Expect(filepath.Join(tmp, "home", "vcap", "app", "Procfile")).To(BeAnExistingFile())
		})
	})

	Context("serving blobs", func() {
		It("serves the blob by its digest", func() {
			shaOfDroplet, _, err := blobStore.Put(strings.NewReader("the-droplet"))
			Expect(err).NotTo(HaveOccurred())

			res := fetch(ts.URL + "/v2/a-space/an-app/blobs/" + shaOfDroplet)

			Expect(res.StatusCode).To(Equal(200))
			Expect(ioutil.ReadAll(res.Body)).To(Equal([]byte("the-droplet")))

		})
	})

	Context("serving manifests for droplets", func() {
		Context("when the droplet guid is known", func() {
			BeforeEach(func() {
				dropletStore["<droplet-guid>"] = registry.BlobRef{Digest: "the-droplet-digest"}
			})

			It("replies with an OCI manifest", func() {
				res := fetch(ts.URL + "/v2/a-space/an-app/manifests/<droplet-guid>")
				Expect(res.StatusCode).To(Equal(200))
				Expect(res.Header).To(HaveKeyWithValue("Content-Type", []string{"application/vnd.docker.distribution.manifest.v2+json"}))
			})

			Describe("the OCI manifest", func() {
				var manifest map[string]interface{}

				JustBeforeEach(func() {
					res := fetch(ts.URL + "/v2/a-space/an-app/manifests/<droplet-guid>")
					Expect(res.StatusCode).To(Equal(200))
					Expect(json.NewDecoder(res.Body).Decode(&manifest)).To(Succeed())
				})

				Describe("the layers element", func() {
					It("should contain 2 layers", func() {
						Expect(manifest["layers"]).To(HaveLen(2))
					})

					Describe("the first layer", func() {
						It("should reference the current rootfs blob", func() {
							Expect(manifest["layers"]).To(
								ConsistOf(HaveKeyWithValue("digest", "the-rootfs-digest"), HaveKey("digest")))
						})
					})

					Describe("the second layer", func() {
						It("should reference the current requested droplet blob", func() {
							// note: the only way to get the-droplet-digest is to look it
							// up in the dropletStore
							Expect(manifest["layers"]).To(
								ConsistOf(HaveKey("digest"),
									HaveKeyWithValue("digest", "the-droplet-digest")))
						})
					})
				})

				Describe("the config element", func() {
					It("should reference a config blob", func() {
						Expect(manifest["config"]).To(HaveKeyWithValue("mediaType", "application/vnd.docker.container.image.v1+json"))
					})

					// this is a bit of a hack to allow us to dynamically create images just-in-time
					// other options would be to do this at staging time and have a convergence loop
					// to update existing blobs or an upstream registry
					It("should dynamically create an OCI config blob in the blobstore", func() {
						var buf bytes.Buffer
						blobStore.Get(manifest["config"].(map[string]interface{})["digest"].(string), &buf)

						var decoded map[string]interface{}
						Expect(json.NewDecoder(bytes.NewReader(buf.Bytes())).Decode(&decoded)).To(Succeed())
						Expect(decoded).To(HaveKey("config"))
					})

					Describe("the image config", func() {
						var config map[string]interface{}

						JustBeforeEach(func() {
							var buf bytes.Buffer
							blobStore.Get(manifest["config"].(map[string]interface{})["digest"].(string), &buf)
							Expect(json.NewDecoder(bytes.NewReader(buf.Bytes())).Decode(&config)).To(Succeed())
						})

						It("should have the 'vcap' user set", func() {
							Expect(config).To(HaveKeyWithValue("config",
								HaveKeyWithValue("user", "vcap")))
						})

						It("should have a rootfs element with the correct diffids for the rootfs and the droplet", func() {
							Expect(config).To(HaveKeyWithValue("rootfs",
								HaveKeyWithValue("type", "layers")))
							Expect(config).To(HaveKeyWithValue("rootfs",
								HaveKeyWithValue("diff_ids", ConsistOf(
									"the-rootfs-digest",
									"the-droplet-digest",
								))))
						})
					})
				})
			})
		})
	})
})

func fetch(url string) *http.Response {
	res, err := http.Get(url)
	Expect(err).NotTo(HaveOccurred())
	return res
}

func post(url, contentType string, body io.Reader) *http.Response {
	res, err := http.Post(url, contentType, body)
	Expect(err).NotTo(HaveOccurred())
	return res
}
