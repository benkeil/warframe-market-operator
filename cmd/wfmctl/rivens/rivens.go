package rivens

import (
	"github.com/spf13/cobra"
)

func NewCommand(debug *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rivens",
		Short: "Search riven mod auctions",
	}

	cmd.AddCommand(newSearchCommand(debug))

	return cmd
}
