// Package filepath provides utility functions and data structure for working
// on filesystem path and directories.
package filepath

import (
	"fmt"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
)

var logger *log.Entry

func init() {
	logger = log.WithFields(log.Fields{"source": "utility.filepath"})
}

// makeFilePathMode is an internal function constructing the FilePathMode object
// from the given path and its os.FileMode.
// It also resolves the information to the target path if the given path is a
// symbolic link.
func makeFilePathMode(root string, fm os.FileMode) (*FilePathMode, error) {

	// get the target's FileInfo for the symbolic link
	if fm&os.ModeSymlink != 0 {
		referent, err := os.Readlink(root)
		if err != nil {
			return nil, err
		}
		if []rune(referent)[0] != os.PathSeparator {
			referent = filepath.Join(root, referent)
		}
		// replace path and fi with the referent's path and FileInfo
		root = referent
		fi, err := os.Stat(root)
		if err != nil {
			return nil, err
		}
		fm = fi.Mode()
	}

	// prepend os.PathSeparator to directory so that it can deal with automount
	// or submount of a remote filesystem.
	if fm.IsDir() {
		root = fmt.Sprintf("%s%c", root, os.PathSeparator)
	}

	return &FilePathMode{Path: root, Mode: fm}, nil
}

// FileFilter is a function passed to the GoPathWalk function for selecting the
// files and directories interested by the caller of the GoPathWalk function.
// When the boolean value "true" is returned, the corresponding file/directory
// is selected; otherwise it is ignored.
type FileFilter func(os.FileInfo) bool

// FilePathMode is a data structure containing
// - Path: the filesystem path
// - Mode: the os.FileMode of the Path
type FilePathMode struct {
	Path string
	Mode os.FileMode
}

// GetFilePathMode constructs the FilePathMode data structure from the given path.
// If the given path is a symbolic link, this function follows the link and returns
// the FilePathMode structure of the referent of the symbolic link.
func GetFilePathMode(path string) (*FilePathMode, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}

	// get FileInfo and referent path of a symbolic link
	if fi.Mode()&os.ModeSymlink != 0 {
		dirname := filepath.Dir(path)
		referent, _ := os.Readlink(path)
		if []rune(referent)[0] != os.PathSeparator {
			path = filepath.Join(dirname, referent)
		} else {
			path = referent
		}
		fi, err = os.Stat(path)
	}

	m := fi.Mode()

	// append ending "/" for path
	if m.IsDir() {
		path += "/"
	}

	return &FilePathMode{Path: path, Mode: m}, nil
}

// GoWalk loops through files and directories under the given root recursively using a
// go routine.
// It applies the FileFilter provided by the caller to select certain files and directories
// the caller is interested in.  The selected files and directories are represented
// in the FilePathMode structure and pushed to the returned channel with a specified
// buffer size.  The channel is closed after the walk visited the last file/directory.
func GoWalk(root string, filter FileFilter, buffer int) chan FilePathMode {
	chan_f := make(chan FilePathMode, buffer)
	go func() {
		filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
			if filter(fi) {
				// convert path p into FilePathInfo
				if fpm, err := makeFilePathMode(p, fi.Mode()); err == nil {
					chan_f <- *fpm
				}
			}
			return nil
		})
		defer close(chan_f)
	}()
	return chan_f
}
