package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"dccn.nl/project/acl"
	ufp "dccn.nl/utility/filepath"
	ustr "dccn.nl/utility/strings"
	log "github.com/sirupsen/logrus"
)

var path *string
var recursion *bool
var nthreads *int
var verbose *bool
var optsFollowLink *bool

func init() {
	path = flag.String("d", "/project", "root path of project storage")
	recursion = flag.Bool("r", false, "get roles on files and directories recursively")
	nthreads = flag.Int("n", 4, "number of concurrent processing threads")
	verbose = flag.Bool("v", false, "print debug messages")
	optsFollowLink = flag.Bool("l", false, "`follow` symlinks to set roles on referents")

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

	// resolve any symlinks on ppath
	ppath, _ = filepath.EvalSymlinks(ppath)

	fpinfo, err := ufp.GetFilePathMode(ppath)
	if err != nil {
		log.Fatal(fmt.Sprintf("path not found or unaccessible: %s", ppath))
	} else {
		// disable recursion if ppath is not a directory
		if !fpinfo.Mode.IsDir() {
			*recursion = false
		}
	}

	var chanD chan ufp.FilePathMode
	if *recursion {
		chanD = ufp.GoFastWalk(ppath, *optsFollowLink, *nthreads)
	} else {
		*nthreads = 1
		chanD = make(chan ufp.FilePathMode)
		go func() {
			chanD <- *fpinfo
			defer close(chanD)
		}()
	}

	chanOut := goGetACL(chanD, *nthreads)

	for o := range chanOut {
		fmt.Printf("%s:\n", o.Path)
		for _, r := range []acl.Role{acl.Manager, acl.Contributor, acl.Writer, acl.Viewer, acl.Traverse} {
			if users, ok := o.RoleMap[r]; ok {
				fmt.Printf("%12s: %s\n", r, strings.Join(users, ","))
			}
		}
	}
}

// goGetACL performs getting ACL on walked paths, using a go routine. The result is pushed to
// a channel of acl.RolePathMap.  It also closes the channel when all walked paths are processed.
func goGetACL(chanD chan ufp.FilePathMode, nthreads int) chan acl.RolePathMap {

	// output channel
	chanOut := make(chan acl.RolePathMap)

	// launch parallel go routines for getting ACL
	chanSync := make(chan int)
	for i := 0; i < nthreads; i++ {
		go func() {
			for p := range chanD {

				roler := acl.GetRoler(p)
				if roler == nil {
					log.Warn(fmt.Sprintf("roler not found: %s", p.Path))
					continue
				}
				log.Debug(fmt.Sprintf("path: %s %s", p.Path, reflect.TypeOf(roler)))
				if roles, err := roler.GetRoles(p); err == nil {
					chanOut <- acl.RolePathMap{Path: p.Path, RoleMap: roles}
				} else {
					log.Error(fmt.Sprintf("%s: %s", err, p.Path))
				}
			}
			chanSync <- 1
		}()
	}

	// launch synchronise go routine
	go func() {
		i := 0
		for {
			i = i + <-chanSync
			if i == nthreads {
				break
			}
		}
		close(chanOut)
	}()

	return chanOut
}
