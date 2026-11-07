package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "relayforge",
	Short: "RelayForge: Distributed Webhook Delivery Infrastructure",
	Long: `RelayForge is an enterprise-grade distributed webhook delivery infrastructure platform 
designed to guarantee reliable, scalable, and secure event delivery with at-least-once guarantees.`,
}

func init() {
	rootCmd.AddCommand(apiCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

