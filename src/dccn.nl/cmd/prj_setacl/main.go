package main

import (
    "fmt"
    "flag"
    "errors"
    "strings"
    "reflect"
    "path/filepath"
    "os"
    "os/user"
    "dccn.nl/project/acl"
    log "github.com/sirupsen/logrus"
    ufp "dccn.nl/utility/filepath"
    ustr "dccn.nl/utility/strings"
)

var opts_base *string
var opts_path *string
var opts_manager *string
var opts_contributor *string
var opts_viewer *string
var opts_traverse *bool
var opts_nthreads *int
var opts_force *bool
var opts_verbose *bool

var logger *log.Entry

// ppath_user stores a path constructed from user input.
// It is used for checking whether a path under it deviated from the path.
// For example, a path "/project/3010000.01/data" has a symlink to "/project/3044022.10/data".
// In this case, the base directory of the referent (i.e. "/project/30440220.10") should be taken
// into account for setting up traverse role in addition to the base of the ppath_user
// (i.e. "/project/3010000.01").
var ppath_user string

func init() {
    opts_manager     = flag.String("m", "", "specify a comma-separated-list of users for the manager role")
    opts_contributor = flag.String("c", "", "specify a comma-separated-list of users for the contributor role")
    opts_viewer      = flag.String("u", "", "specify a comma-separated-list of users for the viewer role")
    opts_traverse    = flag.Bool("t", true, "enable/disable role users to travel through parent directories")
    opts_base        = flag.String("d", "/project", "set the root path of project storage")
    opts_path        = flag.String("p", "", "set path of a sub-directory in the project folder")
    opts_nthreads    = flag.Int("n", 4, "set number of concurrent processing threads")
    opts_force       = flag.Bool("f", false, "force role setting regardlessly")
    opts_verbose     = flag.Bool("v", false, "print debug messages")

    flag.Usage = usage

    flag.Parse()

    // set logging
    log.SetOutput(os.Stderr)

    // set logging level
    llevel := log.InfoLevel
    if *opts_verbose {
        llevel = log.DebugLevel
    }
    log.SetLevel(llevel)

    logger = log.WithFields(log.Fields{"source":filepath.Base(os.Args[0])})
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
    fmt.Printf("\n%s\n", ustr.StringWrap("Adding or setting users 'honlee' and 'edwger' to the 'contributor' role on a specific path, and allowing the two users to traverse through the parent directories",80))
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
    roleSpec[acl.Manager]     = *opts_manager
    roleSpec[acl.Contributor] = *opts_contributor
    roleSpec[acl.Viewer]      = *opts_viewer

    // construct operable map and check duplicated specification
    roles, usersT, err := parseRoles(roleSpec)
    if err != nil {
        logger.Fatal(fmt.Sprintf("%s", err))
    }

    // constructing the input path from arguments
    ppath := args[0]
    if []rune(ppath)[0] != os.PathSeparator {
        ppath = filepath.Join(*opts_base, ppath, *opts_path)
    }

    // copy over the constructed ppath to ppath_user
    ppath_user = ppath

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
    roles_now, err := roler.GetRoles(*fpinfo)
    if err != nil {
        logger.Fatal(fmt.Sprintf("%s: %s", err, fpinfo.Path))
    }
    // if there is a new role to set, n will be larger than 0
    n := 0
    for r, users := range roles {
        if _, ok := roles_now[r]; ! ok {
            n += 1
            break
        }
        ulist := "," + strings.Join(roles_now[r], ",") + ","
        for _, u := range users {
            if strings.Index(ulist, "," + u + ",") < 0 {
                n += 1
                break
            }
        }
    }
    if n == 0 && ! *opts_force {
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
    chan_f := ufp.GoFastWalk(ppath, *opts_nthreads * 4)
    chan_out := goSetRoles(roles, chan_f, *opts_nthreads)

    // RoleMap for traverse role
    rolesT := make(map[acl.Role][]string)
    rolesT[acl.Traverse] = usersT

    // channels for setting traverse roles
    chan_ft   := make(chan ufp.FilePathMode, *opts_nthreads * 4)
    chan_outt := goSetRoles(rolesT, chan_ft, *opts_nthreads)

    // loops over results of setting specified user roles and resolves paths
    // on which the traverse role should be set, using a go routine.
    go func() {
        n := 0
        for o := range chan_out {
            // the role has been set to the path
            logger.Info(fmt.Sprintf("%s", o.Path))
            for r, users := range o.RoleMap {
                logger.Debug(fmt.Sprintf("%12s: %s", r, strings.Join(users, ",")))
            }
            // examine the path to see if the path is derived from the ppath_user from
            // the project storage perspective.  If so, it should be considered for the
            // traverse role settings.
            if *opts_traverse && ! acl.IsSameProjectPath(o.Path, ppath_user) {
                n+=1
                go func() {
                    acl.GetPathsForSetTraverse(o.Path, rolesT, &chan_ft)
                    n-=1
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
        if *opts_traverse {
            acl.GetPathsForSetTraverse(ppath, rolesT, &chan_ft)
        }
        defer close(chan_ft)
    }()

    // loops over results of setting the traverse role.
    for o := range chan_outt {
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
        usersT = append( usersT, roles[r]... )
        for _, u := range roles[r] {

            // cannot change the role for the user himself
            if u == me.Username {
                return nil,nil,errors.New("managing yourself is not permitted: " + u)
            }

            // cannot specify the same user name more than once
            if users[u] {
                return nil,nil,errors.New("user specified more than once: " + u)
            }
            users[u] = true
        }
    }
    return roles,usersT,nil
}

/*
    performs actual setacl in a concurrent way.
*/
func goSetRoles(roles acl.RoleMap, chan_f chan ufp.FilePathMode, nthreads int) (chan acl.RolePathMap) {

    // output channel
    chan_out := make(chan acl.RolePathMap)

    // core function of updating ACL on the given file path
    updateAcl := func(f ufp.FilePathMode) {
        // TODO: make the roler depends on path
        roler := acl.GetRoler(f)
        logger.Debug(fmt.Sprintf("path: %s %s", f.Path, reflect.TypeOf(roler)))

        if roler == nil {
            logger.Warn(fmt.Sprintf("roler not found: %s", f.Path))
            return
        }

        if roles_new, err := roler.SetRoles(f, roles, false, false); err == nil {
            chan_out <-acl.RolePathMap{Path: f.Path, RoleMap: roles_new}
        } else {
            logger.Error(fmt.Sprintf("%s: %s", err, f.Path))
        }
    }

    // launch parallel go routines for setting ACL
    chan_sync := make(chan int)
    for i := 0; i < nthreads; i++ {
        go func() {
            for f := range chan_f {
                logger.Debug("process file: " + f.Path)
                updateAcl(f)
            }
            chan_sync <-1
        }()
    }

    // launch synchronise go routine
    go func() {
        i := 0
        for {
            i = i + <-chan_sync
            if i == nthreads {
                break
            }
        }
        close(chan_out)
    }()

    return chan_out
}
