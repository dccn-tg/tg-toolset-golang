package acl

import (
    "fmt"
    "os/user"
    "os/exec"
    "strings"
    "github.com/pkg/errors"
    log  "github.com/sirupsen/logrus"
    ustr "dccn.nl/utility/strings"
)

var logger *log.Entry

func init() {
    logger = log.WithFields(log.Fields{"source":"acl.nfs4"})
}

// aceAlias defines the ace mask alias supported by the NFSv4 ACL.
var aceAlias = map[string]string{
    "R": "rntcy",
    "W": "watTNcCy",
    "X": "xtcy",
}

// aceFlag defines the ace flags supported by the NFSv4 ACL.
var aceFlag = map[bool]string{
    true : "fdg",
    false: "fd",
}

// aceMask maps the roles into the NFSv4 ACE masks.
var aceMask = map[Role]string{
    Manager      : aceAlias["R"] + aceAlias["W"] + aceAlias["X"] + "dDoy",
    Contributor  : "rwaDdxnNtTcy",
    Viewer       : aceAlias["R"] + aceAlias["X"] + "y",
    Traverse     : "x",
}

// aceSysPrinciple defines a set of ACE principles referring to the system permissions.
var aceSysPrinciple = map[string]bool{
    "OWNER@"     : true,
    "GROUP@"     : true,
    "EVERYONE@"  : true,
}

// userDomain is the domain name of the DCCN Active Directory.
// All DCCN domain users/groups specified in ACE should have it as suffix
// of the corresponding ACE Principles.
var userDomain = "dccn.nl"

// ACE holds the attributes of a NFSv4 Access-Control Entry (ACE)
type ACE struct {
    Type      string
    Flag      string
    Principle string
    Mask      string
}

// String implements the string formation of the ACE.
// It reconstructs the format from the command "nfs4_getfacl"
func (ace ACE) String() (string) {
    return ace.Type + ":" +
           ace.Flag + ":" +
           ace.Principle + ":" +
           ace.Mask
}

// IsSysPermission checks if the ACE refers to a Linux permission for
// owner, group or others.
func (ace *ACE) IsSysPermission() bool {
    return aceSysPrinciple[ace.Principle]
}

// IsValidPrinciple checks if the ACE's Principle pertains to a valid system user or group.
// If the ACE's Flag contains "g", the Principle is considered as a group; otherwise, the user.
// If the ACE refers to a Linux system permission,  it returns true.
func (ace *ACE) IsValidPrinciple() bool {
    uname := strings.Split(ace.Principle, "@")[0]

    // system priciple
    if ace.IsSysPermission() {
        return true
    }

    if strings.Index(ace.Flag, "g") >= 0  {
        // look up group
        _, err := user.LookupGroup(uname)
        return err == nil
    } else {
        // look up user
        _, err := user.Lookup(uname)
        return err == nil
    }
}

// ToRole resolves the name of the access role referred by the ACE.
// If the ACE refers to a Linux system permission, it returns "system".
func (ace ACE) ToRole() Role {
    // return empty role for system principles
    if ace.IsSysPermission() {
        return System
    }

    var role Role
    var lm   int = 99
    for r,m := range aceMask {
        if _lm := len(ustr.StringXOR(ace.Mask, m)); _lm < lm {
            lm = _lm
            role = r
        }
    }
    return role
}

// getACL gets the ACL of the given path as a ACE list by calling the "nfs4_getacl" command.
// It assumes that the "nfs4_getacl" is available in the $PATH environment.
func getACL(path string) ([]ACE, error) {
    cmd_nfs4_getfacl := "nfs4_getfacl"
    out,err := exec.Command(cmd_nfs4_getfacl, path).Output()
    if err != nil {
        return nil, errors.Wrap(err, fmt.Sprintf("%s exec failure", cmd_nfs4_getfacl))
    }
    var aces []ACE
    for _, line := range strings.Split(string(out), "\n") {
        ace, err := parseAce(line)
        if err != nil {
            // an empty line is ok to ignore
            if line == "" {
                continue
            }
            logger.Warn(fmt.Sprintf("%s", err))
        }
        aces = append(aces, *ace)
    }
    return aces, nil
}

// setACL sets a list of ACEs to the given path by calling the "nfs4_setfacl" command.
// It assumes that the "nfs4_setacl" is available in the $PATH environment.
func setACL(path string, aces []ACE, recursive bool, followLink bool) error {
    cmd_nfs4_setfacl := "nfs4_setfacl"

    var naces []string     // domain user ACEs
    var acess []string    // system default ACEs
    var cmd_args []string

    // extract valid ACEs
    for _, ace := range aces {

        // ignore System principles
        if ace.IsSysPermission() {
            acess = append(acess, fmt.Sprintf("%s", ace))
            continue
        }

        // ignore invalid principles
        if ace.IsValidPrinciple() {
            naces = append(naces, fmt.Sprintf("%s", ace))
        } else {
            logger.Warn(fmt.Sprintf("invalid user or group: %s %s", ace.Principle, path))
        }
    }

    // put system default ACEs at the end of the list
    naces = append(naces, acess...)

    // create the full command-line arguments for nfs4_setfacl
    if recursive {
        cmd_args = append(cmd_args, "-R")
    }
    if followLink {
        cmd_args = append(cmd_args, "-L")
    }
    cmd_args = append(cmd_args, "-s")
    cmd_args = append(cmd_args, strings.Join(naces[:], ","))
    cmd_args = append(cmd_args, path)

    logger.Debug(fmt.Sprintf("%s %s", cmd_nfs4_setfacl, strings.Join(cmd_args," ")))

    _,err := exec.Command(cmd_nfs4_setfacl, cmd_args...).Output()
    return errors.Wrap(err, fmt.Sprintf("%s exec failure", cmd_nfs4_setfacl))
}

// getPrincipleName transforms the ACE's Principle into the valid system user or group name.
func getPrincipleName(ace ACE) (string) {
    if strings.Index(ace.Flag, "g") >= 0 {
        return "g:" + strings.TrimRight(ace.Principle, "@" + userDomain)
    } else {
        return strings.TrimRight(ace.Principle, "@" + userDomain)
    }
}

// parseAce parses Nfs4 ace string into the ACE structure.
func parseAce(ace string) (*ACE, error) {
    d := strings.Split(ace, ":")
    if len(d) != 4 {
        return nil, errors.New("invalid ACE string: " + ace)
    } else {
        return &ACE{
            Type     : d[0],
            Flag     : d[1],
            Principle: d[2],
            Mask     : d[3],
        }, nil
    }
}

// newAceFromRole constructs a ACE from the given role and the system user (or group) name.
func newAceFromRole(role Role, userOrGroupName string) (*ACE,error) {
    group := false
    if strings.Index(userOrGroupName, "g:") == 0 {
        group = true
        userOrGroupName = strings.TrimLeft(userOrGroupName, "g:")
    }

    return &ACE{
        Type: "A",
        Flag: aceFlag[group],
        Principle: fmt.Sprintf("%s@%s", userOrGroupName, userDomain),
        Mask: aceMask[role],
    }, nil
}
