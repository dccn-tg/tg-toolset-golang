package acl

import (
	"fmt"
	"strings"

	ufp "github.com/dccn-tg/tg-toolset-golang/pkg/filepath"
	log "github.com/dccn-tg/tg-toolset-golang/pkg/logger"
)

// FreeNasRoler implements Roler interfaces for the FreeNAS filer.
type FreeNasRoler struct{}

// SetRoles implements interface for setting user roles to a given path mounted to
// an endpoint of the FreeNAS filer.
func (FreeNasRoler) SetRoles(pinfo ufp.FilePathMode, roles RoleMap,
	recursive bool, followLink bool) (RoleMap, error) {

	acesNow, err := getACL(pinfo.Path)

	if err != nil {
		return nil, err
	}

	// create map for faster user lookup
	umap := make(map[string]bool)
	for _, users := range roles {
		for _, u := range users {
			umap[u] = true
		}
	}

	// remove all users in question in the current ACE list
	var acesNew []ACE
	for _, ace := range acesNow {
		if !umap[getPrincipleName(ace)] {
			acesNew = append(acesNew, ace)
		}
	}

	// prepend users in the specified role to the ACE list
	for r, users := range roles {
		for _, u := range users {
			acesNew = append(newAcesFromRole(r, u, pinfo), acesNew...)
		}
	}

	// set the new ACEs to the path
	if err := setACL(pinfo.Path, acesNew, recursive, followLink); err != nil {
		return nil, err
	}

	// return the new RoleMap converted from the new ACEs
	rolesNew := make(map[Role][]string)
	for _, ace := range acesNew {
		r := ace.ToRole()
		rolesNew[r] = append(rolesNew[r], getPrincipleName(ace))
	}

	return rolesNew, nil
}

// GetRoles implements interface for getting user roles on a given path mounted to
// an endpoint of the FreeNAS filer.
func (FreeNasRoler) GetRoles(pinfo ufp.FilePathMode) (RoleMap, error) {
	roles := make(map[Role][]string)
	aces, err := getACL(pinfo.Path)
	if err != nil {
		return nil, err
	}
	for _, ace := range aces {

		if isDenyAceForDeletion(ace) { // ignore specific DENY
			log.Debugf("ignore deny type ace: %s\n", ace.String())
			continue
		}

		r := ace.ToRole()
		// exclude the same user appearing twice: one for file and one for directory
		uname := getPrincipleName(ace)
		if strings.Contains(strings.Join(roles[r], ","), uname) {
			continue
		}
		roles[r] = append(roles[r], getPrincipleName(ace))
	}
	return roles, nil
}

// DelRoles implements interface for removing users from the specified roles on a path
// mounting a FreeNAS NFSv4 volume.
func (FreeNasRoler) DelRoles(pinfo ufp.FilePathMode, roles RoleMap,
	recursive bool, followLink bool) (RoleMap, error) {

	acesNow, err := getACL(pinfo.Path)

	if err != nil {
		return nil, err
	}

	// remove all users in question in the current ACE list
	var acesNew []ACE
	for _, ace := range acesNow {

		var r Role
		if isDenyAceForDeletion(ace) {
			r = Writer
		} else {
			r = ace.ToRole()
		}

		users, ok := roles[r]
		// the users to be removed not in the same role as this ace
		if !ok {
			acesNew = append(acesNew, ace)
			continue
		}

		// create map for faster user lookup
		umap := make(map[string]bool)
		for _, u := range users {
			umap[u] = true
		}

		// principle not found on the user list of the role to be removed
		if !umap[getPrincipleName(ace)] {
			acesNew = append(acesNew, ace)
		}
	}

	// set the new ACEs to the path
	if err := setACL(pinfo.Path, acesNew, recursive, followLink); err != nil {
		return nil, err
	}

	// return the new RoleMap converted from the new ACEs
	rolesNew := make(map[Role][]string)
	for _, ace := range acesNew {
		r := ace.ToRole()
		rolesNew[r] = append(rolesNew[r], getPrincipleName(ace))
	}

	return rolesNew, nil
}

// isDenyAceForDeletion checks if the given ACE is the DENY ace specific for the
// role that allows file writing and (sub-)directory creation; but not deletion.
func isDenyAceForDeletion(ace ACE) bool {
	return ace.IsDeny() && (ace.Mask == "dD" || ace.Mask == "Dd" || ace.Mask == "d")
}

// newAcesFromRole constructs two ACEs from the given role for directory and file.
func newAcesFromRole(role Role, userOrGroupName string, p ufp.FilePathMode) []ACE {
	group := false
	if strings.Index(userOrGroupName, "g:") == 0 {
		group = true
		userOrGroupName = strings.TrimLeft(userOrGroupName, "g:")
	}

	flagD := strings.Replace(aceFlag[group], "f", "", 1)
	flagF := strings.Replace(aceFlag[group], "d", "", 1)
	maskNx := strings.Replace(aceMask[role], "x", "", 1)

	// regular ACEs with ACCEPT type
	var aces []ACE

	// special ACEs with DENY type
	var acesDeny []ACE

	switch {
	case p.Mode.IsDir(): // in case of a directory
		aces = []ACE{
			{
				Type:      "A",
				Flag:      flagD,
				Principle: fmt.Sprintf("%s@%s", userOrGroupName, userDomain),
				Mask:      aceMask[role],
			},
			{
				Type:      "A",
				Flag:      flagF,
				Principle: fmt.Sprintf("%s@%s", userOrGroupName, userDomain),
				Mask:      maskNx,
			},
		}

		// explicit DENY ACEs for user allow writing but not deleting files (e.g. Writer role)
		acesDeny = []ACE{
			{
				Type:      "D",
				Flag:      flagD,
				Principle: fmt.Sprintf("%s@%s", userOrGroupName, userDomain),
				Mask:      "dD",
			},
			{
				Type:      "D",
				Flag:      flagF,
				Principle: fmt.Sprintf("%s@%s", userOrGroupName, userDomain),
				Mask:      "d",
			},
		}
	default: // in case of not a directory (assuming it's a file)
		aces = []ACE{
			{
				Type:      "A",
				Flag:      strings.Replace(flagD, "d", "", 1),
				Principle: fmt.Sprintf("%s@%s", userOrGroupName, userDomain),
				Mask:      aceMask[role],
			},
		}
		acesDeny = []ACE{
			{
				Type:      "D",
				Flag:      strings.Replace(flagD, "d", "", 1),
				Principle: fmt.Sprintf("%s@%s", userOrGroupName, userDomain),
				Mask:      "d",
			},
		}
	}

	if role == Writer {
		return append(acesDeny, aces...)
	}
	return aces
}
