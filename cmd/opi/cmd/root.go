package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func Execute() {
	rootCmd := &cobra.Command{
		Use:   "opi",
		Short: "put a K8s behind CF",
	}

	connectCmd := &cobra.Command{
		Use:   "connect",
		Short: "connects CloudFoundry with Kubernetes",
		Run:   connect,
	}
	connectCmd.Flags().StringP("config", "c", "", "Path to the Eirini config file")

	rootCmd.AddCommand(connectCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err) //nolint:makezero
		os.Exit(1)
	}
}
