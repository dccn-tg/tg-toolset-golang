package acl

import (
    ufp "dccn.nl/utility/filepath"
)

// NetAppRoler implements Roler interfaces for the NetApp filer.
type NetAppRoler struct {}

// SetRoles implements interface for setting user roles to a given FilePathMode p mounted to
// an endpoint of the NetApp filer.
func (NetAppRoler) SetRoles(pinfo ufp.FilePathMode, roles RoleMap,
    recursive bool, followLink bool) (RoleMap, error) {

    aces_now, err := getACL(pinfo.Path)

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
    var new_aces []ACE
    for _, ace := range aces_now {
        if ! umap[getPrincipleName(ace)] {
            new_aces = append(new_aces, ace)
        }
    }

    // prepend users in the specified role to the ACE list
    for r, users := range roles {
        for _, u := range users {
            ace, _ := newAceFromRole(r, u)
            new_aces = append( []ACE{ *ace }, new_aces... )
        }
    }

    // set the new ACEs to the path
    if err := setACL(pinfo.Path, new_aces, recursive, followLink); err != nil {
        return nil, err
    }

    // return the new RoleMap converted from the new ACEs
    new_roles := make(map[Role][]string)
    for _, ace := range new_aces {
        r := ace.ToRole()
        new_roles[r] = append(new_roles[r], getPrincipleName(ace))
    }

    return new_roles, nil
}

// GetRoles implements interface for getting user roles on a path mounting a NetApp NFSv4 volume.
func (NetAppRoler) GetRoles(pinfo ufp.FilePathMode) (RoleMap,error) {
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

    aces_now, err := getACL(pinfo.Path)

    if err != nil {
        return nil, err
    }

    // remove all users in question in the current ACE list
    var new_aces []ACE
    for _, ace := range aces_now {

        users, ok := roles[ace.ToRole()]
        // the users to be removed not in the same role as this ace
        if ! ok {
            new_aces = append(new_aces, ace)
            continue
        }

        // create map for faster user lookup
        umap := make(map[string]bool)
        for _, u := range users {
            umap[u] = true
        }

        // principle not found on the user list of the role to be removed
        if ! umap[getPrincipleName(ace)] {
            new_aces = append(new_aces, ace)
        }
    }

    // set the new ACEs to the path
    if err := setACL(pinfo.Path, new_aces, recursive, followLink); err != nil {
        return nil, err
    }

    // return the new RoleMap converted from the new ACEs
    new_roles := make(map[Role][]string)
    for _, ace := range new_aces {
        r := ace.ToRole()
        new_roles[r] = append(new_roles[r], getPrincipleName(ace))
    }

    return new_roles, nil
}
