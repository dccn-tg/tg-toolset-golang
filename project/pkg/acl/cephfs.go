package acl

import (
	"bufio"
	"fmt"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/pkg/xattr"

	ufp "github.com/Donders-Institute/tg-toolset-golang/pkg/filepath"
	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
)

// CephFsRoler implements roler interface for the CephFS.
type CephFsRoler struct{}

// GetRoles implements interface for getting user roles on a given path mounted to
// an endpoint of the CephFS.
func (CephFsRoler) GetRoles(pinfo ufp.FilePathMode) (RoleMap, error) {

	rmap := map[Role][]string{
		Manager:     {},
		Contributor: {},
		Viewer:      {},
		Traverse:    {},
	}

	// skip the permission mask
	aces, _, err := getfacl(pinfo.Path)
	if err != nil {
		return rmap, err
	}

	for _, ace := range aces {
		log.Debugf("%s", ace)
		r := ace.ToRole()
		rmap[r] = append(rmap[r], ace.Qualifier)
	}

	return rmap, nil
}

// SetRoles implements interface for setting user roles to a given path mounted to
// an endpoint of the CephFS.
func (CephFsRoler) SetRoles(pinfo ufp.FilePathMode, roles RoleMap, recursive bool, followLink bool) (RoleMap, error) {
	return nil, fmt.Errorf("not implemented")
}

// DelRoles implements interface for removing users from the specified roles on a path
// mounted to an endpoint of the CephFS.
func (CephFsRoler) DelRoles(pinfo ufp.FilePathMode, roles RoleMap, recursive bool, followLink bool) (RoleMap, error) {
	return nil, fmt.Errorf("not implemented")
}

// PosixACE is the posix-style access-control entry
type PosixACE struct {
	Tag        string
	Qualifier  string
	Permission string
	path       string
}

// ToRole maps the permission to project role.
func (ace PosixACE) ToRole() Role {
	var role Role

	switch ace.Permission {
	case "--x":
		role = Traverse
	case "r-x":
		role = Viewer
	case "r--":
		role = Viewer
	case "rwx":
		role = Contributor
		if isManager(ace.path, ace.Qualifier) {
			role = Manager
		}
	case "rw-":
		role = Contributor
		if isManager(ace.path, ace.Qualifier) {
			role = Manager
		}
	default:
	}

	return role
}

// isManager checks if the given `username` is listed as a manager in the extended attribute
// `user.project.managers` of the given `path` and its predecending paths up to the
// project's top directory.
//
// It will check on current user if the `username` is an empty string.
func isManager(path, username string) bool {

	out := false

	if username == "" {
		me, err := user.Current()
		if err != nil {
			log.Errorf("cannot get current  user: %s", err)
			return out
		}
		username = me.Username
	}

	for {

		d, err := xattr.Get(path, "user.project.managers")
		//d, err := getfattr(path, "user.project.managers")
		if err != nil {
			// use debugf since it is fine that files/sub-directories do not have
			// the `user.project.managers` attribute.
			log.Debugf("cannot get manager list of %s: %s", path, err)
		}

		log.Debugf("manager list of %s: %s", path, d)

		// found current user on the manager list.
		if strings.Contains(string(d), username) {
			out = true
			break
		}

		// move to parent directory
		path = filepath.Dir(path)

		// path reaches one of the root directories in which projects
		// are organized.
		if _, ok := RolerMap[path]; ok {
			break
		}

		// path reaches the absolute or relative root.
		if path == "/" || path == "." || path == ".." {
			break
		}
	}

	return out
}

// getfattr is a wrapper of `getfattr` command to get extended attribute of
// `key` associated with the given `path`.
func getfattr(path, key string) ([]byte, error) {

	// find size.
	size, err := syscall.Getxattr(path, key, nil)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, size)
	// Read into buffer of that size.
	read, err := syscall.Getxattr(path, key, buf)
	if err != nil {
		return nil, err
	}
	return buf[:read], nil

	// out := ""
	// cmd := exec.Command("getfattr", "-n", key, "--only-values", path)

	// stdout, err := cmd.Output()
	// if err != nil {
	// 	return out, err
	// }
	// return stdout, nil
}

// getfacl is a wrapper of `getfacl` command and returns only the
// non-default extended ACEs applied on the `path`.
func getfacl(path string) ([]PosixACE, string, error) {

	out := []PosixACE{}

	mask := ""

	cmd := exec.Command("getfacl", "--omit-header", path)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return out, mask, err
	}

	if err = cmd.Start(); err != nil {
		return out, mask, err
	}

	outScanner := bufio.NewScanner(stdout)
	outScanner.Split(bufio.ScanLines)

	for outScanner.Scan() {
		l := outScanner.Text()

		// skip lines starts with `#` or `default` or empty lines
		if strings.HasPrefix(l, "#") || strings.HasPrefix(l, "default") || l == "" {
			continue
		}

		// trim the effective permission and split the ACE fields
		d := strings.Split(strings.Split(l, "\t")[0], ":")

		// skip entry where the qualifier is empty or invalid
		if len(d) < 3 {
			log.Warnf("cannot parse ACL entry: %s", l)
		}

		if d[1] == "" {
			continue
		}

		switch d[0] {
		case "user":
			if _, err := user.Lookup(d[1]); err != nil {
				continue
			}
		case "group":
			if _, err := user.LookupGroup(d[1]); err != nil {
				continue
			}
		case "mask":
			mask = d[2]
			continue
		default:
			continue
		}
		out = append(out, PosixACE{
			Tag:        d[0],
			Qualifier:  d[1],
			Permission: d[2],
			path:       path,
		})
	}

	if err = outScanner.Err(); err != nil {
		log.Errorf("error reading output of command: %s", err)
	}

	// wait the cmd to finish and the IO pipes are closed.
	// write out error if the command execution is failed.
	if err = cmd.Wait(); err != nil {
		log.Errorf("%s fail: %s", cmd.String(), err)
	}

	return out, mask, nil
}
