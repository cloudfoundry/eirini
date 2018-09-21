package cmd

import (
	"log"
	"net/http"
	"os"

	"code.cloudfoundry.org/eirini/blobondemand"
	"code.cloudfoundry.org/eirini/registry"
	"github.com/spf13/cobra"
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

	handler := registry.NewHandler(
		registry.BlobRef{
			Digest: rootfsDigest,
			Size:   rootfsSize,
		},
		make(registry.InMemoryDropletStore),
		blobstore,
	)

	cert, err := cmd.Flags().GetString("cert")
	if err != nil {
		panic(err)
	}

	key, err := cmd.Flags().GetString("key")
	if err != nil {
		panic(err)
	}

	if cert != "" && key != "" {
		serveTLS(cert, key, handler)
	}

	serveHTTP(handler)
}

func serveHTTP(handler http.Handler) {
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", handler))
}

func serveTLS(cert, key string, handler http.Handler) {
	go func() {
		log.Fatal(http.ListenAndServeTLS("0.0.0.0:8081", cert, key, handler))
	}()
}

func initRegistry() {
	registryCmd.Flags().StringP("rootfs", "r", "", "Path to the rootfs tarball")
	registryCmd.Flags().StringP("cert", "c", "", "Path to cert")
	registryCmd.Flags().StringP("key", "k", "", "Path to key")
}
