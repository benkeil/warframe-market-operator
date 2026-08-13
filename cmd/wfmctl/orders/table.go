package orders

import (
	"fmt"
	"io"
	"strconv"

	"github.com/olekukonko/tablewriter"

	"github.com/benkeil/warframe-market-operator/internal/domain/service"
)

func renderOrdersTable(w io.Writer, orders []service.Order) error {
	table := tablewriter.NewWriter(w)
	table.Header("PLATINUM", "QUANTITY", "USER", "STATUS", "PLATFORM", "CROSSPLAY", "REPUTATION")
	for _, o := range orders {
		crossplay := "no"
		if o.User.Crossplay {
			crossplay = "yes"
		}
		if err := table.Append([]string{
			strconv.Itoa(o.Platinum),
			strconv.Itoa(o.Quantity),
			o.User.IngameName,
			string(o.User.Status),
			string(o.User.Platform),
			crossplay,
			strconv.Itoa(o.User.Reputation),
		}); err != nil {
			return fmt.Errorf("rendering table: %w", err)
		}
	}
	return table.Render()
}
