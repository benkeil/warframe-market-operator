package items

import (
	"github.com/spf13/cobra"
)

func NewCommand(debug *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "items",
		Short: "Interact with Warframe Market items",
	}

	cmd.AddCommand(newSearchCommand(debug))

	return cmd
}
