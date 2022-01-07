package repocli

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/Donders-Institute/tg-toolset-golang/pkg/config"
	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
	shell "github.com/brianstrauch/cobra-shell"
	"github.com/spf13/cobra"
	dav "github.com/studio-b12/gowebdav"
)

var verbose bool
var configFile string
var nthreads int

var silent bool

// var cliUsername string
// var cliPassword string
var davBaseURL string

var cfg log.Configuration

var cli *dav.Client

func init() {

	user, err := user.Current()
	if err != nil {
		log.Fatalf(err.Error())
	}

	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", filepath.Join(user.HomeDir, ".repocli.yml"), "`path` of the configuration YAML file.")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().IntVarP(&nthreads, "nthreads", "n", 4, "`number` of concurrent worker threads.")
	rootCmd.PersistentFlags().BoolVarP(&silent, "silent", "s", false, "set to slient mode (i.e. do not show progress)")

	// rootCmd.PersistentFlags().StringVarP(
	// 	&cliUsername,
	// 	"user", "u", "",
	// 	"`username` of the repository data access account.",
	// )
	// rootCmd.PersistentFlags().StringVarP(
	// 	&cliPassword,
	// 	"pass", "p", "",
	// 	"`password` of the repository data access account.",
	// )
	rootCmd.PersistentFlags().StringVarP(
		&davBaseURL,
		"url", "l", "https://webdav.data.donders.ru.nl",
		"`URL` of the webdav server.",
	)

	// subcommand for enable interactive shell prompt
	rootCmd.AddCommand(shell.New(rootCmd))

	// initiate default logger
	cfg = log.Configuration{
		EnableConsole:     true,
		ConsoleJSONFormat: false,
		ConsoleLevel:      log.Info,
	}
	log.NewLogger(cfg, log.InstanceLogrusLogger)
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

var rootCmd = &cobra.Command{
	Use:          "repocli",
	Short:        "A user's CLI for managing data content of the Donders Repository collections.",
	Long:         ``,
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// reset logger level based on command flag
		if cmd.Flags().Changed("verbose") {
			cfg.ConsoleLevel = log.Debug
		}
		log.NewLogger(cfg, log.InstanceLogrusLogger)

		// load repo configuration
		repoCfg := loadConfig().Repository

		repoUser := repoCfg.Username
		repoPass := repoCfg.Password

		if repoUser == "" || repoPass == "" {
			return fmt.Errorf("username or password is missing")
		}

		// load global webdav client object
		cli = dav.NewClient(davBaseURL, repoUser, repoPass)

		return nil
	},
}

// Execute is the main entry point of the cluster command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
