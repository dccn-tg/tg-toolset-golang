package dicom_worklist

import (
	"os"

	"github.com/Donders-Institute/tg-toolset-golang/pkg/config"
	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/pdb"
	"github.com/spf13/cobra"
)

var verbose bool
var configFile string
var cfg log.Configuration

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "config.yml", "`path` of the configuration YAML file.")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// initiate default logger
	cfg = log.Configuration{
		EnableConsole:     true,
		ConsoleJSONFormat: false,
		ConsoleLevel:      log.Info,
	}
	log.NewLogger(cfg, log.InstanceLogrusLogger)
}

var rootCmd = &cobra.Command{
	Use:   "dicom_worklist",
	Short: "The utility for managing DICOM worklist for DCCN lab dataflow",
	Long:  ``,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// renew the logger with overwritten configuration.
		if cmd.Flags().Changed("verbose") {
			cfg.ConsoleLevel = log.Debug
		}
		log.NewLogger(cfg, log.InstanceLogrusLogger)
	},
}

// loadConfig loads configuration YAML file specified by `configFile`.
// This function fatals out if there is an error.
func loadConfig() config.Configuration {
	conf, err := config.LoadConfig(configFile)
	if err != nil {
		log.Fatalf("%s", err)
	}
	return conf
}

// loadPdb initializes the PDB interface package using the configuration YAML file.
// This function fatals out if there is an error.
func loadPdb() pdb.PDB {
	// initialize pdb interface
	conf := loadConfig()
	ipdb, err := pdb.New(conf.PDB)
	if err != nil {
		log.Fatalf("%s", err)
	}

	return ipdb
}

// Execute is the main entry point of the cluster command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Errorf("%s", err)
		os.Exit(1)
	}
}
