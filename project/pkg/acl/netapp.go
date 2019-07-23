package acl

import (
	ufp "github.com/Donders-Institute/tg-toolset-golang/pkg/filepath"
)

// NetAppRoler implements Roler interfaces for the NetApp filer.
type NetAppRoler struct{}

// SetRoles implements interface for setting user roles to a given FilePathMode p mounted to
// an endpoint of the NetApp filer.
func (NetAppRoler) SetRoles(pinfo ufp.FilePathMode, roles RoleMap,
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
			// enforce inheritance on ACEs of the system principle.
			// this is so far only needed for NetApp to be Windows friendly.
			if ace.IsSysPermission() && pinfo.Mode.IsDir() {
				ace.ForceInheritance()
			}
			acesNew = append(acesNew, ace)
		}
	}

	// prepend users in the specified role to the ACE list
	for r, users := range roles {
		for _, u := range users {
			ace, _ := newAceFromRole(r, u)
			acesNew = append([]ACE{*ace}, acesNew...)
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

// GetRoles implements interface for getting user roles on a path mounting a NetApp NFSv4 volume.
func (NetAppRoler) GetRoles(pinfo ufp.FilePathMode) (RoleMap, error) {
	roles := make(map[Role][]string)
	aces, err := getACL(pinfo.Path)
	if err != nil {
		return nil, err
	}
	for _, ace := range aces {
		r := ace.ToRole()
		roles[r] = append(roles[r], getPrincipleName(ace))
	}
	return roles, nil
}

// DelRoles implements interface for removing users from the specified roles on a path
// mounting a NetApp NFSv4 volume.
//
func (NetAppRoler) DelRoles(pinfo ufp.FilePathMode, roles RoleMap,
	recursive bool, followLink bool) (RoleMap, error) {

	acesNow, err := getACL(pinfo.Path)

	if err != nil {
		return nil, err
	}

	// remove all users in question in the current ACE list
	var acesNew []ACE
	for _, ace := range acesNow {

		users, ok := roles[ace.ToRole()]
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
