package acl

import (
	"fmt"

	ufp "github.com/Donders-Institute/tg-toolset-golang/pkg/filepath"

	pacl "github.com/naegelejd/go-acl"
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

	a, err := pacl.GetFileAccess(path)

	if err != nil {
		return []string{}, err
	}
	defer a.Free()

	for e := a.FirstEntry(); e != nil; e = a.NextEntry() {
		p, _ := e.GetPermset()
		t, _ := e.GetTag()
		q, _ := e.GetQualifier()

		fmt.Printf("p: %v, t: %v, q: %d\n", p, t, q)
	}

	return []string{}, fmt.Errorf("not implemented")
}
