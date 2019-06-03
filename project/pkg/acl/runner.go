package acl

import (
	"fmt"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"

	ufp "github.com/Donders-Institute/tg-toolset-golang/pkg/filepath"
	ustr "github.com/Donders-Institute/tg-toolset-golang/pkg/strings"
	log "github.com/sirupsen/logrus"
)

func init() {

}

var signalHandled = []os.Signal{
	syscall.SIGABRT,
	syscall.SIGHUP,
	syscall.SIGTERM,
	syscall.SIGINT,
}

// Runner implements high-level functions for setting and deleting
// project roles in a given path.
type Runner struct {
	// RootPath is the top-level path from which the roles are being set/deleted.
	RootPath string
	// Managers is a comma-separated list of system UIDs to be set as managers or deleted from the manager role.
	Managers string
	// Contributors is a comma-separated list of system UIDs to be set as contributors or deleted from the contributor role.
	Contributors string
	// Viewers is a comma-separated list of system UIDs to be set as viewers or deleted from the viewer role.
	Viewers string
	// Nthreads defines number of workers performing the setting/deleting operation in parallel
	// while walking through the filesystem tree.
	Nthreads int
	// Traverse specifies whether the parent directories of the RootPath should be set with traverse
	// permission.  This flag does not apply to the delete operation.
	Traverse bool
	// Force specifies whether the set/delete action should be performed forcefully even the
	// the RootPath has already the end result of the set/delete action.
	Force bool
	// Silence specifies whether the set/delete action should be performed in silence mode.
	// In silence mode, only the number of files/directories visited is shown.
	Silence bool
	// FollowLink specifies whether the set/delete action should be performed on the target
	// of a symbolic link.
	FollowLink bool

	// ppath is an absolute path evaluated from RootPath.  If RootPath is a symbolic link,
	// the ppath will be pointed to the evaluated target.
	ppath string
}

// SetRoles sets roles recursively for users on a the path defined by RootPath.
func (r *Runner) SetRoles() (exitcode int, err error) {

	// map for role specification inputs (commad options)
	roleSpec := make(map[Role]string)
	roleSpec[Manager] = r.Managers
	//roleSpec[Writer] = r.Writers
	roleSpec[Contributor] = r.Contributors
	roleSpec[Viewer] = r.Viewers

	// construct operable map and check duplicated specification
	roles, usersT, err := r.parseRolesForSet(roleSpec)
	if err != nil {
		exitcode = 1
		return
	}

	// resolve any symlinks on ppathSym to actual path this program should work on.
	r.ppath, _ = filepath.EvalSymlinks(r.RootPath)

	fpinfo, err := ufp.GetFilePathMode(r.ppath)
	if err != nil {
		exitcode = 1
		err = fmt.Errorf("path not found or unaccessible: %s", r.ppath)
		return
	}

	// check whether there is a need to set ACL based on the ACL set on ppath.
	roler := GetRoler(*fpinfo)
	if roler == nil {
		exitcode = 1
		err = fmt.Errorf("roler not found for path: %s", fpinfo.Path)
		return
	}

	log.Debug(fmt.Sprintf("%+v", fpinfo))
	rolesNow, err := roler.GetRoles(*fpinfo)
	if err != nil {
		exitcode = 1
		err = fmt.Errorf("%s: %s", err, fpinfo.Path)
		return
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
	if n == 0 && !r.Force {
		log.Warnln("All roles in place, I have nothing to do.")
		return
	}

	// acquiring operation lock file
	if fpinfo.Mode.IsDir() {
		// acquire lock for the current process
		flock := filepath.Join(r.ppath, ".prj_setacl.lock")
		if err = ufp.AcquireLock(flock); err != nil {
			exitcode = 1
			return
		}
		defer os.Remove(flock)
	}

	chanS := make(chan os.Signal, 1)
	signal.Notify(chanS, signalHandled...)

	// RoleMap for traverse role
	rolesT := make(map[Role][]string)
	rolesT[Traverse] = usersT

	// set specified user roles
	chanF := ufp.GoFastWalk(r.ppath, r.FollowLink, r.Nthreads*4)
	chanOut := r.goSetRoles(roles, chanF, r.Nthreads)

	// set traverse roles
	chanFt := r.goPrintOut(chanOut, r.Traverse, rolesT, r.Nthreads*4)
	chanOutt := r.goSetRoles(rolesT, chanFt, r.Nthreads)

	// block main until the output is all printed, or a system signal is received
	select {
	case s := <-chanS:
		log.Warnf("Stopped due to received signal: %s\n", s)
		exitcode = int(s.(syscall.Signal))
		return
	case <-r.goPrintOut(chanOutt, false, nil, 0):
		exitcode = 0
		return
	}
}

// DeleteRoles removes roles recursively for users on a the path defined by RootPath.
func (r *Runner) DeleteRoles() error {
	return nil
}

// GetRoles retrieves roles for users on a the path defined by RootPath.
// Use the `recursion` argument to enable/disable recursion through filesystem tree.
func (r *Runner) GetRoles(recursion bool) error {
	// resolve any symlinks on ppath
	r.ppath, _ = filepath.EvalSymlinks(r.RootPath)

	fpinfo, err := ufp.GetFilePathMode(r.ppath)
	if err != nil {
		return fmt.Errorf("path not found or unaccessible: %s", r.ppath)
	} else {
		// disable recursion if ppath is not a directory
		if !fpinfo.Mode.IsDir() {
			recursion = false
		}
	}

	var chanD chan ufp.FilePathMode
	nthreads := r.Nthreads
	if recursion {
		chanD = ufp.GoFastWalk(r.ppath, r.FollowLink, nthreads)
	} else {
		nthreads = 1
		chanD = make(chan ufp.FilePathMode)
		go func() {
			chanD <- *fpinfo
			defer close(chanD)
		}()
	}

	chanOut := r.goGetACL(chanD, nthreads)

	for o := range chanOut {
		fmt.Printf("%s:\n", o.Path)
		for _, r := range []Role{Manager, Contributor, Writer, Viewer, Traverse} {
			if users, ok := o.RoleMap[r]; ok {
				fmt.Printf("%12s: %s\n", r, strings.Join(users, ","))
			}
		}
	}
	return nil
}

// parseRolesForSet checks the role specification from the caller on the following two things:
//
// 1. The users specified in the roleSpec cannot contain the current user.
//
// 2. The same user id cannot appear twice.
func (r Runner) parseRolesForSet(roleSpec map[Role]string) (map[Role][]string, []string, error) {
	roles := make(map[Role][]string)
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
				return nil, nil, fmt.Errorf("managing yourself is not permitted: %s", u)
			}

			// cannot specify the same user name more than once
			if users[u] {
				return nil, nil, fmt.Errorf("user specified more than once: %s", u)
			}
			users[u] = true
		}
	}
	return roles, usersT, nil
}

