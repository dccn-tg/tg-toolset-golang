package cmd

import (
	"os/user"

	"github.com/spf13/cobra"
)

var configFile string

// showAdminCmd defines a function to determine whether the admin command should be shown.
var showAdminCmd = func() bool {
	me, err := user.Current()

	if err != nil {
		return false
	}

	grp, err := user.LookupGroupId(me.Gid)
	if err != nil {
		return false
	}

	// allows "root" or "tg" member to see the admin subcommand
	if me.Username == "root" || grp.Name == "tg" {
		return true
	}

	return false
}

func init() {
	adminCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "config.xml", "path of the configuration YAML file.")

	// only show admin command if the user is root
	if showAdminCmd() {
		rootCmd.AddCommand(adminCmd)
	}
}

var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "project administration CLI",
	Long:  ``,
}
