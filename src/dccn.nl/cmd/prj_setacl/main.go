package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
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

var logger *log.Entry

// ppathUser stores a path constructed from user input.
// It is used for checking whether a path under it deviated from the path.
// For example, a path "/project/3010000.01/data" has a symlink to "/project/3044022.10/data".
// In this case, the base directory of the referent (i.e. "/project/30440220.10") should be taken
// into account for setting up traverse role in addition to the base of the ppathUser
// (i.e. "/project/3010000.01").
var ppathUser string

func init() {
	optsManager = flag.String("m", "", "specify a comma-separated-list of users for the manager role")
	optsContributor = flag.String("c", "", "specify a comma-separated-list of users for the contributor role")
	optsViewer = flag.String("u", "", "specify a comma-separated-list of users for the viewer role")
	optsTraverse = flag.Bool("t", true, "enable/disable role users to travel through parent directories")
	optsBase = flag.String("d", "/project", "set the root path of project storage")
	optsPath = flag.String("p", "", "set path of a sub-directory in the project folder")
	optsNthreads = flag.Int("n", 4, "set number of concurrent processing threads")
	optsForce = flag.Bool("f", false, "force role setting regardlessly")
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
	fmt.Printf("\n%s sets users' access permission on a given project or a path.\n", filepath.Base(os.Args[0]))
	fmt.Printf("\nUsage: %s [OPTIONS] projectId|path\n", os.Args[0])
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
		logger.Fatal(fmt.Sprintf("unknown project number: %v", args))
	}

	// map for role specification inputs (commad options)
	roleSpec := make(map[acl.Role]string)
	roleSpec[acl.Manager] = *optsManager
	roleSpec[acl.Contributor] = *optsContributor
	roleSpec[acl.Viewer] = *optsViewer

	// construct operable map and check duplicated specification
	roles, usersT, err := parseRoles(roleSpec)
	if err != nil {
		logger.Fatal(fmt.Sprintf("%s", err))
	}

	// constructing the input path from arguments
	ppath := args[0]
	if []rune(ppath)[0] != os.PathSeparator {
		ppath = filepath.Join(*optsBase, ppath, *optsPath)
	}

	// copy over the constructed ppath to ppathUser
	ppathUser = ppath

	fpinfo, err := ufp.GetFilePathMode(ppath)
	if err != nil {
		logger.Fatal(fmt.Sprintf("path not found or unaccessible: %s", ppath))
	}

	// check whether there is a need to set ACL based on the ACL set on ppath.
	roler := acl.GetRoler(*fpinfo)
	if roler == nil {
		logger.Fatal(fmt.Sprintf("roler not found for path: %s", fpinfo.Path))
	}
	logger.Debug(fmt.Sprintf("%+v", fpinfo))
	rolesNow, err := roler.GetRoles(*fpinfo)
	if err != nil {
		logger.Fatal(fmt.Sprintf("%s: %s", err, fpinfo.Path))
	}
	// if there is a new role to set, n will be larger than 0
	n := 0
	for r, users := range roles {
		if _, ok := rolesNow[r]; !ok {
			n++
			break
		}
		ulist := "," + strings.Join(rolesNow[r], ",") + ","
		for _, u := range users {
			if strings.Index(ulist, ","+u+",") < 0 {
				n++
				break
			}
		}
	}
	if n == 0 && !*optsForce {
		logger.Warn("All roles in place, I have nothing to do.")
		os.Exit(0)
	}

	// acquiring operation lock file
	if fpinfo.Mode.IsDir() {
		// acquire lock for the current process
		flock := filepath.Join(ppath, ".prj_setacl.lock")
		if err := ufp.AcquireLock(flock); err != nil {
			logger.Fatal(fmt.Sprintf("%s", err))
		}
		defer os.Remove(flock)
	}

	// sets specified user roles
	chanF := ufp.GoFastWalk(ppath, *optsNthreads*4)
	chanOut := goSetRoles(roles, chanF, *optsNthreads)

	// RoleMap for traverse role
	rolesT := make(map[acl.Role][]string)
	rolesT[acl.Traverse] = usersT

	// channels for setting traverse roles
	chanFt := make(chan ufp.FilePathMode, *optsNthreads*4)
	chanOutt := goSetRoles(rolesT, chanFt, *optsNthreads)

	// loops over results of setting specified user roles and resolves paths
	// on which the traverse role should be set, using a go routine.
	go func() {
		n := 0
		for o := range chanOut {
			// the role has been set to the path
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
					acl.GetPathsForSetTraverse(o.Path, rolesT, &chanFt)
					n--
				}()
			}
		}
		// wait until all go routines for acl.GetPathsForSetTraverse to finish
		for {
			if n == 0 {
				break
			}
		}
		// go over ppath for traverse role synchronously
		if *optsTraverse {
			acl.GetPathsForSetTraverse(ppath, rolesT, &chanFt)
		}
		defer close(chanFt)
	}()

	// loops over results of setting the traverse role.
	for o := range chanOutt {
		logger.Info(fmt.Sprintf("%s", o.Path))
		for r, users := range o.RoleMap {
			logger.Debug(fmt.Sprintf("%12s: %s", r, strings.Join(users, ",")))
		}
	}
}

// parseRoles checks the role specification from the caller on the following two things:
//
// 1. The users specified in the roleSpec cannot contain the current user.
//
// 2. The same user id cannot appear twice.
func parseRoles(roleSpec map[acl.Role]string) (map[acl.Role][]string, []string, error) {
	roles := make(map[acl.Role][]string)
	users := make(map[string]bool)

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

			// cannot specify the same user name more than once
			if users[u] {
				return nil, nil, errors.New("user specified more than once: " + u)
			}
			users[u] = true
		}
	}
	return roles, usersT, nil
}

/*
   performs actual setacl in a concurrent way.
*/
func goSetRoles(roles acl.RoleMap, chanF chan ufp.FilePathMode, nthreads int) chan acl.RolePathMap {

	// output channel
	chanOut := make(chan acl.RolePathMap)

	// core function of updating ACL on the given file path
	updateACL := func(f ufp.FilePathMode) {
		// TODO: make the roler depends on path
		roler := acl.GetRoler(f)
		logger.Debug(fmt.Sprintf("path: %s %s", f.Path, reflect.TypeOf(roler)))

		if roler == nil {
			logger.Warn(fmt.Sprintf("roler not found: %s", f.Path))
			return
		}

		if rolesNew, err := roler.SetRoles(f, roles, false, false); err == nil {
			chanOut <- acl.RolePathMap{Path: f.Path, RoleMap: rolesNew}
		} else {
			logger.Error(fmt.Sprintf("%s: %s", err, f.Path))
		}
	}

	// launch parallel go routines for setting ACL
	chanSync := make(chan int)
	for i := 0; i < nthreads; i++ {
		go func() {
			for f := range chanF {
				logger.Debug("process file: " + f.Path)
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
