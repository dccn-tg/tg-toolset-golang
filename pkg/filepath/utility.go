package filepath

import (
	"fmt"
	"os"
	"path/filepath"
)

// ResolveAndCheckPath evaulates the given `FileInfo` under a given directory `dir`.
// It resolves to the path's absolute pate (for symbolic links), and checks whether
// the absolute path is existing and accessible to the caller of the function.
func ResolveAndCheckPath(dir string, pinfo os.FileInfo) (*FilePathMode, error) {
	p := filepath.Join(dir, pinfo.Name())

	// resolve symlink
	if pinfo.Mode()&os.ModeSymlink != 0 {
		referent, err := os.Readlink(p)
		if err != nil {
			return nil, fmt.Errorf("cannot resolve referent of symlink: %s, reason: %+v", p, err)
		}
		if []rune(referent)[0] != os.PathSeparator {
			p = filepath.Join(p, referent)
		} else {
			p = referent
		}
	}

	// make the path absolute and clean
	p, _ = filepath.Abs(p)

	// check availability of the path
	stat, err := os.Stat(p)
	if err != nil {
		return nil, fmt.Errorf("project path not found: %s, reason: %+v", p, err)
	}

	fpm := FilePathMode{
		Path: p,
		Mode: stat.Mode(),
	}

	return &fpm, nil
}
