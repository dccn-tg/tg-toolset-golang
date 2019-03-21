package filepath

import (
	"errors"
	"fmt"
	"os"
	"os/user"
)

// AcquireLock creates a lock file at the path specified by the flock argument, and writes a piece of information to the file.
// The information contains: 1) current user id, 2) hostname, and 3) the current process id.
func AcquireLock(flock string) error {
	if _, err := os.Stat(flock); err == nil {
		s := fmt.Sprintf("program locked due to incomplete or on-going run on the same project!\n\nRemove %s and run again!!\n", flock)
		return errors.New(s)
	}

	f, err := os.Create(flock)
	if err != nil {
		return err
	}
	u, _ := user.Current()
	h, _ := os.Hostname()
	s := fmt.Sprintf("%s %s %d", u.Username, h, os.Getpid())
	f.WriteString(s)
	f.Close()
	return nil
}
