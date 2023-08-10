// This program uses the linux capabilities for operations granted to
// project managers when POSIX ACL system is used on the filesystem (e.g.
// CephFs). Specific capababilities are:
//
//   - CAP_SYS_ADMIN: for accessing the `trusted.managers` xattr that maintains
//     a list of project managers.
//
//   - CAP_FOWNER: for allowing managers to set ACLs via the `setfacl` command
//     without being the owner of files and directories.
//
// In order to allow this trick to work, this executable should be set in
// advance to allow using the linux capability using the following command.
//
// ```
// $ sudo setcap cap_fowner,cap_sys_admin+eip prj_setacl
// ```
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	log "github.com/dccn-tg/tg-toolset-golang/pkg/logger"
	ustr "github.com/dccn-tg/tg-toolset-golang/pkg/strings"
	"github.com/dccn-tg/tg-toolset-golang/project/pkg/acl"
)

// global variables from command-line arguments
var optsBase *string
var optsPath *string
var optsManager *string
var optsContributor *string

// var optsWriter *string
var optsViewer *string
var optsNoTraverse *bool
var optsNthreads *int
var optsForce *bool
var optsVerbose *bool
var optsSilence *bool
var optsFollowLink *bool
var optsSkipFiles *bool

func init() {
	optsManager = flag.String("m", "", "specify a comma-separated-list of users for the manager role")
	optsContributor = flag.String("c", "", "specify a comma-separated-list of users for the contributor role")
	// optsWriter = flag.String("w", "", "specify a comma-separated-list of users for the writer role")
	optsViewer = flag.String("u", "", "specify a comma-separated-list of users for the viewer role")
	optsNoTraverse = flag.Bool("t", false, "`skip` setting role users to travel through parent directories")
	optsBase = flag.String("d", "/project", "set the root path of project storage")
	optsPath = flag.String("p", "", "set path of a sub-directory in the project folder")
	optsNthreads = flag.Int("n", 4, "set number of concurrent processing threads")
	optsForce = flag.Bool("f", false, "force role setting regardlessly")
	optsVerbose = flag.Bool("v", false, "print `verbosed` messages")
	optsSilence = flag.Bool("s", false, "set to `silence` mode")
	optsFollowLink = flag.Bool("l", false, "`follow` symlink to set roles on its first non-symlink referent")
	optsSkipFiles = flag.Bool("k", false, "`skip` setting roles on existing files")

	flag.Usage = usage

	flag.Parse()

	cfg := log.Configuration{
		EnableConsole:     true,
		ConsoleJSONFormat: false,
		ConsoleLevel:      log.Info,
	}

	if *optsVerbose {
		cfg.ConsoleLevel = log.Debug
	}

	// initialize logger
	log.NewLogger(cfg, log.InstanceLogrusLogger)
}

func usage() {
	fmt.Printf("\nSetting users' access permission on a given project or a path.\n")
	fmt.Printf("\nUSAGE: %s [OPTIONS] projectId|path\n", os.Args[0])
	fmt.Printf("\nOPTIONS:\n")
	flag.PrintDefaults()
	fmt.Printf("\nEXAMPLES:\n")
	fmt.Printf("\n%s\n", ustr.StringWrap("Adding or setting users 'honlee' and 'edwger' to the 'contributor' role on project 3010000.01", 80))
	fmt.Printf("\n  %s -c honlee,edwger 3010000.01\n", os.Args[0])
	fmt.Printf("\n%s\n", ustr.StringWrap("Adding or setting user 'honlee' to the 'manager' role, and 'edwger' to the 'viewer' role on project 3010000.01", 80))
	fmt.Printf("\n  %s -m honlee -u edwger 3010000.01\n", os.Args[0])
	fmt.Printf("\n%s\n", ustr.StringWrap("Adding or setting users 'honlee' and 'edwger' to the 'contributor' role on a specific path, and allowing the two users to traverse through the parent directories", 80))
	fmt.Printf("\n  %s -c honlee,edwger /project/3010000.01/data_dir\n", os.Args[0])
	fmt.Printf("\n")
}

func main() {

	// command-line options
	args := flag.Args()

	if len(args) < 1 {
		flag.Usage()
		log.Fatalf("unknown project number: %v", args)
	}

	// the input argument starts with 7 digits (considered as project number)
	ppathSym := args[0]
	if matched, _ := regexp.MatchString("^[0-9]{7,}", ppathSym); matched {
		ppathSym = filepath.Join(*optsBase, ppathSym, *optsPath)
	} else {
		ppathSym, _ = filepath.Abs(ppathSym)
	}

	runner := acl.Runner{
		Managers:     *optsManager,
		Contributors: *optsContributor,
		Viewers:      *optsViewer,
		RootPath:     ppathSym,
		Traverse:     !*optsNoTraverse,
		Force:        *optsForce,
		FollowLink:   *optsFollowLink,
		SkipFiles:    *optsSkipFiles,
		Silence:      *optsSilence,
		Nthreads:     *optsNthreads,
	}

	exitcode, err := runner.SetRoles()
	if err != nil {
		log.Fatalf("%s", err)
	}
	os.Exit(exitcode)
}
