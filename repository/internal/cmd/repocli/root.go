package repocli

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/Donders-Institute/tg-toolset-golang/pkg/config"
	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
	shell "github.com/Donders-Institute/tg-toolset-golang/pkg/shell"
	"github.com/c-bata/go-prompt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

// current working directory
var cwd string = "/"

var lcwd string

var rootCmd = New()

func init() {

	// current user
	user, err := user.Current()
	if err != nil {
		log.Fatalf(err.Error())
	}

	// current working directory at local
	lcwd, err = os.Getwd()
	if err != nil {
		log.Fatalf(err.Error())
	}

	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", filepath.Join(user.HomeDir, ".repocli.yml"), "`path` of the configuration YAML file.")
	// rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	// rootCmd.PersistentFlags().IntVarP(&nthreads, "nthreads", "n", 4, "`number` of concurrent worker threads.")
	// rootCmd.PersistentFlags().BoolVarP(&silent, "silent", "s", false, "set to slient mode (i.e. do not show progress)")

	rootCmd.PersistentFlags().StringVarP(
		&davBaseURL,
		"url", "u", davBaseURL,
		"`URL` of the webdav server.",
	)

	// subcommand for entering interactive shell prompt
	shellCmd := shell.New(
		rootCmd,
		New,
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
		rootCmd.AddCommand(loginCmd, cdCmd, pwdCmd, lcdCmd, lpwdCmd, llsCmd())
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

func New() *cobra.Command {
	cmd := &cobra.Command{
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
			viper.BindPFlag("repository.baseurl", cmd.Flags().Lookup("url"))
			repoCfg := loadConfig().Repository

			repoUser := repoCfg.Username
			repoPass := repoCfg.Password
			baseURL := repoCfg.BaseURL

			if !shellMode && (repoUser == "" || repoPass == "") {
				return fmt.Errorf("username or password is missing")
			}

			if !shellMode && (baseURL == "") {
				return fmt.Errorf("repo baseURL is missing")
			}

			if cli == nil || (baseURL != "" && baseURL != davBaseURL) {
				// initiate a new webdav client with new baseURL
				davBaseURL = baseURL
				cli = dav.NewClient(baseURL, repoUser, repoPass)
			}

			return nil
		},
	}

	cmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	cmd.PersistentFlags().IntVarP(&nthreads, "nthreads", "n", 4, "`number` of concurrent worker threads.")
	cmd.PersistentFlags().BoolVarP(&silent, "silent", "s", false, "set to slient mode (i.e. do not show progress)")

	if shellMode {
		cmd.AddCommand(loginCmd, cdCmd, pwdCmd, lcdCmd, lpwdCmd, llsCmd())
	}

	cmd.AddCommand(lsCmd(), putCmd, getCmd, rmCmd(), mvCmd(), cpCmd(), mkdirCmd)

	return cmd
}

// Execute is the main entry point of the cluster command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
