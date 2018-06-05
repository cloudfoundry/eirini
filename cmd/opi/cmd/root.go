package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "opi",
	Short: "put a K8s behind CF",
	Long:  `some long description`,
}

func init() {
	initConnect()
	initRegistry()
	rootCmd.AddCommand(connectCmd)
	rootCmd.AddCommand(registryCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
