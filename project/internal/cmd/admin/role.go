package admin

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/acl"
	"github.com/spf13/cobra"
)

var uidsManager string
var uidsContributor string
var uidsViewer string
var uidsAll string
var forceFlag bool
var numThreads int
var followSymlink bool
var skipFiles bool
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

	roleRemoveCmd.PersistentFlags().StringVarP(
		&uidsManager,
		"manager", "m", "",
		"comma-separated system uids to be removed from the project manager",
	)
	roleRemoveCmd.PersistentFlags().StringVarP(
		&uidsContributor,
		"contributor", "c", "",
		"comma-separated system uids to be removed from the project contributor",
	)
	roleRemoveCmd.PersistentFlags().StringVarP(
		&uidsViewer,
		"viewer", "u", "",
		"comma-separated system uids to be removed from the project viewer",
	)
	roleRemoveCmd.PersistentFlags().StringVarP(
		&uidsAll,
		"all", "a", "",
		"comma-separated system uids to be removed from the project (regardless of the role)",
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
	roleCmd.PersistentFlags().BoolVarP(
		&skipFiles,
		"skip-files", "k", false,
		"skip setting/deleting/getting roles on individual files",
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

	roleCmd.AddCommand(roleGetCmd, roleSetCmd, roleRemoveCmd)
	rootCmd.AddCommand(roleCmd)

	// // administrator's CLI
	// rolePdbCmd.PersistentFlags().IntVarP(
	// 	&numThreads,
	// 	"nthreads", "n", 8,
	// 	"number of parallel worker threads",
	// )
	// rolePdbCmd.AddCommand(rolePdbUpdateCmd, rolePdbGetPendingCmd)
	// roleAdminCmd.AddCommand(rolePdbCmd)
	// pdbCmd.AddCommand(roleAdminCmd)
}

// roleCmd is the top-level CLI command for managing project roles.
var roleCmd = &cobra.Command{
	Use:   "role",
	Short: "Manage data access role for projects",
	Long:  ``,
}

// roleGetCmd is the CLI command for setting project roles.
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
			SkipFiles:  skipFiles,
			Nthreads:   numThreads,
		}

		return runner.PrintRoles(recursion)
	},
}

// roleRemoveCmd is the CLI command for removing project roles.
var roleRemoveCmd = &cobra.Command{
	Use:   "remove [ projectID | path ]",
	Short: "Remove data access roles for a project or a path",
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
			Managers:     strings.Join([]string{uidsManager, uidsAll}, ","),
			Contributors: strings.Join([]string{uidsContributor, uidsAll}, ","),
			Viewers:      strings.Join([]string{uidsViewer, uidsAll}, ","),
			Traversers:   uidsAll,
			FollowLink:   followSymlink,
			SkipFiles:    skipFiles,
			Nthreads:     numThreads,
			Silence:      silenceFlag,
			Traverse:     false,
			Force:        forceFlag,
		}

		_, err := runner.RemoveRoles()
		return err
	},
}

// roleSetCmd is the CLI command for setting project roles.
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
			SkipFiles:    skipFiles,
			Nthreads:     numThreads,
			Silence:      silenceFlag,
			Traverse:     true,
			Force:        forceFlag,
		}

		_, err := runner.SetRoles()
		return err
	},
}

// // roleAdminCmd is the CLI command for administrating project roles.
// var roleAdminCmd = &cobra.Command{
// 	Use:   "role",
// 	Short: "Administer project roles",
// 	Long:  ``,
// }

// // rolePdbCmd is the CLI command for administrating project roles in project database.
// var rolePdbCmd = &cobra.Command{
// 	Use:   "pdb",
// 	Short: "Administer project roles in project database",
// 	Long:  ``,
// }

// // rolePdbUpdateCmd is the CLI command for administrator to update project roles
// // to the project database, according to the role settings on the project storage.
// var rolePdbUpdateCmd = &cobra.Command{
// 	Use:   "update",
// 	Short: "Update project roles in project database",
// 	Long: `
// Update project roles in project database based on the role settings on project storage.

// This command retrieves the role settings from all project directories and updates the project database accordingly.`,
// 	Args: cobra.NoArgs,
// 	RunE: func(cmd *cobra.Command, args []string) error {

// 		runner := pdb.Runner{
// 			Nthreads:   numThreads,
// 			ConfigFile: configFile,
// 		}

// 		return runner.SyncRolesWithStorage(ProjectRootPath)
// 	},
// }

// var rolePdbGetPendingCmd = &cobra.Command{}
