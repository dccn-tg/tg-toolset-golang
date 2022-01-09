package repocli

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/Donders-Institute/tg-toolset-golang/pkg/config"
	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
	shell "github.com/brianstrauch/cobra-shell"
	"github.com/c-bata/go-prompt"
	"github.com/spf13/cobra"
	dav "github.com/studio-b12/gowebdav"
)

var verbose bool
var configFile string
var nthreads int

var silent bool

var shellMode bool

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

	rootCmd.PersistentFlags().StringVarP(
		&davBaseURL,
		"url", "u", "https://webdav.data.donders.ru.nl",
		"`URL` of the webdav server.",
	)

	// subcommand for entering interactive shell prompt
	shellCmd := shell.New(
		rootCmd,
		prompt.OptionSuggestionBGColor(prompt.DarkGray),
		prompt.OptionSuggestionTextColor(prompt.LightGray),
		prompt.OptionDescriptionBGColor(prompt.LightGray),
		prompt.OptionDescriptionTextColor(prompt.DarkGray),
		prompt.OptionSelectedDescriptionTextColor(prompt.Black),
		prompt.OptionSelectedDescriptionBGColor(prompt.Blue),
		prompt.OptionSelectedSuggestionTextColor(prompt.Black),
		prompt.OptionSelectedSuggestionBGColor(prompt.Blue),
		prompt.OptionScrollbarBGColor(prompt.Blue),
		prompt.OptionScrollbarThumbColor(prompt.DarkGray),
	)
	shellCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		shellMode = true
		// enable subcommands that make sense in interactive shell
		rootCmd.AddCommand(loginCmd, cdCmd, pwdCmd)
	}
	rootCmd.AddCommand(shellCmd)

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
	if err != nil && !shellMode {
		log.Fatalf("%s", err)
	}
	return conf
}

var rootCmd = &cobra.Command{
	Use:          "repocli",
	Short:        "A CLI for managing data content of the Donders Repository collections.",
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

		if !shellMode && (repoUser == "" || repoPass == "") {
			return fmt.Errorf("username or password is missing")
		}

		if cli == nil {
			// load global webdav client object
			cli = dav.NewClient(davBaseURL, repoUser, repoPass)
		}

		return nil
	},
}

// Execute is the main entry point of the cluster command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
