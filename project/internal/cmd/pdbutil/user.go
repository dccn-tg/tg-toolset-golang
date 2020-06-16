package pdbutil

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

//var findByEmail bool

func init() {

	//userFindCmd.Flags().BoolVarP(&findByEmail, "email", "e", true, "find user with the given email address.")

	userCmd.AddCommand(userInfoCmd, userFindCmd)
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
		uinfo, err := pdb.GetUser(args[0])
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

var userFindCmd = &cobra.Command{
	Use:   "find [email]",
	Short: "Find users by matching email address",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pdb := loadPdb()
		uinfo, err := pdb.GetUserByEmail(args[0])
		if err != nil {
			return err
		}

		// for the simplicity, just print the user id.
		fmt.Printf("%s", uinfo.ID)
		return nil
	},
}
