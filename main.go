package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/aarnaud/crowdsec-central-api/cmd/serve"
)

func main() {
	root := &cobra.Command{
		Use:   "crowdsec-capi",
		Short: "CrowdSec Self-Hosted Central API",
	}
	root.AddCommand(serve.Command())
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
