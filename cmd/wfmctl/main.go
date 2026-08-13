package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/benkeil/warframe-market-operator/cmd/wfmctl/items"
	"github.com/benkeil/warframe-market-operator/cmd/wfmctl/orders"
	"github.com/benkeil/warframe-market-operator/cmd/wfmctl/sets"
)

func main() {
	var debug bool

	root := &cobra.Command{
		Use:   "wfmctl",
		Short: "Warframe Market CLI",
	}

	root.PersistentFlags().BoolVar(&debug, "debug", false, "Print HTTP request details to stderr")

	root.AddCommand(items.NewCommand(&debug))
	root.AddCommand(orders.NewCommand(&debug))
	root.AddCommand(sets.NewCommand(&debug))

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
