package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"

	"dccn.nl/project/acl"
	ufp "dccn.nl/utility/filepath"
	ustr "dccn.nl/utility/strings"
	log "github.com/sirupsen/logrus"
)

var optsBase *string
var optsPath *string
var optsManager *string
var optsContributor *string
var optsViewer *string
var optsTraverse *bool
var optsNthreads *int
var optsForce *bool
var optsVerbose *bool
var ppathUser string

var logger *log.Entry

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

	logger = log.WithFields(log.Fields{"source": filepath.Base(os.Args[0])})
}

func usage() {
	fmt.Printf("\n%s removes users' access permission on a given project or a path.\n", filepath.Base(os.Args[0]))
	fmt.Printf("\nUsage: %s [OPTIONS] projectId|path\n", os.Args[0])
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
		logger.Fatal(fmt.Sprintf("unknown project number: %v", args))
	}

	if len(args) >= 2 && *optsManager+*optsContributor+*optsViewer != "" {
		flag.Usage()
		logger.Fatal("use only one way to specify users: with or without role options (-m|-c|-u), not both.")
	}

	// map for role specification inputs (commad options)
	roleSpec := make(map[acl.Role]string)

	ppath := args[0]

	if len(args) >= 2 {
		roleSpec[acl.Manager] = args[0]
		roleSpec[acl.Contributor] = args[0]
		roleSpec[acl.Viewer] = args[0]
		roleSpec[acl.Traverse] = args[0]
		ppath = args[1]
	} else {
		roleSpec[acl.Manager] = *optsManager
		roleSpec[acl.Contributor] = *optsContributor
		roleSpec[acl.Viewer] = *optsViewer
	}

	// construct operable map and check duplicated specification
	roles, usersT, err := parseRoles(roleSpec)

	if err != nil {
		logger.Fatal(fmt.Sprintf("%s", err))
	}

	doLock := false

	// the input argument starts with 7 digits (considered as project number)
	if matched, _ := regexp.MatchString("^[0-9]{7,}", ppath); matched {
		ppath = filepath.Join(*optsBase, ppath, *optsPath)
	} else {
		ppath, _ = filepath.Abs(ppath)
	}

	// resolve any symlinks on ppath
	ppath, _ = filepath.EvalSymlinks(ppath)

	// copy over the constructed ppath to ppathUser
	ppathUser = ppath

	fpinfo, err := ufp.GetFilePathMode(ppath)
	if err != nil {
		logger.Fatal(fmt.Sprintf("path not found or unaccessible: %s", ppath))
	}

	roler := acl.GetRoler(*fpinfo)
	if roler == nil {
		logger.Fatal(fmt.Sprintf("roler not found: %s", fpinfo.Path))
	}

	logger.Debug(fmt.Sprintf("+%v", fpinfo))
	rolesNow, err := roler.GetRoles(*fpinfo)
	if err != nil {
		logger.Fatal(fmt.Sprintf("%s: %s", err, fpinfo.Path))
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
		logger.Warn("All roles in place, I have nothing to do.")
		os.Exit(0)
	}

	if doLock {
		// acquire lock for the current process
		flock := filepath.Join(ppath, ".prj_setacl.lock")
		if err := ufp.AcquireLock(flock); err != nil {
			logger.Fatal(fmt.Sprintf("%s", err))
		}
		defer os.Remove(flock)
	}

	// remove specified user roles
	chanF := ufp.GoFastWalk(ppath, *optsNthreads*4)
	chanOut := goDelRoles(roles, chanF, *optsNthreads)

	// RoleMap for traverse role removal
	rolesT := make(map[acl.Role][]string)
	rolesT[acl.Traverse] = usersT

	// channels for removing traverse roles
	chanFt := make(chan ufp.FilePathMode, *optsNthreads*4)
	chanOutt := goDelRoles(rolesT, chanFt, *optsNthreads)

	// loops over results of removing specified user roles and resolves paths
	// on which the traverse role should be removed, using a go routine.
	go func() {
		n := 0
		for o := range chanOut {
			logger.Info(fmt.Sprintf("%s", o.Path))
			for r, users := range o.RoleMap {
				logger.Debug(fmt.Sprintf("%12s: %s", r, strings.Join(users, ",")))
			}
			// examine the path to see if the path is derived from the ppathUser from
			// the project storage perspective.  If so, it should be considered for the
			// traverse role settings.
			if *optsTraverse && !acl.IsSameProjectPath(o.Path, ppathUser) {
				n++
				go func() {
					acl.GetPathsForDelTraverse(o.Path, rolesT, &chanFt)
					n--
				}()
			}
		}
		// wait until all go routines for acl.GetPathsForDelTraverse to finish
		for {
			if n == 0 {
				break
			}
		}
		// go over ppath for traverse role synchronously
		if *optsTraverse {
			acl.GetPathsForDelTraverse(ppath, rolesT, &chanFt)
		}
		defer close(chanFt)
	}()

	// loops over results of removing the traverse role.
	for o := range chanOutt {
		logger.Info(fmt.Sprintf("%s", o.Path))
		for r, users := range o.RoleMap {
			logger.Debug(fmt.Sprintf("%12s: %s", r, strings.Join(users, ",")))
		}
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

/*
   performs actual setacl in a concurrent way
*/
func goDelRoles(roles acl.RoleMap, chanF chan ufp.FilePathMode, nthreads int) chan acl.RolePathMap {

	// output channel
	chanOut := make(chan acl.RolePathMap)

	// core function of updating ACL on the given file path
	updateACL := func(f ufp.FilePathMode) {
		// TODO: make the roler depends on path
		roler := acl.GetRoler(f)

		if roler == nil {
			logger.Warn(fmt.Sprintf("roler not found: %s", f.Path))
			return
		}

		if rolesNew, err := roler.DelRoles(f, roles, false, false); err == nil {
			chanOut <- acl.RolePathMap{Path: f.Path, RoleMap: rolesNew}
		} else {
			logger.Error(fmt.Sprintf("%s: %s", err, f.Path))
		}
	}

	// launch parallel go routines for getting ACL
	chanSync := make(chan int)
	for i := 0; i < nthreads; i++ {
		go func() {
			for f := range chanF {
				logger.Debug("processing file: " + f.Path)
				updateACL(f)
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
