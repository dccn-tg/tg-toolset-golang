package cmd

import (
	"strconv"

	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/pdb"
	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/vol"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var volManagerAddress string

func init() {
	volCmd.PersistentFlags().StringVarP(
		&volManagerAddress,
		"manager", "m", "filer-a-mi.dccn.nl:22",
		"IP or hostname of the storage's management server",
	)

	// administrator's CLI
	volProvisionCmd.Flags().IntVarP(
		&numThreads,
		"nthreads", "n", 2,
		"number of parallel worker threads",
	)

	volCmd.AddCommand(volCreateCmd, volProvisionCmd)
	adminCmd.AddCommand(volCmd)
}

var volCmd = &cobra.Command{
	Use:   "vol",
	Short: "Manage storage volume for projects",
	Long:  ``,
}

var volCreateCmd = &cobra.Command{
	Use:   "create [projectID] [quotaGiB]",
	Short: "Create storage volume for a project",
	Long:  ``,
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		volManager := vol.NetAppVolumeManager{
			AddressFilerMI: volManagerAddress,
		}

		// parse second argument to integer
		quota, err := strconv.Atoi(args[1])
		if err != nil {
			log.Fatalf("quota value not an integer: %s\n", args[1])
		}

		if err := volManager.Create(args[0], quota); err != nil {
			log.Errorln(err)
		}
	},
}

var volProvisionCmd = &cobra.Command{
	Use:   "provision [projectID]",
	Short: "Provision storage volume and pending access roles for projects.",
	Long: `Provision storage volume and pending access roles for projects.
	
If no specific "projectID" is given in the argument, it runs over all projects
with pending access-role settings in the project database.

If the namespace of the project storage doesn't exist, it will creates the
corresponding storage volume on the file server with the calcuated quota stored
in the project database.

`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runner := pdb.Runner{
			Nthreads:   numThreads,
			ConfigFile: configFile,
		}
		runner.ProvisionOrUpdateStorage(args...)
	},
}
