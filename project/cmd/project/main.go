package main

import (
	"fmt"
	"os"
	"os/signal"
	"os/user"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"syscall"

	ufp "github.com/Donders-Institute/tg-toolset-golang/pkg/filepath"
	ustr "github.com/Donders-Institute/tg-toolset-golang/pkg/strings"
	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/acl"
	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/vol"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	// ProjectRootPath defines the filesystem root path of the project storage.
	ProjectRootPath = "/project"
)

var verbose bool
var volManagerAddress string
var uidsManager string
var uidsContributor string
var uidsViewer string
var forceFlag bool
var numThreads int
var followSymlink bool
var silenceFlag bool

// two top-level path variable for resolving parent directories for setting travers role.
var ppath, spath string

var signalHandled = []os.Signal{
	syscall.SIGABRT,
	syscall.SIGHUP,
	syscall.SIGTERM,
	syscall.SIGINT,
}

func init() {

	volCmd.PersistentFlags().StringVarP(
		&volManagerAddress,
		"manager", "m", "filer-a-mi.dccn.nl:22",
		"IP or hostname of the storage's management server",
	)
	volCmd.AddCommand(volCreateCmd)

	roleCmd.PersistentFlags().StringVarP(
		&uidsManager,
		"manager", "m", "",
		"comma-separated system uids to be set as project managers",
	)
	roleCmd.PersistentFlags().StringVarP(
		&uidsContributor,
		"contributor", "c", "",
		"comma-separated system uids to be set as project contributors",
	)
	roleCmd.PersistentFlags().StringVarP(
		&uidsViewer,
		"viewer", "u", "",
		"comma-separated system uids to be set as project viewers",
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
	roleCmd.PersistentFlags().IntVarP(
		&numThreads,
		"nthreads", "n", 8,
		"number of parallel worker threads",
	)

	roleCmd.AddCommand(roleSetCmd)

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.AddCommand(volCmd, roleCmd)
}

var rootCmd = &cobra.Command{
	Use:   "project",
	Short: "Utility CLI for managing DCCN project",
	Long:  ``,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if cmd.Flags().Changed("verbose") {
			log.SetLevel(log.DebugLevel)
		}
	},
}

var volCmd = &cobra.Command{
	Use:   "vol",
	Short: "Manage storage volume for projects",
	Long:  ``,
}

var volCreateCmd = &cobra.Command{
	Use:   "create [projectID] [quotaGiB]",
	Short: "Create storage volume for a project",
	Long:  ``,
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		volManager := vol.NetAppVolumeManager{
			AddressFilerMI: volManagerAddress,
		}

		// parse second argument to integer
		quota, err := strconv.Atoi(args[1])
		if err != nil {
			log.Fatalf("quota value not an integer: %s\n", args[1])
		}

		if err := volManager.Create(args[0], quota); err != nil {
			log.Errorln(err)
		}
	},
}

var roleCmd = &cobra.Command{
	Use:   "role",
	Short: "Manage data access role for projects",
	Long:  ``,
}

