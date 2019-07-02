package filepath

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

const (
	blockSize = 4096
	separator = string(filepath.Separator)
)

// See zsyscall_linux_amd64.go/Getdents.
// len(buf)>0.
func getdents(fd int, buf []byte) (n int, err int) {
	var _p0 unsafe.Pointer
	_p0 = unsafe.Pointer(&buf[0])
	r0, _, errno := syscall.Syscall(syscall.SYS_GETDENTS64, uintptr(fd), uintptr(_p0), uintptr(len(buf)))
	n = int(r0)
	err = int(errno)
	return
}

func clen(n []byte) int {
	for i := 0; i < len(n); i++ {
		if n[i] == 0 {
			return i
		}
	}
	return len(n)
}

// fastWalk uses linux specific way (i.e. syscall.SYS_GETDENT64) to walk through
// files and directories under the given root recursively.  Each path it walks through
// is pushed to a given channel of type FilePathMode. The caller is responsible for
// initiating and closing the provided channel.
//
// If mode is provided, both root and mode are respected. Otherwise, the root is stated to
// retrieve its FileMode.  If the root is a symbolic link, the returned FilePathInfo contains
// information and path referring to the referent of the link.
func fastWalk(root string, mode *os.FileMode, followLink bool, chanP *chan FilePathMode) {

	if mode == nil {
		// retrieve FileMode when it is not provided by the caller
		fpm, err := GetFilePathMode(root)
		if err != nil {
			return
		}
		// respect the path returned so that symlink can be followed on the referent's path.
		root = filepath.Clean(fpm.Path)
		*chanP <- *fpm
	} else {
		*chanP <- FilePathMode{Path: root, Mode: *mode}
	}

	dir, err := os.Open(root)
	if err != nil {
		logger.Error(fmt.Sprintf("%s", err))
		return
	}
	defer dir.Close()

	// Opendir.
	// See dir_unix.go/readdirnames.
	buf := make([]byte, blockSize)
	nbuf := len(buf)
	for {
		var errno int
		nbuf, errno = getdents(int(dir.Fd()), buf)
		if errno != 0 || nbuf <= 0 {
			return
		}

		// See syscall_linux.go/ParseDirent.
		subbuf := buf[0:nbuf]
		for len(subbuf) > 0 {
			dirent := (*syscall.Dirent)(unsafe.Pointer(&subbuf[0]))
			subbuf = subbuf[dirent.Reclen:]
			bytes := (*[10000]byte)(unsafe.Pointer(&dirent.Name[0]))

			// Using Reclen we compute the first multiple of 8 above the length of
			// Dirent.Name. This value can be used to compute the length of long
			// Dirent.Name faster by checking the last 8 bytes only.
			minlen := uintptr(dirent.Reclen) - unsafe.Offsetof(dirent.Name)
			if minlen > 8 {
				minlen -= 8
			} else {
				minlen = 0
			}

			var name = string(bytes[0 : minlen+uintptr(clen(bytes[minlen:]))])
			if name == "." || name == ".." { // Useless names
				continue
			}

			vpath := filepath.Join(root, name)

			switch dirent.Type {
			case 0:
				*chanP <- FilePathMode{Path: vpath, Mode: 0}
			case syscall.DT_REG:
				*chanP <- FilePathMode{Path: vpath, Mode: 0}
			case syscall.DT_DIR:
				m := os.ModeDir
				fastWalk(vpath, &m, followLink, chanP)
			case syscall.DT_LNK:

				// TODO: walk through symlinks is not supported due to issue with
				//       eventual infinite walk loop of A -> B -> C -> A cannot be
				//       easily detected.
				// logger.Warnf("skip symlink: %s\n", vpath)
				// continue

				if !followLink {
					logger.Warnf("skip symlink: %s\n", vpath)
					continue
				}

				// follow the link; but only to its first level referent.
				logger.Warnf("symlink only followed to its first non-symlink referent: %s\n", vpath)
				fastWalk(vpath, nil, false, chanP)

				// referent, err := filepath.EvalSymlinks(vpath)
				// if err != nil {
				// 	logger.Errorf("cannot resolve symlink: %s error: %s\n", vpath, err)
				// 	continue
				// }

				// // avoid the situation that the symlink refers to its parent, which
				// // can cause infinite filesystem walk loop.
				// if referent == root {
				// 	logger.Warnf("skip path to avoid symlink loop: %s\n", vpath)
				// 	continue
				// }
			default:
				logger.Warnf("skip unhandled file: %s (type: %s)", vpath, string(dirent.Type))
				continue
			}
		}
	}
}

// GoFastWalk goes through files and directories iteratively within a given root,
// using a go routine.
// It returns a channel in which every visited path is represented with the
// FilePathMode data structure.  The channel can be configured with a given buffer.
// The channel is closed after the last file/directory is visited.
// This GoFastWalk function is based on an example posted by Pierre Neidhardt on
// the following google group discussion:
// https://groups.google.com/forum/#!topic/golang-nuts/PZH2jEAlAOE
//
// Note: This method uses the linux specific way (i.e. syscall.SYS_GETDENT64)
// of getting directory content.  Thus it can only be used with $GOOS=linux.
func GoFastWalk(root string, followLink bool, buffer int) chan FilePathMode {

	chanP := make(chan FilePathMode, buffer)

	go func() {
		fastWalk(root, nil, followLink, &chanP)
		defer close(chanP)
	}()

	return chanP
}
