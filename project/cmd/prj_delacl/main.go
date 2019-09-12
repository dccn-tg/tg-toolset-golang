package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	ustr "github.com/Donders-Institute/tg-toolset-golang/pkg/strings"
	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/acl"
	log "github.com/sirupsen/logrus"
)

// global variables from command-line arguments
var optsBase *string
var optsPath *string
var optsManager *string
var optsContributor *string

//var optsWriter *string
var optsViewer *string
var optsTraverse *bool
var optsNthreads *int
var optsForce *bool
var optsVerbose *bool
var optsSilence *bool
var optsFollowLink *bool
var optsSkipFiles *bool

func init() {
	optsManager = flag.String("m", "", "specify a comma-separated-list of users to be removed from the manager role")
	optsContributor = flag.String("c", "", "specify a comma-separated-list of users to be removed from the contributor role")
	//optsWriter = flag.String("w", "", "specify a comma-separated-list of users for the writer role")
	optsViewer = flag.String("u", "", "specify a comma-separated-list of users to be removed from the viewer role")
	optsTraverse = flag.Bool("t", false, "remove users' traverse permission from the parent directories")
	optsBase = flag.String("d", "/project", "set the root path of project storage")
	optsPath = flag.String("p", "", "set path of a sub-directory in the project folder")
	optsNthreads = flag.Int("n", 2, "set number of concurrent processing threads")
	optsForce = flag.Bool("f", false, "force the deletion regardlessly")
	optsVerbose = flag.Bool("v", false, "print debug messages")
	optsSilence = flag.Bool("s", false, "set to `silence` mode")
	optsFollowLink = flag.Bool("l", false, "`follow` symlinks to set roles on referents")
	optsSkipFiles = flag.Bool("k", false, "`skip` deleting roles on existing files")

	flag.Usage = usage

	flag.Parse()

	// set logging
	log.SetOutput(os.Stderr)
	// set logging level
	llevel := log.InfoLevel
	if *optsVerbose {
		llevel = log.DebugLevel
	}
	log.SetLevel(llevel)
}

func usage() {
	fmt.Printf("\nRemoving users' access permission on a given project or a path.\n")
	fmt.Printf("\nUSAGE: %s [OPTIONS] projectId|path\n", os.Args[0])
	fmt.Printf("\nOPTIONS:\n")
	flag.PrintDefaults()
	fmt.Printf("\nEXAMPLES:\n")
	fmt.Printf("\n%s\n", ustr.StringWrap("Removing users 'honlee' and 'edwger' from accessing the project 3010000.01", 80))
	fmt.Printf("\n  %s honlee,edwger 3010000.01\n", os.Args[0])
	fmt.Printf("\n%s\n", ustr.StringWrap("Removing users 'honlee' and 'edwger' from the 'contributor' role on project 3010000.01", 80))
	fmt.Printf("\n  %s -c honlee,edwger 3010000.01\n", os.Args[0])
	fmt.Printf("\n%s\n", ustr.StringWrap("Removing users 'honlee' and 'edwger' from accessing files and directories under a specific path", 80))
	fmt.Printf("\n  %s honlee,edwger /project/3010000.01/data_dir\n", os.Args[0])
	fmt.Printf("\n%s\n", ustr.StringWrap("Removing users 'honlee' and 'edwger' from accessing files and directories under a specific path, and the traverse permission on its parent directories", 80))
	fmt.Printf("\n  %s -t honlee,edwger /project/3010000.01/data_dir\n", os.Args[0])
	fmt.Printf("\n")
}

func main() {

	// command-line options
	args := flag.Args()

	if len(args) < 1 {
		flag.Usage()
		log.Fatal(fmt.Sprintf("unknown project number: %v", args))
	}

	if len(args) >= 2 && *optsManager+*optsContributor+*optsViewer != "" {
		flag.Usage()
		log.Fatal("use only one way to specify users: with or without role options (-m|-c|-u), not both.")
	}

	uidsAll := ""
	ppathSym := args[0]
	if len(args) >= 2 {
		uidsAll = args[0]
		ppathSym = args[1]
	}

	// the input argument starts with 7 digits (considered as project number)
	if matched, _ := regexp.MatchString("^[0-9]{7,}", ppathSym); matched {
		ppathSym = filepath.Join(*optsBase, ppathSym, *optsPath)
	} else {
		ppathSym, _ = filepath.Abs(ppathSym)
	}

	runner := acl.Runner{
		RootPath:     ppathSym,
		Managers:     strings.Join([]string{*optsManager, uidsAll}, ","),
		Contributors: strings.Join([]string{*optsContributor, uidsAll}, ","),
		Viewers:      strings.Join([]string{*optsViewer, uidsAll}, ","),
		Traversers:   uidsAll,
		FollowLink:   *optsFollowLink,
		SkipFiles:    *optsSkipFiles,
		Nthreads:     *optsNthreads,
		Silence:      *optsSilence,
		Traverse:     *optsTraverse,
		Force:        *optsForce,
	}

	exitcode, err := runner.RemoveRoles()
	if err != nil {
		log.Fatalln(err)
	}
	os.Exit(exitcode)
}
