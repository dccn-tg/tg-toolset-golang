package acl

import (
	"fmt"
	"os/exec"
	"os/user"
	"strings"

	log "github.com/dccn-tg/tg-toolset-golang/pkg/logger"
	ustr "github.com/dccn-tg/tg-toolset-golang/pkg/strings"
	"github.com/pkg/errors"
)

// aceAlias defines the ace mask alias supported by the NFSv4 ACL.
var aceAlias = map[string]string{
	"R": "rntcy",
	"W": "watTNcCy",
	"X": "xtcy",
}

// aceFlag defines the ace flags supported by the NFSv4 ACL.
var aceFlag = map[bool]string{
	true:  "fdg",
	false: "fd",
}

// aceMask maps the roles into the NFSv4 ACE masks.
var aceMask = map[Role]string{
	Manager:     aceAlias["R"] + aceAlias["W"] + aceAlias["X"] + "dDoy",
	Contributor: "rwaDdxnNtTcy",
	Writer:      "rwaxnNtTcy",
	Viewer:      aceAlias["R"] + aceAlias["X"] + "y",
	Traverse:    "x",
}

// aceSysPrinciple defines a set of ACE principles referring to the system permissions.
var aceSysPrinciple = map[string]bool{
	"OWNER@":    true,
	"GROUP@":    true,
	"EVERYONE@": true,
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
func (ace ACE) String() string {
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

// IsDeny checks if the ACE is a deny-type ACE.
func (ace *ACE) IsDeny() bool {
	return ace.Type == "D"
}

// ForceInheritance modifies the `Flag` to ensure the `f` and `d` flags
// are added.
func (ace *ACE) ForceInheritance() {
	flag := strings.ReplaceAll(ace.Flag, "f", "")
	flag = strings.ReplaceAll(flag, "d", "")
	ace.Flag = fmt.Sprintf("fd%s", flag)
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

	if strings.Contains(ace.Flag, "g") {
		// look up group
		_, err := user.LookupGroup(uname)
		return err == nil
	}

	// look up user
	_, err := user.Lookup(uname)
	return err == nil
}

// ToRole resolves the name of the access role referred by the ACE.
// If the ACE refers to a Linux system permission, it returns "system".
func (ace ACE) ToRole() Role {
	// return empty role for system principles
	if ace.IsSysPermission() {
		return System
	}

	var role Role
	lm := 99
	for r, m := range aceMask {

		// Extend the ace mask of the `Writer` with extra `+` to avoid
		// ambiguity in resolving file's ACE into `Contributor` and `Writer`
		if r == Writer {
			m = fmt.Sprintf("%s+", m)
		}

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
	cmdNfs4Getfacl := "nfs4_getfacl"
	out, err := exec.Command(cmdNfs4Getfacl, path).Output()
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("%s exec failure", cmdNfs4Getfacl))
	}
	var aces []ACE
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			// an empty line or a line stars with "#" is ok to ignore
			continue
		}
		ace, err := parseAce(line)
		if err != nil {
			// print a warning and continue
			log.Warnf("%s", err)
			continue
		}
		aces = append(aces, *ace)
	}
	return aces, nil
}

// setACL sets a list of ACEs to the given path by calling the "nfs4_setfacl" command.
// It assumes that the "nfs4_setacl" is available in the $PATH environment.
func setACL(path string, aces []ACE, recursive bool, followLink bool) error {
	cmdNfs4Setfacl := "nfs4_setfacl"

	var naces []string // domain user ACEs
	var acess []string // system default ACEs
	var cmdArgs []string

	// extract valid ACEs
	for _, ace := range aces {

		// ignore System principles
		if ace.IsSysPermission() {
			acess = append(acess, ace.String())
			continue
		}

		// ignore invalid principles
		if ace.IsValidPrinciple() {
			naces = append(naces, ace.String())
		} else {
			log.Warnf("invalid user or group: %s %s", ace.Principle, path)
		}
	}

	// put system default ACEs at the end of the list
	naces = append(naces, acess...)

	// create the full command-line arguments for nfs4_setfacl
	if recursive {
		cmdArgs = append(cmdArgs, "-R")
	}
	if followLink {
		cmdArgs = append(cmdArgs, "-L")
	}
	cmdArgs = append(cmdArgs, "-s")
	cmdArgs = append(cmdArgs, strings.Join(naces[:], ","))
	cmdArgs = append(cmdArgs, path)

	log.Debugf("%s %s", cmdNfs4Setfacl, strings.Join(cmdArgs, " "))

	_, err := exec.Command(cmdNfs4Setfacl, cmdArgs...).Output()
	return errors.Wrap(err, fmt.Sprintf("%s exec failure", cmdNfs4Setfacl))
}

// getPrincipleName transforms the ACE's Principle into the valid system user or group name.
func getPrincipleName(ace ACE) string {
	if strings.Contains(ace.Flag, "g") {
		return "g:" + strings.TrimSuffix(ace.Principle, "@"+userDomain)
	}
	return strings.TrimSuffix(ace.Principle, "@"+userDomain)
}

// parseAce parses Nfs4 ace string into the ACE structure.
func parseAce(ace string) (*ACE, error) {
	d := strings.Split(ace, ":")
	if len(d) != 4 {
		return nil, errors.New("invalid ACE string: " + ace)
	}
	return &ACE{
		Type:      d[0],
		Flag:      d[1],
		Principle: d[2],
		Mask:      d[3],
	}, nil
}

// newAceFromRole constructs a ACE from the given role and the system user (or group) name.
func newAceFromRole(role Role, userOrGroupName string) (*ACE, error) {
	group := false
	if strings.Index(userOrGroupName, "g:") == 0 {
		group = true
		userOrGroupName = strings.TrimLeft(userOrGroupName, "g:")
	}

	return &ACE{
		Type:      "A",
		Flag:      aceFlag[group],
		Principle: fmt.Sprintf("%s@%s", userOrGroupName, userDomain),
		Mask:      aceMask[role],
	}, nil
}
