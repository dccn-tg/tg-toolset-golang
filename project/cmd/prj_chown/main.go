// This program uses the linux capability CAP_CHOWN for project manager to change
// the owner of a file or directory.
//
// In order to allow this program to work, this executable should be set in
// advance to allow using the linux capability using the following command.
//
// ```
// $ sudo setcap cap_chown+eip prj_chown
// ```
package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/acl"
)

func init() {

	cfg := log.Configuration{
		EnableConsole:     true,
		ConsoleJSONFormat: false,
		ConsoleLevel:      log.Info,
	}

	// initialize logger
	log.NewLogger(cfg, log.InstanceLogrusLogger)
}

func usage() {
	fmt.Printf("\nChanging owner of files or directories.\n")
	fmt.Printf("\nUSAGE: %s [OPTIONS] path...\n", os.Args[0])
}

func main() {

	// command-line arguments
	args := os.Args[1:]

	chownArgs := []string{}
	paths := []string{}

	for i, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			_, err := os.Stat(arg)
			if os.IsNotExist(err) || args[i-1] == "--reference" {
				chownArgs = append(chownArgs, arg)
			} else {
				paths = append(paths, arg)
			}
			continue
		}

		chownArgs = append(chownArgs, arg)
	}

	log.Infof("chownArgs: %+v", chownArgs)

	for _, p := range paths {
		chown(p, chownArgs)
	}
}

// isManager determines whether the given user is a manager of the path, using
// the `acl.Runner`.
func isManager(path, username string) bool {

	ppath, _ := filepath.Abs(path)

	runner := acl.Runner{
		RootPath:   ppath,
		FollowLink: true,
		SkipFiles:  false,
		Nthreads:   1,
	}

	chanOut, err := runner.GetRoles(false)
	if err != nil {
		log.Errorf("cannot get user role on path %s: %s", path, err)
		return false
	}

	for o := range chanOut {
		for _, u := range o.RoleMap[acl.Manager] {
			if u == username {
				return true
			}
		}
	}

	return false
}

// chown makes a system call to run `chown` command given the path and arguments.
//
// If the caller of it is a manager of the path, the system call is made with
// the `CAP_CHOWN` linux capability so that it can overcome the files are not
// ownered by the caller.
//
func chown(path string, args []string) error {

	// current program caller
	caller, err := user.Current()
	if err != nil {
		return err
	}

	// chown command
	cmd := exec.Command("chown", append(args, path)...)

	if isManager(path, caller.Username) {
		// get current user's linux capability
		caps, err := getCaps()
		if err != nil {
			return fmt.Errorf("cannot get capability: %s", err)
		}

		// add CAP_CHOWN capability to the permitted and inheritable capability mask.
		const capChown = 0
		caps.data[0].permitted |= 1 << uint(capChown)
		caps.data[0].inheritable |= 1 << uint(capChown)
		if _, _, errno := syscall.Syscall(syscall.SYS_CAPSET, uintptr(unsafe.Pointer(&caps.hdr)), uintptr(unsafe.Pointer(&caps.data[0])), 0); errno != 0 {
			return fmt.Errorf("cannot set CAP_FOWNER capability: %v", errno)
		}

		// use CAP_CHOWN capability for syscall.
		cmd.SysProcAttr = &syscall.SysProcAttr{
			AmbientCaps: []uintptr{capChown},
		}
	}

	stdout, err := cmd.Output()
	log.Debugf("chown stdout: %s", string(stdout))
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			log.Errorf("chown stderr: %s", string(ee.Stderr))
		}
	}

	return err
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
