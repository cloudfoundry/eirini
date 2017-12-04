package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/julz/cube/blobondemand"
)

var blobs = blobondemand.NewInMemoryStore()
var manifests = make(map[string][]byte)

var rootfsDigest string
var rootfsSize int64

func main() {
	rootfsPath := flag.String("rootfs", "", "path to a tarfile to import as the root filesystem for droplets")
	flag.Parse()

	importRootfs(*rootfsPath)

	mux := mux.NewRouter()
	mux.Path("/v2/{space}/{app}/manifests/{droplet}").Methods("POST").HandlerFunc(StageCompletionHandler)
	mux.HandleFunc("/v2/{space}/{app}/manifests/{droplet}", ManifestHandler)
	mux.HandleFunc("/v2/{space}/{app}/blobs/{guid-or-tag}", BlobHandler)
	mux.PathPrefix("/v2").HandlerFunc(PingHandler)

	fmt.Println("started")
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", mux))
}

func importRootfs(tarFile string) {
	tar, err := os.Open("/Users/julz/workspace/minirootfs.tar")
	if err != nil {
		log.Fatal(err)
	}

	rootfsDigest, rootfsSize, err = blobs.Put(tar)
	if err != nil {
		log.Fatal(err)
	}
}

// StageCompletionHandler stores the manifest and the droplet blob for a staged
// app in the blobstore. For now double-uploads the blob to our own store to avoid
// dealing with blobstore URL signing, but it's easy to 307 the docker client to
// another blobstore to avoid having to double-store the droplets
//
// A nice thing we might think about doing here is to sign each of the layers with their provenance: i.e
// the rootfs layer can be signed and the droplet layer can be signed
func StageCompletionHandler(resp http.ResponseWriter, req *http.Request) {
	droplet := mux.Vars(req)["droplet"]

	dropletDigest, _, err := blobs.Put(req.Body)
	if err != nil {
		log.Fatal(err)
	}

	configDigest, configSize, err := blobs.Put(bytes.NewReader([]byte(fmt.Sprintf(`{
		"rootfs": {
			"diff_ids": [ "%s", "%s" ],
			"type": "layers"
		}
	}`, rootfsDigest, dropletDigest))))

	if err != nil {
		log.Fatal(err)
	}

	manifest, err := json.Marshal(map[string]interface{}{
		"mediaType": "application/vnd.docker.distribution.manifest.v2+json",
		"config": map[string]interface{}{
			"mediaType": "application/vnd.docker.container.image.v1+json",
			"size":      configSize,
			"digest":    configDigest,
		},
		"layers": []map[string]interface{}{
			{
				"mediaType": "application/vnd.docker.image.rootfs.diff.tar",
				"size":      rootfsSize,
				"digest":    rootfsDigest,
			},
			{
				"mediaType": "application/vnd.docker.image.rootfs.diff.tar",
				"digest":    dropletDigest,
			},
		},
		"schemaVersion": 2,
	})

	if err != nil {
		log.Fatal(err)
	}

	manifests[droplet] = manifest
}

// PingHandler exists just to reply 200 for now, docker pings
// this to check if it needs to authenticate, which it currently doesnt
// because lol
func PingHandler(resp http.ResponseWriter, req *http.Request) {
	log.Println("ping", req.URL)
	resp.Write([]byte("ok"))
}

// BlobHandler vends a blob from the current store, it should be possible
// for this to 307 to a real blobstore in future, but this is easier for now
func BlobHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("blob", r.URL)
	digest := mux.Vars(r)["guid-or-tag"]
	if !blobs.Has(digest) {
		http.NotFound(w, r)
	}

	blobs.Get(digest, w)
}

// ManifestHandler serves up the OCI manifest for a particular droplet
func ManifestHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("blob", r.URL)
	droplet := mux.Vars(r)["droplet"]
	w.Header().Add("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
	w.Write(manifests[droplet])
}
