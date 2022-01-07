package repocli

import (
	"bytes"
	"fmt"
	"os"
	"syscall"

	"github.com/Donders-Institute/tg-toolset-golang/pkg/config"
	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	dav "github.com/studio-b12/gowebdav"
	"golang.org/x/term"
	"gopkg.in/yaml.v2"
)

// command to change directory in the repository.
// This command only makes sense in shell mode.
var cdCmd = &cobra.Command{
	Use:   "cd <repo_dir>",
	Short: "change present working directory in the repository",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {

		p := getCleanRepoPath(args[0])

		// stat the path to check if the path is a valid directory
		if f, err := cli.Stat(p); err != nil || !f.IsDir() {
			return fmt.Errorf("invalid directory: %s", p)
		}

		// set cwd to the new path
		cwd = p
		return nil
	},
}

// command to show present working directory in the repository.
// This command only makes sense in shell mode.
var pwdCmd = &cobra.Command{
	Use:   "pwd",
	Short: "print present working directory in the repository",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("%s\n", cwd)
		return nil
	},
}

// command to login repository
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "login the repository with the data-access account",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return promptLogin()
	},
}

// promptLogin asks username and password input for
// authenticating to the webdav interface.
func promptLogin() error {
	repoUser := stringPrompt("username")
	repoPass := passwordPrompt("password")

	save := boolPrompt("save credential")

	// try to connect the repo webdav to check authentication
	cli = dav.NewClient(davBaseURL, repoUser, repoPass)
	if err := cli.Connect(); err != nil {
		return err
	}

	// save credential to `configFile`
	if save {
		return saveCredential(repoUser, repoPass)
	}

	return nil
}

// saveCredential saves the username/password to the file `configFile` with file mode 600.
func saveCredential(username, password string) error {
	conf, err := yaml.Marshal(&struct {
		Repository config.RepositoryConfiguration `yaml:"repository"`
	}{
		config.RepositoryConfiguration{
			Username: username,
			Password: password,
		},
	})

	if err != nil {
		return err
	}

	vconf := viper.New()
	vconf.SetConfigType("yaml")
	err = vconf.ReadConfig(bytes.NewBuffer(conf))
	if err != nil {
		return err
	}

	if err := vconf.WriteConfigAs(configFile); err != nil {
		return err
	}

	if err := os.Chmod(configFile, 0600); err != nil {
		return err
	}

	log.Infof("credential saved in %s", configFile)

	return nil
}

// boolPrompt asks for a string value `y/n` and return a boolean accordingly.
func boolPrompt(label string) bool {
	var s string
	fmt.Fprintf(os.Stderr, label+" [y/N]: ")
	fmt.Scanf("%s", &s)

	if s == "y" || s == "Y" {
		return true
	}
	return false
}

// stringPrompt asks for a string value using the label
func stringPrompt(label string) string {
	var s string
	fmt.Fprintf(os.Stderr, label+": ")
	fmt.Scanf("%s", &s)
	return s
}

// passwordPrompt asks for a password value using the label
func passwordPrompt(label string) string {
	var s string
	for {
		fmt.Fprint(os.Stderr, label+": ")
		b, _ := term.ReadPassword(int(syscall.Stdin))
		s = string(b)
		if s != "" {
			break
		}
	}
	fmt.Println()
	return s
}
