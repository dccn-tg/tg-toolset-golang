package acl

import (
	"bufio"
	"fmt"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"github.com/pkg/xattr"

	ufp "github.com/Donders-Institute/tg-toolset-golang/pkg/filepath"
	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
)

// file attribute for registering managers
const fattrManagers string = "trusted.managers"

// CephFsRoler implements roler interface for the CephFS.
type CephFsRoler struct{}

// GetRoles implements interface for getting user roles on a given path mounted to
// an endpoint of the CephFS.
func (CephFsRoler) GetRoles(pinfo ufp.FilePathMode) (RoleMap, error) {

	// make pinfo.Path "clean"
	pinfo.Path = filepath.Clean(pinfo.Path)

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
func (r CephFsRoler) SetRoles(pinfo ufp.FilePathMode, roles RoleMap, recursive bool, followLink bool) (RoleMap, error) {

	// make pinfo.Path "clean"
	pinfo.Path = filepath.Clean(pinfo.Path)

	// don't recalculate the mask.
	args := []string{"-n"}

	// recursion
	if recursive && pinfo.Mode.IsDir() {
		args = append(args, "-R")
	}

	// compose the -m argument
	marg := ""
	noManagers := []string{}
	for r, users := range roles {
		switch r {
		case Manager:
			for _, u := range users {
				if pinfo.Mode.IsDir() {
					marg = fmt.Sprintf("u:%s:rwX,d:u:%s:rwX,%s", u, u, marg)
				} else {
					marg = fmt.Sprintf("u:%s:rwX,%s", u, marg)
				}
			}
		case Contributor:
			noManagers = append(noManagers, users...)
			for _, u := range users {
				if pinfo.Mode.IsDir() {
					marg = fmt.Sprintf("u:%s:rwX,d:u:%s:rwX,%s", u, u, marg)
				} else {
					marg = fmt.Sprintf("u:%s:rwX,%s", u, marg)
				}
			}
		case Viewer:
			noManagers = append(noManagers, users...)
			for _, u := range users {
				if pinfo.Mode.IsDir() {
					marg = fmt.Sprintf("u:%s:rX,d:u:%s:rX,%s", u, u, marg)
				} else {
					marg = fmt.Sprintf("u:%s:rX,%s", u, marg)
				}
			}
		case Traverse:
			noManagers = append(noManagers, users...)
			for _, u := range users {
				if pinfo.Mode.IsDir() {
					marg = fmt.Sprintf("d:u:%s:--x,%s", u, marg)
				}
			}
		default:
			noManagers = append(noManagers, users...)
		}
	}

	if marg == "" {
		log.Debugf("empty -m argument, skip setfacl.")
		return r.GetRoles(pinfo)
	}

	args = append(args, "-m", marg)
	log.Debugf("setfacl -m arguments: %s", marg)

	if err := setfacl(pinfo.Path, args); err != nil {
		log.Errorf("%s", err)
	} else {
		// set fattrManagers for the newly added managers.
		r.setManagers(pinfo.Path, roles[Manager])
		// del fattrManagers in case managers are downgraded to other roles.
		r.delManagers(pinfo.Path, noManagers)
	}

	return r.GetRoles(pinfo)
}

// DelRoles implements interface for removing users from the specified roles on a path
// mounted to an endpoint of the CephFS.
func (r CephFsRoler) DelRoles(pinfo ufp.FilePathMode, roles RoleMap, recursive bool, followLink bool) (RoleMap, error) {

	// make pinfo.Path "clean"
	pinfo.Path = filepath.Clean(pinfo.Path)

	// don't recalculate the mask.
	args := []string{"-n"}

	// recursion
	if recursive && pinfo.Mode.IsDir() {
		args = append(args, "-R")
	}

	// compose the -x argument
	marg := ""
	noManagers := []string{}
	for _, users := range roles {
		log.Debugf("%+v", users)
		noManagers = append(noManagers, users...)
		for _, u := range users {
			marg = fmt.Sprintf("u:%s,d:u:%s,%s", u, u, marg)
		}
	}
	if marg == "" {
		log.Debugf("empty -x argument, skip setfacl.")
		return r.GetRoles(pinfo)
	}

	args = append(args, "-x", marg)
	log.Debugf("setfacl -x arguments: %s", marg)

	if err := setfacl(pinfo.Path, args); err != nil {
		log.Errorf("%s", err)
	} else {
		// del fattrManagers in case managers are downgraded to other roles.
		r.delManagers(pinfo.Path, noManagers)
	}

	return r.GetRoles(pinfo)
}

// setManagers sets list of `users` into the `user.project.managers` file
// attribute of the `path`.
func (r CephFsRoler) setManagers(path string, users []string) {

	// get managers already in the user.project.managers file attribute.
	d, err := xattr.Get(path, fattrManagers)
	if err != nil {
		// use debugf since it is fine that files/sub-directories do not have
		// the `user.project.managers` attribute.
		log.Debugf("cannot get manager list of %s: %s", path, err)
	}
	m := strings.Split(string(d), ",")

	// construct a new list of managers to be set on this path.
	// Note: if the user is already a manager of the parent path, it will not
	// be added to the manager of this path.
	for _, u := range users {
		if !isManager(path, u) {
			m = append(m, u)
		}
	}

	// set fattrManagers file attribute with the new list of managers
	if err := xattr.Set(path, fattrManagers, []byte(strings.Join(m, ","))); err != nil {
		log.Errorf("cannot set manager list of %s: %s", path, err)
	}

	return
}

// delManagers removes list of `users` from the `user.project.managers` file
// attribute of the `path` and its parents.
func (r CephFsRoler) delManagers(path string, users []string) {

	// get managers in the user.project.managers file attribute.
	d, err := xattr.Get(path, fattrManagers)
	if err != nil {
		// use debugf since it is fine that files/sub-directories do not have
		// the `user.project.managers` attribute.
		log.Debugf("cannot get manager list of %s: %s", path, err)
	}

	m := make(map[string]bool)
	for _, u := range strings.Split(string(d), ",") {
		m[u] = true
	}

	// construct a new list of managers to be applied on this path
	for _, u := range users {
		if _, ok := m[u]; ok { // user to be deleted is found in the current manager list
			delete(m, u)
		}
	}
	nm := make([]string, 0, len(m))
	for u := range m {
		nm = append(nm, u)
	}

	// set fattrManagers file attribute with the new list of managers
	if err := xattr.Set(path, fattrManagers, []byte(strings.Join(nm, ","))); err != nil {
		log.Errorf("cannot set manager list of %s: %s", path, err)
	}

	// move to parent directory
	path = filepath.Dir(path)

	// path reaches one of the root directories in which projects
	// are organized.
	if _, ok := RolerMap[path]; ok {
		return
	}

	// path reaches the absolute or relative root.
	if path == "/" || path == "." || path == ".." {
		return
	}

	// delete managers from the parent.
	r.delManagers(path, users)

	return
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

	// user `root` is always a manager.
	if username == "root" {
		return true
	}

	for {

		d, err := xattr.Get(path, fattrManagers)
		//d, err := getfattr(path, fattrManagers)
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

// // getfattr is a wrapper of `getfattr` command to get extended attribute of
// // `key` associated with the given `path`.
// func getfattr(path, key string) ([]byte, error) {

// 	// find size.
// 	size, err := syscall.Getxattr(path, key, nil)
// 	if err != nil {
// 		return nil, err
// 	}
// 	buf := make([]byte, size)
// 	// Read into buffer of that size.
// 	read, err := syscall.Getxattr(path, key, buf)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return buf[:read], nil

// 	// out := ""
// 	// cmd := exec.Command("getfattr", "-n", key, "--only-values", path)

// 	// stdout, err := cmd.Output()
// 	// if err != nil {
// 	// 	return out, err
// 	// }
// 	// return stdout, nil
// }

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

type capHeader struct {
	version uint32
	pid     int
}

type capData struct {
	effective   uint32
	permitted   uint32
	inheritable uint32
}

type caps struct {
	hdr  capHeader
	data [2]capData
}

func getCaps() (caps, error) {
	var c caps

	// Get capability version
	if _, _, errno := syscall.Syscall(syscall.SYS_CAPGET, uintptr(unsafe.Pointer(&c.hdr)), uintptr(unsafe.Pointer(nil)), 0); errno != 0 {
		return c, fmt.Errorf("SYS_CAPGET: %v", errno)
	}

	// Get current capabilities
	if _, _, errno := syscall.Syscall(syscall.SYS_CAPGET, uintptr(unsafe.Pointer(&c.hdr)), uintptr(unsafe.Pointer(&c.data[0])), 0); errno != 0 {
		return c, fmt.Errorf("SYS_CAPGET: %v", errno)
	}

	return c, nil
}

// func mustSupportAmbientCaps() {
// 	var uname syscall.Utsname
// 	if err := syscall.Uname(&uname); err != nil {
// 		log.Fatalf("Uname: %v", err)
// 	}
// 	var buf [65]byte
// 	for i, b := range uname.Release {
// 		buf[i] = byte(b)
// 	}
// 	ver := string(buf[:])
// 	if i := strings.Index(ver, "\x00"); i != -1 {
// 		ver = ver[:i]
// 	}
// 	if strings.HasPrefix(ver, "2.") ||
// 		strings.HasPrefix(ver, "3.") ||
// 		strings.HasPrefix(ver, "4.1.") ||
// 		strings.HasPrefix(ver, "4.2.") {
// 		log.Fatalf("kernel version %q predates required 4.3; skipping test", ver)
// 	}
// }

// setfacl is a wrapper of executing the `setfacl` command on the given `path`
// with the arguments `args`.
//
// It employees the Linux capability `CAP_OWNER` to allow managers of the `path`
// to modify the ACLs.
func setfacl(path string, args []string) error {

	if !isManager(path, "") {
		return fmt.Errorf("permission denied: not a manager")
	}

	// get current user's linux capability
	caps, err := getCaps()
	if err != nil {
		return fmt.Errorf("cannot get capability: %s", err)
	}

	// add CAP_FOWNER capability to the permitted and inheritable capability mask.
	const capFowner = 3
	caps.data[0].permitted |= 1 << uint(capFowner)
	caps.data[0].inheritable |= 1 << uint(capFowner)
	if _, _, errno := syscall.Syscall(syscall.SYS_CAPSET, uintptr(unsafe.Pointer(&caps.hdr)), uintptr(unsafe.Pointer(&caps.data[0])), 0); errno != 0 {
		return fmt.Errorf("cannot set CAP_FOWNER capability: %v", errno)
	}

	// execute the setfacl command
	// try running the setfacl on a file the current user is not the owner
	cmd := exec.Command("setfacl", append(args, path)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		AmbientCaps: []uintptr{capFowner},
	}

	stdout, err := cmd.Output()
	log.Debugf("setfacl stdout: %s", string(stdout))
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			log.Errorf("setfacl stderr: %s", string(ee.Stderr))
		}
	}

	return err
}
