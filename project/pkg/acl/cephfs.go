package acl

import (
	"bufio"
	"fmt"
	"os/exec"
	"os/user"
	"strings"

	ufp "github.com/Donders-Institute/tg-toolset-golang/pkg/filepath"
	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
)

// CephFsRoler implements roler interface for the CephFS.
type CephFsRoler struct{}

// GetRoles implements interface for getting user roles on a given path mounted to
// an endpoint of the CephFS.
func (CephFsRoler) GetRoles(pinfo ufp.FilePathMode) (RoleMap, error) {
	return nil, fmt.Errorf("not implemented")
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

// PosixGetfacl is a wrapper of `getfacl` command and returns only the
// extended ACEs.
func PosixGetfacl(path string) ([]string, error) {

	out := []string{}

	cmd := exec.Command("getfacl", path)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return out, err
	}

	if err = cmd.Start(); err != nil {
		return out, err
	}

	outScanner := bufio.NewScanner(stdout)
	outScanner.Split(bufio.ScanLines)

	for outScanner.Scan() {
		l := outScanner.Text()
		// skip lines starts with `#` or `default`.
		if strings.HasPrefix(l, "#") || strings.HasPrefix(l, "default") {
			continue
		}
		// skip entry where the qualifier is empty or invalid
		d := strings.Split(l, ":")

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
		default:
			continue
		}
		out = append(out, l)
	}

	if err = outScanner.Err(); err != nil {
		log.Errorf("error reading output of command: %s", err)
	}

	// wait the cmd to finish and the IO pipes are closed.
	// write out error if the command execution is failed.
	if err = cmd.Wait(); err != nil {
		log.Errorf("%s fail: %s", cmd.String(), err)
	}

	return []string{}, nil
}
