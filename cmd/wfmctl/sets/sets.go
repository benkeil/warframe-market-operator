package sets

import (
	"github.com/spf13/cobra"
)

func NewCommand(debug *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sets",
		Short: "Interact with Warframe prime sets",
	}

	cmd.AddCommand(newInfoCommand(debug))
	cmd.AddCommand(newPriceCommand(debug))

	return cmd
}
