// Package acl provides functions and objects for setting and getting
// data-access permissions on the DCCN project.
package acl

import (
	"os"
	"strings"

	ufp "github.com/dccn-tg/tg-toolset-golang/pkg/filepath"
)

var roleStrings = map[Role]string{
	Manager:     "manager",
	Contributor: "contributor",
	Writer:      "writer",
	Viewer:      "viewer",
	Traverse:    "traverse",
	System:      "system",
}

// Role is a enumeratable integer referring to one of the data-access role:
// Manager, Contributor, Viewer, Traverse, and System
type Role int

// The valid Roles are listed below:
//
// Manager: the role with read/write/management permission
//
// Contributor: the role with read/write permission
//
// Writer: the role with read/write permission but cannot delete files or (sub-)directories
//
// Viewer: the role with read permission
//
// Traverse: the role for passing through the directory
//
// System: the role referring to the Linux system permissions
const (
	Manager Role = iota
	Contributor
	Writer
	Viewer
	Traverse
	System
)

// String returns the human-readable name of the role.
func (r Role) String() string {
	return roleStrings[r]
}

// IsValidRole checks if the given role is a valid one.
func IsValidRole(role Role) bool {
	return role <= System
}

// RoleMap is a map with key as the role, and value as
// a list of usernames in the role.
type RoleMap map[Role][]string

// RolePathMap is a data structure where the RoleMap is associated with a Path.
type RolePathMap struct {
	Path    string
	RoleMap RoleMap
}

// Roler defines interfaces for managing user roles on a filesystem path
// referred by the FilePathMode (in package dccn.nl/utility/filepath).
type Roler interface {
	GetRoles(pinfo ufp.FilePathMode) (RoleMap, error)
	SetRoles(pinfo ufp.FilePathMode, roles RoleMap, recursive bool, followLink bool) (RoleMap, error)
	DelRoles(pinfo ufp.FilePathMode, roles RoleMap, recursive bool, followLink bool) (RoleMap, error)
}

// RolerMap defines a list of supported rolers with associated path as key of
// the map.  The path is usually refers to the top-level mount point of the
// fileserver on which the roler performs actions.
var RolerMap = map[string]Roler{
	"/project":         NetAppRoler{},
	"/groupshare":      NetAppRoler{},
	"/project_freenas": FreeNasRoler{},
	"/project_cephfs":  CephFsRoler{},
}

// GetRoler returns a proper roler determined from the given path.
// It resolves the symbolic link, and determins the roler based on
// the source of the link.
//
// It returns nil if the roler cannot be determined.
//
// GetRoler accounts only the rolers defined in the RolerMap.
func GetRoler(p ufp.FilePathMode) (roler Roler) {
	for b, roler := range RolerMap {
		if strings.HasPrefix(p.Path, b+string(os.PathSeparator)) {
			return roler
		}
	}
	// path falls outside the path and roler defined in the RolerMap.
	return nil
}
