package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"

	"dccn.nl/project/acl"
	ufp "dccn.nl/utility/filepath"
	ustr "dccn.nl/utility/strings"
	log "github.com/sirupsen/logrus"
)

// global variables from command-line arguments
var optsBase *string
var optsPath *string
var optsManager *string
var optsContributor *string
var optsViewer *string
var optsTraverse *bool
var optsNthreads *int
var optsForce *bool
var optsVerbose *bool
var optsSilence *bool

// global variables derived in the program
var ppathSym string // the absolute path from the input project number or path, it can be a symlink.
var ppath string    // the referent resolved from ppathSym

// global variable for exit code
var exitcode int

var signalHandled = []os.Signal{
	syscall.SIGABRT,
	syscall.SIGHUP,
	syscall.SIGTERM,
	syscall.SIGINT,
}

func init() {
	optsManager = flag.String("m", "", "specify a comma-separated-list of users to be removed from the manager role")
	optsContributor = flag.String("c", "", "specify a comma-separated-list of users to be removed from the contributor role")
	optsViewer = flag.String("u", "", "specify a comma-separated-list of users to be removed from the viewer role")
	optsTraverse = flag.Bool("t", false, "remove users' traverse permission from the parent directories")
	optsBase = flag.String("d", "/project", "set the root path of project storage")
	optsPath = flag.String("p", "", "set path of a sub-directory in the project folder")
	optsNthreads = flag.Int("n", 2, "set number of concurrent processing threads")
	optsForce = flag.Bool("f", false, "force the deletion regardlessly")
	optsVerbose = flag.Bool("v", false, "print debug messages")
	optsSilence = flag.Bool("s", false, "set to `silence` mode")

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

	exitcode = 0
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

	// this defer function ensures that the os.Exit is called with a proper exitcode which is set
	// before this defer operation is registered.
	defer func() {
		os.Exit(exitcode)
	}()

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

	// map for role specification inputs (commad options)
	roleSpec := make(map[acl.Role]string)

	ppathSym = args[0]

	if len(args) >= 2 {
		roleSpec[acl.Manager] = args[0]
		roleSpec[acl.Contributor] = args[0]
		roleSpec[acl.Viewer] = args[0]
		roleSpec[acl.Traverse] = args[0]
		ppathSym = args[1]
	} else {
		roleSpec[acl.Manager] = *optsManager
		roleSpec[acl.Contributor] = *optsContributor
		roleSpec[acl.Viewer] = *optsViewer
	}

	// construct operable map and check duplicated specification
	roles, usersT, err := parseRoles(roleSpec)

	if err != nil {
		log.Fatal(fmt.Sprintf("%s", err))
	}

	// the input argument starts with 7 digits (considered as project number)
	if matched, _ := regexp.MatchString("^[0-9]{7,}", ppathSym); matched {
		ppathSym = filepath.Join(*optsBase, ppathSym, *optsPath)
	} else {
		ppathSym, _ = filepath.Abs(ppathSym)
	}

	// resolve any symlinks on ppath
	ppath, _ = filepath.EvalSymlinks(ppathSym)

	fpinfo, err := ufp.GetFilePathMode(ppath)
	if err != nil {
		log.Fatal(fmt.Sprintf("path not found or unaccessible: %s", ppath))
	}

	roler := acl.GetRoler(*fpinfo)
	if roler == nil {
		log.Fatal(fmt.Sprintf("roler not found: %s", fpinfo.Path))
	}

	log.Debug(fmt.Sprintf("+%v", fpinfo))
	rolesNow, err := roler.GetRoles(*fpinfo)
	if err != nil {
		log.Fatal(fmt.Sprintf("%s: %s", err, fpinfo.Path))
	}

	// check the top-level directory to see if there are actual work to do.
	// if there is a role to remove, n will be larger than 0.
	n := 0
	for r, usersRm := range roles {
		if usersNow, ok := rolesNow[r]; ok {
			// create map for faster user lookup
			umap := make(map[string]bool)
			for _, u := range usersNow {
				umap[u] = true
			}

			for _, u := range usersRm {
				if umap[u] {
					n++
					break
				}
			}
		}

		// break loop if we know there is already some work to do
		if n > 0 {
			break
		}
	}

	if n == 0 && !*optsForce {
		log.Warn("All roles in place, I have nothing to do.")
		os.Exit(0)
	}

	// acquiring operation lock file
	if fpinfo.Mode.IsDir() {
		// acquire lock for the current process
		flock := filepath.Join(ppath, ".prj_setacl.lock")
		if err := ufp.AcquireLock(flock); err != nil {
			log.Fatal(fmt.Sprintf("%s", err))
		}
		defer os.Remove(flock)
	}

	chanS := make(chan os.Signal, 1)
	signal.Notify(chanS, signalHandled...)

	// RoleMap for traverse role removal
	rolesT := make(map[acl.Role][]string)
	rolesT[acl.Traverse] = usersT

	// remove specified user roles
	chanF := ufp.GoFastWalk(ppath, *optsNthreads*4)
	chanOut := goDelRoles(roles, chanF, *optsNthreads)

	// channels for removing traverse roles
	// set traverse roles
	chanFt := goPrintOut(chanOut, *optsTraverse, rolesT, *optsNthreads*4)
	chanOutt := goDelRoles(rolesT, chanFt, *optsNthreads)

	// block main until the output is all printed, or a system signal is received
	select {
	case s := <-chanS:
		log.Warnf("Stopped due to received signal: %s\n", s)
		exitcode = int(s.(syscall.Signal))
		runtime.Goexit()
	case <-goPrintOut(chanOutt, false, nil, 0):
		exitcode = 0
		runtime.Goexit()
	}
}

