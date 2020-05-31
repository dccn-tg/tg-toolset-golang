package pdbutil

import (
	"os"

	"github.com/Donders-Institute/tg-toolset-golang/pkg/config"
	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/pdb"
	"github.com/spf13/cobra"
)

var verbose bool
var configFile string
var ipdb pdb.PDB

const (
	// ProjectRootPath defines the filesystem root path of the project storage.
	ProjectRootPath = "/project"
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "config.yml", "path of the configuration YAML file.")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
}

var rootCmd = &cobra.Command{
	Use:   "pdbutil",
	Short: "The project database utility",
	Long:  ``,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// initialize logger
		cfg := log.Configuration{
			EnableConsole:     true,
			ConsoleJSONFormat: false,
			ConsoleLevel:      log.Info,
		}

		if cmd.Flags().Changed("verbose") {
			cfg.ConsoleLevel = log.Debug
		}
		log.NewLogger(cfg, log.InstanceLogrusLogger)

		// initialize pdb interface
		conf, err := config.LoadConfig(configFile)
		if err != nil {
			log.Fatalf("%s", err)
		}

		ipdb, err = pdb.New(conf.PDB)
		if err != nil {
			log.Fatalf("%s", err)
		}
	},
}

// Execute is the main entry point of the cluster command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Errorf("%s", err)
		os.Exit(1)
	}
}
