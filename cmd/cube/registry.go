package main

import (
	"log"
	"net/http"
	"os"

	"github.com/julz/cube/blobondemand"
	"github.com/julz/cube/registry"
	"github.com/urfave/cli"
)

func registryCmd(c *cli.Context) {
	blobstore := blobondemand.NewInMemoryStore()

	rootfsTar, err := os.Open(c.String("rootfs"))
	if err != nil {
		log.Fatal(err)
	}

	rootfsDigest, rootfsSize, err := blobstore.Put(rootfsTar)
	if err != nil {
		log.Fatal(err)
	}

	log.Fatal(http.ListenAndServe("0.0.0.0:8080", registry.NewHandler(
		registry.BlobRef{
			Digest: rootfsDigest,
			Size:   rootfsSize,
		},
		make(registry.InMemoryDropletStore),
		blobstore,
	)))
}
