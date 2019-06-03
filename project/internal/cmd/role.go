package cmd

import (
	"path/filepath"
	"regexp"

	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/acl"
	"github.com/spf13/cobra"
)

var uidsManager string
var uidsContributor string
var uidsViewer string
var forceFlag bool
var numThreads int
var followSymlink bool
var silenceFlag bool
var recursion bool

func init() {
	roleSetCmd.PersistentFlags().StringVarP(
		&uidsManager,
		"manager", "m", "",
		"comma-separated system uids to be set as project managers",
	)
	roleSetCmd.PersistentFlags().StringVarP(
		&uidsContributor,
		"contributor", "c", "",
		"comma-separated system uids to be set as project contributors",
	)
	roleSetCmd.PersistentFlags().StringVarP(
		&uidsViewer,
		"viewer", "u", "",
		"comma-separated system uids to be set as project viewers",
	)

	roleCmd.PersistentFlags().BoolVarP(
		&forceFlag,
		"force", "f", false,
		"force the role setting",
	)
	roleCmd.PersistentFlags().BoolVarP(
		&silenceFlag,
		"silence", "s", false,
		"enable silence mode",
	)
	roleCmd.PersistentFlags().BoolVarP(
		&followSymlink,
		"link", "l", false,
		"follow symlinks to set roles",
	)
	roleCmd.PersistentFlags().IntVarP(
		&numThreads,
		"nthreads", "n", 8,
		"number of parallel worker threads",
	)

	roleGetCmd.PersistentFlags().BoolVarP(
		&recursion,
		"recursive", "r", false,
		"enable recursion for getting roles",
	)

	roleCmd.AddCommand(roleGetCmd, roleSetCmd)
	rootCmd.AddCommand(roleCmd)
}

var roleCmd = &cobra.Command{
	Use:   "role",
	Short: "Manage data access role for projects",
	Long:  ``,
}

var roleGetCmd = &cobra.Command{
	Use:   "get [ projectID | path ]",
	Short: "Get data access roles for a project or a path",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// the input argument starts with 7 digits (considered as project number)
		ppathSym := args[0]
		if matched, _ := regexp.MatchString("^[0-9]{7,}", ppathSym); matched {
			ppathSym = filepath.Join(ProjectRootPath, ppathSym)
		} else {
			ppathSym, _ = filepath.Abs(ppathSym)
		}

		runner := acl.Runner{
			RootPath:   ppathSym,
			FollowLink: followSymlink,
			Nthreads:   numThreads,
		}

		return runner.GetRoles(recursion)
	},
}

var roleSetCmd = &cobra.Command{
	Use:   "set [ projectID | path ]",
	Short: "Set data access roles for a project or a path",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// the input argument starts with 7 digits (considered as project number)
		ppathSym := args[0]
		if matched, _ := regexp.MatchString("^[0-9]{7,}", ppathSym); matched {
			ppathSym = filepath.Join(ProjectRootPath, ppathSym)
		} else {
			ppathSym, _ = filepath.Abs(ppathSym)
		}

		runner := acl.Runner{
			RootPath:     ppathSym,
			Managers:     uidsManager,
			Contributors: uidsContributor,
			Viewers:      uidsViewer,
			FollowLink:   followSymlink,
			Nthreads:     numThreads,
			Silence:      silenceFlag,
			Traverse:     true,
			Force:        forceFlag,
		}

		_, err := runner.SetRoles()
		return err
	},
}
