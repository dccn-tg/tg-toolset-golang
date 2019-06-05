package cmd

import (
	"strconv"

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
	volCmd.AddCommand(volCreateCmd)
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
