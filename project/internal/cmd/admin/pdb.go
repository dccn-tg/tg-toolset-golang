package admin

import (
	"os/user"

	"github.com/spf13/cobra"
)

var configFile string
var flagRole bool
var flagUsage bool

// showPdbCmd restricts PDB sub commands to root and TG members.
var showPdbCmd = func() bool {
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

	pdbUpdateCmd.Flags().BoolVarP(&flagRole, "role", "r", false, "update project access roles on the filer to PDB")
	pdbUpdateCmd.Flags().BoolVarP(&flagUsage, "usage", "u", false, "update project storage usage on the filer to PDB")

	pdbCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "config.xml", "path of the configuration YAML file.")
	pdbCmd.AddCommand(pdbUpdateCmd, pdbExecuteCmd)

	rootCmd.AddCommand(pdbCmd)
}

var pdbCmd = &cobra.Command{
	Use:   "pdb",
	Short: "Actions around the project database.",
	Long:  ``,
}

// subcommand to update PDB registry with filer information.
var pdbUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update PDB registry with filer information",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

// subcommand to execute pending PDB actions on storage.
var pdbExecuteCmd = &cobra.Command{
	Use:   "execute",
	Short: "Execute pending PDB actions on the filer",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}
