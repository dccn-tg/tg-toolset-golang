package main

import (
	"os"
	"strconv"

	"github.com/Donders-Institute/tg-toolset-golang/project/internal/vol"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var verbose bool
var volManagerAddress string

func init() {

	volCmd.PersistentFlags().StringVarP(
		&volManagerAddress,
		"manager", "m", "filer-a-mi.dccn.nl:22",
		"IP or hostname of the storage's management server",
	)
	volCmd.AddCommand(volCreateCmd)

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.AddCommand(volCmd)
}

var rootCmd = &cobra.Command{
	Use:   "project",
	Short: "Utility CLI for managing DCCN project",
	Long:  ``,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if cmd.Flags().Changed("verbose") {
			log.SetLevel(log.DebugLevel)
		}
	},
}

var volCmd = &cobra.Command{
	Use:   "vol",
	Short: "Manage project volume on central storage",
	Long:  ``,
}

var volCreateCmd = &cobra.Command{
	Use:   "create [projectID] [quotaGiB]",
	Short: "Create volume on central storage for the given project",
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

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Errorln(err)
		os.Exit(1)
	}
}
