package registry

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
)

type BlobRef struct {
	Digest string
	Size   int64
}

type BlobStore interface {
	Put(buf io.Reader) (digest string, size int64, err error)
	PutWithId(guid string, buf io.Reader) (digest string, size int64, err error)
	Has(digest string) bool
	Get(digest string, dest io.Writer) error
}

type DropletStore interface {
	Get(guid string) *BlobRef
	Set(guid string, blob BlobRef)
}

func NewHandler(rootfsBlob BlobRef, dropletStore DropletStore, blobs BlobStore) http.Handler {
	mux := mux.NewRouter()
	mux.Path("/v2").HandlerFunc(Ping)
	mux.Path("/v2/{space}/{app}/blobs/").Methods("POST").Handler(Stager{blobs, dropletStore})
	mux.Path("/v2/{space}/{app}/blobs/{digest}").Methods("GET").Handler(BlobHandler{blobs})
	mux.Path("/v2/{space}/{app}/manifests/{guid}").Handler(ManifestHandler{Rootfs: rootfsBlob, DropletStore: dropletStore, BlobStore: blobs})

	return mux
}

func Ping(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Printf("[%s]\tReceived Ping\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(w, "Pong")
}

type BlobHandler struct {
	blobs BlobStore
}

func (b BlobHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	digest := vars["digest"]

	if !b.blobs.Has(digest) {
		http.NotFound(w, r)
	}

	b.blobs.Get(digest, w)
}

type ManifestHandler struct {
	Rootfs       BlobRef
	DropletStore DropletStore
	BlobStore    BlobStore
}

func (b ManifestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	dropletGuid := mux.Vars(r)["guid"]
	droplet := b.DropletStore.Get(dropletGuid)

	config, err := json.Marshal(map[string]interface{}{
		"config": map[string]interface{}{
			"user": "vcap",
		},
		"rootfs": map[string]interface{}{
			"type": "layers",
			"diff_ids": []string{
				b.Rootfs.Digest,
				droplet.Digest,
			},
		},
	})

	if err != nil {
		http.Error(w, "couldnt create config json for droplet", 500)
		return
	}

	configDigest, configSize, err := b.BlobStore.Put(bytes.NewReader(config))
	if err != nil {
		http.Error(w, "couldnt store config json for droplet", 500)
		return
	}

	w.Header().Add("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"mediaType":     "application/vnd.docker.distribution.manifest.v2+json",
		"schemaVersion": 2,
		"config": map[string]interface{}{
			"mediaType": "application/vnd.docker.container.image.v1+json",
			"digest":    configDigest,
			"size":      configSize,
		},
		"layers": []map[string]interface{}{
			{
				"digest":    b.Rootfs.Digest,
				"size":      b.Rootfs.Size,
				"mediaType": "application/vnd.docker.image.rootfs.diff.tar",
			},
			{
				"digest":    droplet.Digest,
				"mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
				"size":      droplet.Size,
			},
		},
	})
}

type Stager struct {
	blobs    BlobStore
	droplets DropletStore
}

// Stager is a bit of a hack to deal with the fact that droplets aren't content-addressed
// and are rooted at /home/vcap instead of /. Basically it just translates the incoming
// droplet and stores it content-addressed in the local blobstore and stores a mapping
// from the droplet guid to the content-addressed guid. This is fine for now but the real
// solution is just to modify staging so that the droplet is a valid content-addressed layer
func (s Stager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	guid := r.FormValue("guid")
	if guid == "" {
		http.Error(w, "guid parameter is required", http.StatusInternalServerError)
	}

	tmp, err := ioutil.TempFile("", "converted-layer")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer os.Remove(tmp.Name())

	layer := tar.NewWriter(tmp)

	gz, err := gzip.NewReader(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	t := tar.NewReader(gz)
	for {
		hdr, err := t.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		hdr.Name = filepath.Join("/home/vcap", hdr.Name)
		layer.WriteHeader(hdr)
		if _, err := io.Copy(layer, t); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if err := layer.Close(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	layerTar, err := os.Open(tmp.Name())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer layerTar.Close()

	digest, size, err := s.blobs.Put(layerTar)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, _, err = s.blobs.PutWithId(guid, layerTar)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Println("added droplet to Blob: ", guid, " ", digest, " ")

	s.droplets.Set(guid, BlobRef{
		Digest: digest,
		Size:   size,
	})

	w.WriteHeader(http.StatusCreated)

	fmt.Fprintf(w, digest)
}

// InMemoryDropletStore exists because CC droplets aren't content-addressed
// but instead have guids. Therefore we need to store a lookup of droplet digest
// to droplet guid
type InMemoryDropletStore map[string]BlobRef

func (s InMemoryDropletStore) Get(guid string) *BlobRef {
	if r, ok := s[guid]; ok {
		return &r
	}

	return nil
}

func (s InMemoryDropletStore) Set(guid string, blob BlobRef) {
	s[guid] = blob
}