// parseRoles checks the role specification from the caller on the following things:
//
// 1. The users specified in the roleSpec cannot contain the current user.
func parseRoles(roleSpec map[acl.Role]string) (map[acl.Role][]string, []string, error) {
	roles := make(map[acl.Role][]string)
	var usersT []string
	me, _ := user.Current()
	for r, spec := range roleSpec {
		if spec == "" {
			continue
		}
		roles[r] = strings.Split(spec, ",")
		usersT = append(usersT, roles[r]...)
		for _, u := range roles[r] {
			// cannot change the role for the user himself
			if u == me.Username {
				return nil, nil, errors.New("managing yourself is not permitted: " + u)
			}
		}
	}
	return roles, usersT, nil
}

// goSetRoles performs actions for deleting ACL (defined by roles) on paths provided
// through the chanF channel, in a asynchronous manner. It returns a channel containing
// ACL information of paths on which the ACL deletion is correctly applied.
//
// The returned channel can be passed onto the goPrintOut function for displaying the
// results asynchronously.
func goDelRoles(roles acl.RoleMap, chanF chan ufp.FilePathMode, nthreads int) chan acl.RolePathMap {

	// output channel
	chanOut := make(chan acl.RolePathMap)

	// core function of updating ACL on the given file path
	updateACL := func(f ufp.FilePathMode) {
		// TODO: make the roler depends on path
		roler := acl.GetRoler(f)

		if roler == nil {
			log.Warn(fmt.Sprintf("roler not found: %s", f.Path))
			return
		}

		if rolesNew, err := roler.DelRoles(f, roles, false, false); err == nil {
			chanOut <- acl.RolePathMap{Path: f.Path, RoleMap: rolesNew}
		} else {
			log.Error(fmt.Sprintf("%s: %s", err, f.Path))
		}
	}

	// launch parallel go routines for deleting ACL
	go func() {
		var wg sync.WaitGroup
		wg.Add(nthreads)
		for i := 0; i < nthreads; i++ {
			go func() {
				for f := range chanF {
					log.Debug("processing file: " + f.Path)
					updateACL(f)
				}
				wg.Done()
			}()
		}
		wg.Wait()
		close(chanOut)
	}()

	return chanOut
}

// goPrintOut prints out information of paths on which the new ACL has been applied.
//
// Optionally, it also resolves the paths on which the traverse role has to be set.
// The paths resolved for traverse role can be passed onto the goSetRoles function for
// setting the traverse role.
func goPrintOut(chanOut chan acl.RolePathMap, resolvePathForTraverse bool, rolesT map[acl.Role][]string, bufferChanTraverse int) chan ufp.FilePathMode {

	chanFt := make(chan ufp.FilePathMode, bufferChanTraverse)
	go func() {
		counter := 0
		spinner := ustr.NewSpinner()

		for o := range chanOut {
			counter++
			if *optsSilence {
				// print visited directory/path counter
				switch m := counter % 100; m {
				case 1:
					fmt.Printf("\r %s path visited: %d", spinner.Next(), counter)
				default:
					fmt.Printf("\r %s path visited: %d", spinner.Current(), counter)
				}
			} else {
				// the role has been set to the path
				log.Info(fmt.Sprintf("%s", o.Path))
			}

			for r, users := range o.RoleMap {
				log.Debug(fmt.Sprintf("%12s: %s", r, strings.Join(users, ",")))
			}
			// examine the path to see if it is deviated from the ppath from
			// the project storage perspective.  If so, it should be considered for the
			// traverse role settings.
			if resolvePathForTraverse && !acl.IsSameProjectPath(o.Path, ppath) {
				acl.GetPathsForSetTraverse(o.Path, rolesT, &chanFt)
			}
		}
		// enter a newline when using the silence mode
		if *optsSilence {
			fmt.Printf("\n")
		}
		// examine ppath (and ppathSym if it's not the same as ppath) to resolve possible
		// parents for setting the traverse role.
		if resolvePathForTraverse {
			acl.GetPathsForSetTraverse(ppath, rolesT, &chanFt)
			if ppath != ppathSym {
				acl.GetPathsForSetTraverse(ppathSym, rolesT, &chanFt)
			}
		}
		defer close(chanFt)
	}()

	return chanFt
}
