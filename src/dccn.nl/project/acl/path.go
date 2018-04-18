package acl

import (
    "path/filepath"
    "strings"
    "os"
    "fmt"
    ufp "dccn.nl/utility/filepath"
)

// IsSameProjectPath checks whether the two given paths are pointing to the same
// project storage directory.
//
// Current implementation assumes that the project directory path is in either of the
// following formats:
//
//      p1 := "/project/3010000.01"       // this is a path of project 3010000.01 on the NetApp filer.
//      p2 := "/project_ext/3010000.01"   // this is a path of project 3010000.01 on external NAS systems, such as FreeNAS, QNAP, etc.
//
// The given paths can be a file within the project storage; however, the check only takes palce on the first
// two directory levels.
func IsSameProjectPath(p1, p2 string) bool {

    p1d := strings.Split(filepath.Clean(p1), string(os.PathSeparator))
    p2d := strings.Split(filepath.Clean(p2), string(os.PathSeparator))

    // check the first 3 path elements
    for i := 0; i < 3; i++ {
        if p1d[i] != p2d[i] {
            return false
        }
    }
    return true
}

// AddPathsForTraverse addes the given path into an existing paths for traverse check.
// The traverse check is an action that the program should look upward its parent directory (i.e. cd ..)
// recursively from a given path until the root directory.
//
// The caller maintains and passes the reference of a slice of paths4traverse to this function.  The slice is
// then updated according to the new path provided.
//
// The new path is walked upward to the root (e.g. "/") and added to the slice when the walked parent is not
// yet presented in the slice. 
func AddPathForTraverse(path string, paths4traverse *[]string) {
    pmap := make(map[string]bool)
    for _, p := range *paths4traverse {
        pmap[p] = true
    }
    
    for {
        path = filepath.Dir(path)
        if path == string(os.PathSeparator) || path == "." { // path matches the root directory, should stop
            break
        }
        if ! pmap[path] {
            *paths4traverse = append(*paths4traverse, path)
        }
    }
}

// GetPathsForSetTraverse resolves the parent paths of p on which the traverse role
// should be set.
//
// It walks upward to either the root (i.e. /) or a path that is associated with
// one of the roler defined in RolerMap, checks on the parent path whether a
// user has to be added with the traverse role.
func GetPathsForSetTraverse(p string, roles RoleMap, chan_f *chan ufp.FilePathMode) {
    // boolean map for traverse role users
    m_usersT := make(map[string]bool)
    for _, u := range roles[Traverse] {
        m_usersT[u] = true
    }

    // function of checking whether users to be added in the traverse role
    // have already an access role.
    userInRole := func(roles_now RoleMap) bool {
        for _, users := range roles_now {
            for _, u := range users {
                if m_usersT[u] {
                    return true
                }
            }
        }
        return false
    }

    for {
        p = filepath.Dir(p)
        // force stopping iteration if root directory is rearched
        if p == string(os.PathSeparator) || p == "." {
            break
        }
        // stops iteration if the toppest level path is matched
        if _, ok := RolerMap[p]; ok {
            break
        }
        fpm   := ufp.FilePathMode{Path:p,Mode:os.ModeDir}
        roler := GetRoler(fpm)
        if roler == nil {
            logger.Warn(fmt.Sprintf("roler not found: %s",p))
            continue
        }
        roles_now, err := roler.GetRoles(fpm)
        if err != nil {
            logger.Warn(fmt.Sprintf("%s: %s",err, p))
        }
        if ! userInRole(roles_now) {
            *chan_f <-fpm
        }
    }
}

// GetPathsForDelTraverse resolves the parent paths of p on which the traverse role
// should be removed.
//
// It walks upward to either the root (i.e. /) or a path that is associated with
// one of the roler defined in RolerMap, checks on the parent path whether a
// user has to be removed from the traverse role.
func GetPathsForDelTraverse(p string, roles RoleMap, chan_f *chan ufp.FilePathMode) {
    // boolean map for traverse role users
    m_usersT := make(map[string]bool)
    for _, u := range roles[Traverse] {
        m_usersT[u] = true
    }

    // function of checking whether users to be removed is exactly in the traverse
    // role.
    userInRole := func(roles_now RoleMap) bool {
        if users, ok := roles_now[Traverse]; ok {
            for _, u := range users {
                if m_usersT[u] {
                    return true
                }
            }
        }
        return false
    }

    for {
        p = filepath.Dir(p)
        // force stopping iteration if root directory is rearched
        if p == string(os.PathSeparator) || p == "." {
            break
        }
        // stops iteration if the toppest level path is matched
        if _, ok := RolerMap[p]; ok {
            break
        }
        fpm   := ufp.FilePathMode{Path:p,Mode:os.ModeDir}
        roler := GetRoler(fpm)
        if roler == nil {
            logger.Warn(fmt.Sprintf("roler not found: %s",p))
            continue
        }
        roles_now, err := roler.GetRoles(fpm)
        if err != nil {
            logger.Warn(fmt.Sprintf("%s: %s",err, p))
        }
        if userInRole(roles_now) {
            *chan_f <-fpm
        }
    }
}