package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	ustr "github.com/Donders-Institute/tg-toolset-golang/pkg/strings"
	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/acl"
	log "github.com/sirupsen/logrus"
)

var path *string
var recursion *bool
var nthreads *int
var verbose *bool
var optsFollowLink *bool
var optsSkipFiles *bool

func init() {
	path = flag.String("d", "/project", "root path of project storage")
	recursion = flag.Bool("r", false, "get roles on files and directories recursively")
	nthreads = flag.Int("n", 4, "number of concurrent processing threads")
	verbose = flag.Bool("v", false, "print debug messages")
	optsFollowLink = flag.Bool("l", false, "`follow` symlinks to set roles on referents")
	optsSkipFiles = flag.Bool("k", false, "`skip` getting roles on existing files")

	flag.Usage = usage
	flag.Parse()

	// set logging
	log.SetOutput(os.Stderr)
	// set logging level
	llevel := log.InfoLevel
	if *verbose {
		llevel = log.DebugLevel
	}
	log.SetLevel(llevel)
}

func usage() {
	fmt.Printf("\nGetting users' access permission on a given project or a path.\n")
	fmt.Printf("\nUSAGE: %s [OPTIONS] projectId|path\n", os.Args[0])
	fmt.Printf("\nOPTIONS:\n")
	flag.PrintDefaults()
	fmt.Printf("\nEXAMPLES:\n")
	fmt.Printf("\n%s\n", ustr.StringWrap("Getting users with access permission on project 3010000.01", 80))
	fmt.Printf("\n  %s 3010000.01\n", os.Args[0])
	fmt.Printf("\n%s\n", ustr.StringWrap("Getting users with access permission on all files/directories under project 3010000.01", 80))
	fmt.Printf("\n  %s -r 3010000.01\n", os.Args[0])
	fmt.Printf("\n%s\n", ustr.StringWrap("Getting users with access permission on a specific file/directory", 80))
	fmt.Printf("\n  %s /project/3010000.01/test.txt\n", os.Args[0])
	fmt.Printf("\n")
}

func main() {

	// command-line arguments
	args := flag.Args()

	if len(args) < 1 {
		flag.Usage()
		log.Fatal(fmt.Sprintf("unknown project number: %v", args))
	}

	ppath := args[0]
	// the input argument starts with 7 digits (considered as project number)
	if matched, _ := regexp.MatchString("^[0-9]{7,}", ppath); matched {
		ppath = filepath.Join(*path, ppath)
	} else {
		ppath, _ = filepath.Abs(ppath)
	}
	runner := acl.Runner{
		RootPath:   ppath,
		FollowLink: *optsFollowLink,
		SkipFiles:  *optsSkipFiles,
		Nthreads:   *nthreads,
	}

	if err := runner.GetRoles(*recursion); err != nil {
		log.Fatalln(err)
	}
}
