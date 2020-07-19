package repocli

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"sync"

	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
	"github.com/spf13/cobra"
)

// current working directory
var cwd string = "/"
var dataDir string
var recursive bool

func init() {

	if err := initDataDir(); err != nil {
		log.Fatalf("%s", err)
	}

	rmCmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "remove directory recursively")

	rootCmd.AddCommand(lsCmd, putCmd, getCmd, rmCmd, mkdirCmd)
}

// initDataDir determines `dataDir` location and ensures the presence of it.
func initDataDir() error {

	switch runtime.GOOS {
	case "windows":
		// for windows, uses `%APPDATA%\repocli`
		dataDir = filepath.Join(os.Getenv("APPDATA"), "repocli")
	case "darwin", "freebsd", "linux":
		// get current user for retriving the `HomeDir`.
		// TODO: user a better option? https://github.com/mitchellh/go-homedir
		u, err := user.Current()
		if err != nil {
			return fmt.Errorf("cannot determine current user: %s", err)
		}

		// for darwin, freebsd, linux, use `$HOME/.local/share/repocli` as default;
		// and respect to `$XDG_DATA_HOME` variable for systems making use of
		// XDG base directory.
		xdgDataHome := filepath.Join(u.HomeDir, ".local", "share")
		if v := os.Getenv("XDG_DATA_HOME"); v != "" {
			xdgDataHome = v
		}
		dataDir = filepath.Join(xdgDataHome, "repocli")
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	// unsure the presence of `dataDir`
	os.MkdirAll(dataDir, 0644)

	return nil
}

// command to change directory within the repository.
var cdCmd = &cobra.Command{
	Use:   "cd [object|collection]",
	Short: "change directory within the repository",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {

		p := getCleanPath(args[0])

		// stat the path to check if the path is a valid directory
		if f, err := cli.Stat(p); err != nil || !f.IsDir() {
			return fmt.Errorf("invalid directory: %s", p)
		}

		// set cwd to the new path
		cwd = p
		return nil
	},
}

// command to list a file or the content of a directory within the repository.
var lsCmd = &cobra.Command{
	Use:   "ls [object|collection]",
	Short: "list a file or the content of a directory within the repository",
	Long:  ``,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {

		p := cwd
		if len(args) == 1 {
			p = getCleanPath(args[0])
		}

		// check path state
		f, err := cli.Stat(p)
		if err != nil {
			return err
		}

		// path is not a dir, assuming it is just a file, print the info and return
		if !f.IsDir() {
			fmt.Printf("%11s %12d %s\n", f.Mode(), f.Size(), p)
			return nil
		}

		// path is a dir, read the entire content of the dir
		files, err := cli.ReadDir(p)
		if err != nil {
			return nil
		}
		fmt.Printf("%s:\n", p)
		for _, f := range files {
			fmt.Printf("%11s %12d %s\n", f.Mode(), f.Size(), filepath.Join(p, f.Name()))
		}
		return nil
	},
}

var putCmd = &cobra.Command{
	Use:   "put [file|directory] [directory]",
	Short: "upload a local file or a local directory into a directory within the repository",
	Long:  ``,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("not implemented")
	},
}

var getCmd = &cobra.Command{
	Use:   "get [file|directory]",
	Short: "download a file or a directory within the repository into the local current working directory",
	Long:  ``,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("not implemented")
	},
}

var rmCmd = &cobra.Command{
	Use:   "rm [object|collection]",
	Short: "remove a file or a directory from the repository",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {

		p := getCleanPath(args[0])

		f, err := cli.Stat(p)
		if err != nil {
			return err
		}

		// a file
		if !f.IsDir() {
			return cli.Remove(p)
		}

		// a directory.
		return rmRepoDir(p, recursive)
	},
}

var mkdirCmd = &cobra.Command{
	Use:   "mkdir [collection]",
	Short: "create new directory in the repository",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return cli.MkdirAll(args[0], 0664)
	},
}

// getCleanPath resolves provided path into a clean absolute path taking into account
// the `cwd`.
func getCleanPath(path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(cwd, path))
}

// rmRepoDir removes the directory `path` from the repository recursively.
func rmRepoDir(path string, recursive bool) error {

	if !filepath.IsAbs(path) {
		return fmt.Errorf("not an absolute path: %s", path)
	}

	// read the entire content of the dir
	files, err := cli.ReadDir(path)
	if err != nil {
		return err
	}

	if len(files) > 0 && !recursive {
		return fmt.Errorf("directory not empty: %s", path)
	}

	// channel for deleting files concurrently.
	fchan := make(chan string)

	// initalize concurrent workers
	var wg sync.WaitGroup
	nworkers := 4
	// counter for file deletion error by worker
	cntErrFiles := make([]int, nworkers)
	for i := 0; i < nworkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for f := range fchan {
				log.Debugf("removing %s ...", f)
				if err := cli.Remove(f); err != nil {
					log.Errorf("fail removing file: %s", f)
					cntErrFiles[id]++
				}
			}
		}(i)
	}

	// counter for overall deletion errors
	cntErr := 0
	// loop over content
	for _, f := range files {
		p := filepath.Join(path, f.Name())
		if f.IsDir() {
			if err := rmRepoDir(p, recursive); err != nil {
				log.Errorf("fail removing subdir %s: %s", p, err)
				cntErr++
			}
		} else {
			fchan <- p
		}
	}

	// close fchan to release workers
	close(fchan)

	// wait for workers to be released
	wg.Wait()

	// update overall deletion errors with error counts from workers
	for i := 0; i < nworkers; i++ {
		cntErr += cntErrFiles[i]
	}

	if cntErr > 0 {
		return fmt.Errorf("%d files/subdirs not removed", cntErr)
	}

	// remove the directory itself
	log.Debugf("removing %s ...", path)
	return cli.Remove(path)
}
