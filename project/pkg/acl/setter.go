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

// Setter implements high-level functions for setting and deleting
// project roles in a given path.
type Setter struct {
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
func (s Setter) SetRoles() (exitcode int, err error) {

	// map for role specification inputs (commad options)
	roleSpec := make(map[Role]string)
	roleSpec[Manager] = s.Managers
	//roleSpec[Writer] = s.Writers
	roleSpec[Contributor] = s.Contributors
	roleSpec[Viewer] = s.Viewers

	// construct operable map and check duplicated specification
	roles, usersT, err := s.parseRolesForSet(roleSpec)
	if err != nil {
		exitcode = 1
		return
	}

	// resolve any symlinks on ppathSym to actual path this program should work on.
	s.ppath, _ = filepath.EvalSymlinks(s.RootPath)

	fpinfo, err := ufp.GetFilePathMode(s.ppath)
	if err != nil {
		exitcode = 1
		err = fmt.Errorf("path not found or unaccessible: %s", s.ppath)
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
	if n == 0 && !s.Force {
		log.Warnln("All roles in place, I have nothing to do.")
		return
	}

	// acquiring operation lock file
	if fpinfo.Mode.IsDir() {
		// acquire lock for the current process
		flock := filepath.Join(s.ppath, ".prj_setacl.lock")
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
	chanF := ufp.GoFastWalk(s.ppath, s.FollowLink, s.Nthreads*4)
	chanOut := s.goSetRoles(roles, chanF, s.Nthreads)

	// set traverse roles
	chanFt := s.goPrintOut(chanOut, s.Traverse, rolesT, s.Nthreads*4)
	chanOutt := s.goSetRoles(rolesT, chanFt, s.Nthreads)

	// block main until the output is all printed, or a system signal is received
	select {
	case s := <-chanS:
		log.Warnf("Stopped due to received signal: %s\n", s)
		exitcode = int(s.(syscall.Signal))
		return
	case <-s.goPrintOut(chanOutt, false, nil, 0):
		exitcode = 0
		return
	}
}

func (s Setter) DeleteRoles() error {
	return nil
}

// parseRolesForSet checks the role specification from the caller on the following two things:
//
// 1. The users specified in the roleSpec cannot contain the current user.
//
// 2. The same user id cannot appear twice.
func (s Setter) parseRolesForSet(roleSpec map[Role]string) (map[Role][]string, []string, error) {
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

// goSetRoles performs actions for setting ACL (defined by roles) on paths provided
// through the chanF channel, in a asynchronous manner. It returns a channel containing
// ACL information of paths on which the ACL setting is correctly applied.
//
// The returned channel can be passed onto the goPrintOut function for displaying the
// results asynchronously.
func (s Setter) goSetRoles(roles RoleMap, chanF chan ufp.FilePathMode, nthreads int) chan RolePathMap {

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
func (s Setter) goPrintOut(chanOut chan RolePathMap,
	resolvePathForTraverse bool, rolesT map[Role][]string, bufferChanTraverse int) chan ufp.FilePathMode {

	chanFt := make(chan ufp.FilePathMode, bufferChanTraverse)
	go func() {
		counter := 0
		spinner := ustr.NewSpinner()
		for o := range chanOut {
			counter++
			if s.Silence {
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
			if resolvePathForTraverse && !IsSameProjectPath(o.Path, s.ppath) {
				GetPathsForSetTraverse(o.Path, rolesT, &chanFt)
			}
		}
		// enter a newline when using the silence mode
		if s.Silence && counter != 0 {
			fmt.Printf("\n")
		}
		// examine ppath (and RootPath if it's not the same as ppath) to resolve possible
		// parents for setting the traverse role.
		if resolvePathForTraverse {
			GetPathsForSetTraverse(s.ppath, rolesT, &chanFt)
			if s.ppath != s.RootPath {
				GetPathsForSetTraverse(s.RootPath, rolesT, &chanFt)
			}
		}
		defer close(chanFt)
	}()

	return chanFt
}
