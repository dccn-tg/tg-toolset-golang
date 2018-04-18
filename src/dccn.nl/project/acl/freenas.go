package acl

import (
    "fmt"
    "strings"
    ufp "dccn.nl/utility/filepath"
)

// FreeNasRoler implements Roler interfaces for the FreeNAS filer.
type FreeNasRoler struct {}

// SetRoles implements interface for setting user roles to a given path mounted to
// an endpoint of the FreeNAS filer.
func (FreeNasRoler) SetRoles(pinfo ufp.FilePathMode, roles RoleMap,
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
            new_aces = append( newAcesFromRole(r, u, pinfo), new_aces... )
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

// GetRoles implements interface for getting user roles on a given path mounted to
// an endpoint of the FreeNAS filer.
func (FreeNasRoler) GetRoles(pinfo ufp.FilePathMode) (RoleMap,error) {
    roles := make(map[Role][]string)
    aces, err := getACL(pinfo.Path)
    if err != nil {
        return nil, err
    }
    for _, ace := range aces {
        r := ace.ToRole()
        // exclude the same user appearing twice: one for file and one for directory
        uname := getPrincipleName(ace)
        if strings.Index( strings.Join(roles[r], ","), uname ) >= 0 {
            continue
        }
        roles[r] = append(roles[r], getPrincipleName(ace))
    }
    return roles, nil
}

// DelRoles implements interface for removing users from the specified roles on a path
// mounting a FreeNAS NFSv4 volume.
//
func (FreeNasRoler) DelRoles(pinfo ufp.FilePathMode, roles RoleMap,
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

// newAcesFromRole constructs two ACEs from the given role for directory and file.
func newAcesFromRole(role Role, userOrGroupName string, p ufp.FilePathMode) ([]ACE) {
    group := false
    if strings.Index(userOrGroupName, "g:") == 0 {
        group = true
        userOrGroupName = strings.TrimLeft(userOrGroupName, "g:")
    }

    flag_d  := strings.Replace(aceFlag[group], "f", "", 1)
    flag_f  := strings.Replace(aceFlag[group], "d", "", 1)
    mask_nx := strings.Replace(aceMask[role], "x", "", 1)

    if p.Mode.IsDir() {
        return []ACE {
            ACE{
                Type: "A",
                Flag: flag_d,
                Principle: fmt.Sprintf("%s@%s", userOrGroupName, userDomain),
                Mask: aceMask[role],
            },
            ACE {
                Type: "A",
                Flag: flag_f,
                Principle: fmt.Sprintf("%s@%s", userOrGroupName, userDomain),
                Mask: mask_nx,
            },
        }
    }
    return []ACE {
        ACE{
            Type: "A",
            Flag: strings.Replace(flag_d, "d", "", 1),
            Principle: fmt.Sprintf("%s@%s", userOrGroupName, userDomain),
            Mask: aceMask[role],
        },
    }
}
