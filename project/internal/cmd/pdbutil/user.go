package pdbutil

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	userCmd.AddCommand(userInfoCmd)
	rootCmd.AddCommand(userCmd)
}

var userCmd = &cobra.Command{
	Use:   "user",
	Short: "Utility for user",
	Long:  ``,
}

var userInfoCmd = &cobra.Command{
	Use:   "info [userID]",
	Short: "Get information of a user",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pdb := loadPdb()
		uinfo, err := pdb.GetUser(args[1])
		if err != nil {
			return err
		}

		b, err := json.Marshal(uinfo)
		if err != nil {
			return err
		}

		// TODO: pretty print marshaled json string.
		fmt.Printf("%s", b)
		return nil
	},
}
