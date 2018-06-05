package cmd

import (
	"log"
	"net/http"
	"os"

	"code.cloudfoundry.org/eirini/blobondemand"
	"code.cloudfoundry.org/eirini/registry"
	"github.com/spf13/cobra"
)

var (
	rootfs string
)

var registryCmd = &cobra.Command{
	Use:   "registry",
	Short: "starts the eirini registry",
	Run:   reg,
}

func reg(cmd *cobra.Command, args []string) {
	blobstore := blobondemand.NewInMemoryStore()

	path, err := cmd.Flags().GetString("rootfs")
	exitWithError(err)

	rootfsTar, err := os.Open(path)
	exitWithError(err)

	rootfsDigest, rootfsSize, err := blobstore.Put(rootfsTar)
	exitWithError(err)

	log.Fatal(http.ListenAndServe("0.0.0.0:8080", registry.NewHandler(
		registry.BlobRef{
			Digest: rootfsDigest,
			Size:   rootfsSize,
		},
		make(registry.InMemoryDropletStore),
		blobstore,
	)))
}

func initRegistry() {
	registryCmd.Flags().StringVarP(&rootfs, "rootfs", "r", "", "Path to the rootfs tarball")
}