// goGetACL performs getting ACL on walked paths, using a go routine. The result is pushed to
// a channel of acl.RolePathMap.  It also closes the channel when all walked paths are processed.
func (r Runner) goGetACL(chanD chan ufp.FilePathMode, nthreads int) chan RolePathMap {

	// output channel
	chanOut := make(chan RolePathMap)

	// launch parallel go routines for getting ACL
	chanSync := make(chan int)
	for i := 0; i < nthreads; i++ {
		go func() {
			for p := range chanD {

				roler := GetRoler(p)
				if roler == nil {
					log.Warn(fmt.Sprintf("roler not found: %s", p.Path))
					continue
				}
				log.Debug(fmt.Sprintf("path: %s %s", p.Path, reflect.TypeOf(roler)))
				if roles, err := roler.GetRoles(p); err == nil {
					chanOut <- RolePathMap{Path: p.Path, RoleMap: roles}
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

// goSetRoles performs actions for setting ACL (defined by roles) on paths provided
// through the chanF channel, in a asynchronous manner. It returns a channel containing
// ACL information of paths on which the ACL setting is correctly applied.
//
// The returned channel can be passed onto the goPrintOut function for displaying the
// results asynchronously.
func (r Runner) goSetRoles(roles RoleMap, chanF chan ufp.FilePathMode, nthreads int) chan RolePathMap {

	// output channel
	chanOut := make(chan RolePathMap)

	// core function of updating ACL on the given file path
	updateACL := func(f ufp.FilePathMode) {
		// TODO: make the roler depends on path
		roler := GetRoler(f)
		log.Debug(fmt.Sprintf("path: %s %s", f.Path, reflect.TypeOf(roler)))

		if roler == nil {
			log.Warn(fmt.Sprintf("roler not found: %s", f.Path))
			return
		}

		if rolesNew, err := roler.SetRoles(f, roles, false, false); err == nil {
			chanOut <- RolePathMap{Path: f.Path, RoleMap: rolesNew}
		} else {
			log.Error(fmt.Sprintf("%s: %s", err, f.Path))
		}
	}

	// launch parallel go routines for setting ACL
	go func() {
		var wg sync.WaitGroup
		wg.Add(nthreads)
		for i := 0; i < nthreads; i++ {
			go func() {
				for f := range chanF {
					log.Debug("process file: " + f.Path)
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
func (r Runner) goPrintOut(chanOut chan RolePathMap,
	resolvePathForTraverse bool, rolesT map[Role][]string, bufferChanTraverse int) chan ufp.FilePathMode {

	chanFt := make(chan ufp.FilePathMode, bufferChanTraverse)
	go func() {
		counter := 0
		spinner := ustr.NewSpinner()
		for o := range chanOut {
			counter++
			if r.Silence {
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
			if resolvePathForTraverse && !IsSameProjectPath(o.Path, r.ppath) {
				GetPathsForSetTraverse(o.Path, rolesT, &chanFt)
			}
		}
		// enter a newline when using the silence mode
		if r.Silence && counter != 0 {
			fmt.Printf("\n")
		}
		// examine ppath (and RootPath if it's not the same as ppath) to resolve possible
		// parents for setting the traverse role.
		if resolvePathForTraverse {
			GetPathsForSetTraverse(r.ppath, rolesT, &chanFt)
			if r.ppath != r.RootPath {
				GetPathsForSetTraverse(r.RootPath, rolesT, &chanFt)
			}
		}
		defer close(chanFt)
	}()

	return chanFt
}
