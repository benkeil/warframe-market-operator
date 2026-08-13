package orders

import (
	"github.com/spf13/cobra"
)

func NewCommand(debug *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "orders",
		Short: "Interact with Warframe Market orders",
	}

	cmd.AddCommand(newTopCommand(debug))
	cmd.AddCommand(newListCommand(debug))

	return cmd
}