var roleSetCmd = &cobra.Command{
	Use:   "set [projectID]",
	Short: "Set data access roles for a project",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		spath = path.Join(ProjectRootPath, args[0])
		if stat, err := os.Stat(spath); os.IsNotExist(err) || !stat.IsDir() {
			return fmt.Errorf("path doesn't exist or not a directory: %s", spath)
		}

		roleSpec := make(map[acl.Role]string)
		roleSpec[acl.Manager] = uidsManager
		roleSpec[acl.Contributor] = uidsContributor
		roleSpec[acl.Viewer] = uidsViewer

		// construct operable map and check duplicated specification
		roles, usersT, err := parseRolesForSet(roleSpec)
		if err != nil {
			return err
		}

		ppath, _ = filepath.EvalSymlinks(spath)
		fpinfo, err := ufp.GetFilePathMode(ppath)
		if err != nil {
			return fmt.Errorf("path not found or unaccessible: %s", ppath)
		}

		// check whether there is a need to set ACL based on the ACL set on ppath.
		roler := acl.GetRoler(*fpinfo)
		if roler == nil {
			return fmt.Errorf("roler not found for path: %s", fpinfo.Path)
		}
		log.Debug(fmt.Sprintf("%+v", fpinfo))
		rolesNow, err := roler.GetRoles(*fpinfo)
		if err != nil {
			return fmt.Errorf("%s: %s", err, fpinfo.Path)
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
		if n == 0 && !forceFlag {
			log.Warnln("All roles in place, I have nothing to do.")
			return nil
		}

		// acquiring operation lock file
		if fpinfo.Mode.IsDir() {
			// acquire lock for the current process
			flock := filepath.Join(ppath, ".prj_setacl.lock")
			if err := ufp.AcquireLock(flock); err != nil {
				return fmt.Errorf("%s", err)
			}
			defer os.Remove(flock)
		}

		// channel for capture handled system signals
		chanS := make(chan os.Signal, 1)
		signal.Notify(chanS, signalHandled...)

		// RoleMap for traverse role
		rolesT := make(map[acl.Role][]string)
		rolesT[acl.Traverse] = usersT

		// set specified user roles
		chanF := ufp.GoFastWalk(ppath, followSymlink, numThreads*4)
		chanOut := goSetRoles(roles, chanF, numThreads)

		// block main until the output is all printed, or a system signal is received
		select {
		case s := <-chanS:
			log.Errorf("Stopped due to received signal: %s\n", s)
			break
		case <-goPrintOut(chanOut, false, nil, 0):
			break
		}

		return nil
	},
}

// parseRolesForSet checks the role specification from the caller on the following two things:
//
// 1. The users specified in the roleSpec cannot contain the current user.
//
// 2. The same user id cannot appear twice.
func parseRolesForSet(roleSpec map[acl.Role]string) (map[acl.Role][]string, []string, error) {
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
				return nil, nil, fmt.Errorf("managing yourself is not permitted: " + u)
			}

			// cannot specify the same user name more than once
			if users[u] {
				return nil, nil, fmt.Errorf("user specified more than once: " + u)
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
func goSetRoles(roles acl.RoleMap, chanF chan ufp.FilePathMode, nthreads int) chan acl.RolePathMap {

	// output channel
	chanOut := make(chan acl.RolePathMap)

	// core function of updating ACL on the given file path
	updateACL := func(f ufp.FilePathMode) {
		// TODO: make the roler depends on path
		roler := acl.GetRoler(f)
		log.Debug(fmt.Sprintf("path: %s %s", f.Path, reflect.TypeOf(roler)))

		if roler == nil {
			log.Warn(fmt.Sprintf("roler not found: %s", f.Path))
			return
		}

		if rolesNew, err := roler.SetRoles(f, roles, false, false); err == nil {
			chanOut <- acl.RolePathMap{Path: f.Path, RoleMap: rolesNew}
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
func goPrintOut(chanOut chan acl.RolePathMap, resolvePathForTraverse bool, rolesT map[acl.Role][]string, bufferChanTraverse int) chan ufp.FilePathMode {

	chanFt := make(chan ufp.FilePathMode, bufferChanTraverse)
	go func() {
		counter := 0
		spinner := ustr.NewSpinner()
		for o := range chanOut {
			counter++
			if silenceFlag {
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
		if silenceFlag && counter != 0 {
			fmt.Printf("\n")
		}
		// examine ppath (and ppathSym if it's not the same as ppath) to resolve possible
		// parents for setting the traverse role.
		if resolvePathForTraverse {
			acl.GetPathsForSetTraverse(ppath, rolesT, &chanFt)
			if ppath != spath {
				acl.GetPathsForSetTraverse(spath, rolesT, &chanFt)
			}
		}
		defer close(chanFt)
	}()

	return chanFt
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Errorln(err)
		os.Exit(1)
	}
}
