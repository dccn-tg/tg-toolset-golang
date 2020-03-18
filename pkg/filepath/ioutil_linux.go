package filepath

import (
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

// ListDir lists directory content in a faster way using Linux's system call.
func ListDir(path string) ([]string, error) {

	const (
		blockSize = 4096
		separator = string(filepath.Separator)
	)

	objs := make([]string, 0)

	dir, err := os.Open(path)
	if err != nil {
		return objs, err
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
			return objs, nil
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

			objs = append(objs, filepath.Join(path, name))
		}
	}
}
