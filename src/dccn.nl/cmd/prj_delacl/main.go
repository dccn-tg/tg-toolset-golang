package main

import (
    "fmt"
    "flag"
    "errors"
    "strings"
    log "github.com/sirupsen/logrus"
    "path/filepath"
    "os"
    "os/user"
    "dccn.nl/project/acl"
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
var ppath_user string

var logger *log.Entry

func init() {
    opts_manager     = flag.String("m", "", "specify a comma-separated-list of users to be removed from the manager role")
    opts_contributor = flag.String("c", "", "specify a comma-separated-list of users to be removed from the contributor role")
    opts_viewer      = flag.String("u", "", "specify a comma-separated-list of users to be removed from the viewer role")
    opts_traverse    = flag.Bool("t", false, "remove users' traverse permission from the parent directories")
    opts_base        = flag.String("d", "/project", "set the root path of project storage")
    opts_path        = flag.String("p", "", "set path of a sub-directory in the project folder")
    opts_nthreads    = flag.Int("n", 2, "set number of concurrent processing threads")
    opts_force       = flag.Bool("f", false, "force the deletion regardlessly")
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
    fmt.Printf("\n%s removes users' access permission on a given project or a path.\n", filepath.Base(os.Args[0]))
    fmt.Printf("\nUsage: %s [OPTIONS] projectId|path\n", os.Args[0])
    fmt.Printf("\nOPTIONS:\n")
    flag.PrintDefaults()
    fmt.Printf("\nEXAMPLES:\n")
    fmt.Printf("\n%s\n", ustr.StringWrap("Removing users 'honlee' and 'edwger' from accessing the project 3010000.01", 80))
    fmt.Printf("\n  %s honlee,edwger 3010000.01\n", os.Args[0])
    fmt.Printf("\n%s\n", ustr.StringWrap("Removing users 'honlee' and 'edwger' from the 'contributor' role on project 3010000.01", 80))
    fmt.Printf("\n  %s -c honlee,edwger 3010000.01\n", os.Args[0])
    fmt.Printf("\n%s\n", ustr.StringWrap("Removing users 'honlee' and 'edwger' from accessing files and directories under a specific path",80))
    fmt.Printf("\n  %s honlee,edwger /project/3010000.01/data_dir\n", os.Args[0])
    fmt.Printf("\n%s\n", ustr.StringWrap("Removing users 'honlee' and 'edwger' from accessing files and directories under a specific path, and the traverse permission on its parent directories",80))
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

    if len(args) >= 2 && *opts_manager + *opts_contributor + *opts_viewer != "" {
        flag.Usage()
        logger.Fatal("use only one way to specify users: with or without role options (-m|-c|-u), not both.")
    }

    // map for role specification inputs (commad options)
    roleSpec := make(map[acl.Role]string)

    ppath := args[0]

    if len(args) >= 2 {
        roleSpec[acl.Manager]     = args[0]
        roleSpec[acl.Contributor] = args[0]
        roleSpec[acl.Viewer]      = args[0]
        roleSpec[acl.Traverse]    = args[0]
        ppath = args[1]
    } else {
        roleSpec[acl.Manager]     = *opts_manager
        roleSpec[acl.Contributor] = *opts_contributor
        roleSpec[acl.Viewer]      = *opts_viewer
    }

    // construct operable map and check duplicated specification
    roles, usersT, err := parseRoles(roleSpec)

    if err != nil {
        logger.Fatal(fmt.Sprintf("%s", err))
    }

    doLock := false

    if []rune(ppath)[0] != os.PathSeparator {
        ppath = filepath.Join(*opts_base, ppath, *opts_path)
    }

    // copy over the constructed ppath to ppath_user
    ppath_user = ppath

    fpinfo, err := ufp.GetFilePathMode(ppath)
    if err != nil {
        logger.Fatal(fmt.Sprintf("path not found or unaccessible: %s", ppath))
    }

    roler := acl.GetRoler(*fpinfo)
    if roler == nil {
        logger.Fatal(fmt.Sprintf("roler not found: %s", fpinfo.Path))
    }

    logger.Debug(fmt.Sprintf("+%v", fpinfo))
    roles_now, err := roler.GetRoles(*fpinfo)
    if err != nil {
        logger.Fatal(fmt.Sprintf("%s: %s", err, fpinfo.Path))
    }

    // check the top-level directory to see if there are actual work to do.
    // if there is a role to remove, n will be larger than 0.
    n := 0
    for r, users_rm := range roles {
        if users_now, ok := roles_now[r]; ok {
            // create map for faster user lookup
            umap := make(map[string]bool)
            for _, u := range users_now {
                umap[u] = true
            }

            for _, u := range users_rm {
                if umap[u] {
                    n += 1
                    break
                }
            }
        }

        // break loop if we know there is already some work to do
        if n > 0 {
            break
        }
    }

    if n == 0 && ! *opts_force {
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
    chan_f := ufp.GoFastWalk(ppath, *opts_nthreads * 4)
    chan_out := goDelRoles(roles, chan_f, *opts_nthreads)

    // RoleMap for traverse role removal
    rolesT := make(map[acl.Role][]string)
    rolesT[acl.Traverse] = usersT

    // channels for removing traverse roles
    chan_ft   := make(chan ufp.FilePathMode, *opts_nthreads * 4)
    chan_outt := goDelRoles(rolesT, chan_ft, *opts_nthreads)

    // loops over results of removing specified user roles and resolves paths
    // on which the traverse role should be removed, using a go routine.
    go func() {
        n := 0
        for o := range chan_out {
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
                    acl.GetPathsForDelTraverse(o.Path, rolesT, &chan_ft)
                    n-=1
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
        if ( *opts_traverse ) {
            acl.GetPathsForDelTraverse(ppath, rolesT, &chan_ft)
        }
        defer close(chan_ft)
    }()

    // loops over results of removing the traverse role.
    for o := range chan_outt {
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
        usersT = append( usersT, roles[r]... )
        for _, u := range roles[r] {
            // cannot change the role for the user himself
            if u == me.Username {
                return nil,nil,errors.New("managing yourself is not permitted: " + u)
            }
        }
    }
    return roles,usersT,nil
}

/*
    performs actual setacl in a concurrent way
*/
func goDelRoles(roles acl.RoleMap, chan_f chan ufp.FilePathMode, nthreads int) (chan acl.RolePathMap) {

    // output channel
    chan_out := make(chan acl.RolePathMap)

    // core function of updating ACL on the given file path
    updateAcl := func(f ufp.FilePathMode) {
        // TODO: make the roler depends on path
        roler := acl.GetRoler(f)

        if roler == nil {
            logger.Warn(fmt.Sprintf("roler not found: %s", f.Path))
            return
        }

        if roles_new, err := roler.DelRoles(f, roles, false, false); err == nil {
            chan_out <-acl.RolePathMap{Path: f.Path, RoleMap: roles_new}
        } else {
            logger.Error(fmt.Sprintf("%s: %s", err, f.Path))
        }
    }

    // launch parallel go routines for getting ACL
    chan_sync := make(chan int)
    for i := 0; i < nthreads; i++ {
        go func() {
            for f := range chan_f {
                logger.Debug("processing file: " + f.Path)
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
