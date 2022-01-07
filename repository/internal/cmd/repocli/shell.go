package repocli

import (
	"fmt"
	"os"
	"syscall"

	"github.com/spf13/cobra"
	dav "github.com/studio-b12/gowebdav"
	"golang.org/x/term"
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
	repoUser := stringPrompt("username:")
	repoPass := passwordPrompt("password:")

	// try to connect the repo webdav to check authentication
	cli = dav.NewClient(davBaseURL, repoUser, repoPass)
	return cli.Connect()
}

// stringPrompt asks for a string value using the label
func stringPrompt(label string) string {
	var s string
	fmt.Fprintf(os.Stderr, label+" ")
	fmt.Scanf("%s", &s)
	return s
}

// passwordPrompt asks for a password value using the label
func passwordPrompt(label string) string {
	var s string
	for {
		fmt.Fprint(os.Stderr, label+" ")
		b, _ := term.ReadPassword(int(syscall.Stdin))
		s = string(b)
		if s != "" {
			break
		}
	}
	fmt.Println()
	return s
}
