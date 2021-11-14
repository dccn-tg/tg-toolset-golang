package repocli

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"unicode/utf8"

	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
	"github.com/spf13/cobra"

	pb "github.com/schollz/progressbar/v3"
)

// pathFileInfo is an internal data structure containing the absolute `path`
// and the `fs.FileInfo` of a given file.
type pathFileInfo struct {
	path string
	info fs.FileInfo
}

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

	// ensure the presence of `dataDir`
	os.MkdirAll(dataDir, 0644)

	return nil
}

// command to change directory in the repository.
// At the moment, this command makes no sense as the current directory is not persistent.
var cdCmd = &cobra.Command{
	Use:   "cd [directory]]",
	Short: "change directory in the repository",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {

		p := getCleanRepoPath(args[0])

		// stat the path to check if the path is a valid directory
		if f, err := cli.Stat(p); err != nil || !f.IsDir() {
			return fmt.Errorf("invalid directory: %s", p)
		}

		// set cwd to the new path
		cwd = p
		return nil
	},
}

// command to list a file or the content of a directory in the repository.
var lsCmd = &cobra.Command{
	Use:   "ls [file|directory]",
	Short: "list a file or the content of a directory in the repository",
	Long:  ``,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {

		p := cwd
		if len(args) == 1 {
			p = getCleanRepoPath(args[0])
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
			fmt.Printf("%11s %12d %s\n", f.Mode(), f.Size(), path.Join(p, f.Name()))
		}
		return nil
	},
}

var putCmd = &cobra.Command{
	Use:   "put [file|directory] [directory]",
	Short: "upload a local file or a local directory into a repository directory",
	Long:  ``,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {

		// resolve into absolute path at local
		lfpath, err := filepath.Abs(args[0])
		if err != nil {
			return nil
		}

		// get local path's FileInfo
		lfinfo, err := os.Stat(lfpath)
		if err != nil {
			return err
		}
		if lfinfo.IsDir() {
			return fmt.Errorf("put directory not implemented")
		}

		// construct filepath at repository
		pfinfoRepo := pathFileInfo{
			path: path.Join(getCleanRepoPath(args[1]), lfinfo.Name()),
		}

		// a file
		pfinfoLocal := pathFileInfo{
			path: lfpath,
			info: lfinfo,
		}
		return putRepoFile(pfinfoLocal, pfinfoRepo, true)
	},
}

var getCmd = &cobra.Command{
	Use:   "get [file|directory]",
	Short: "download a file or a directory within the repository into the current directory at local",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {

		p := getCleanRepoPath(args[0])

		f, err := cli.Stat(p)
		if err != nil {
			return err
		}

		// repo pathInfo
		pfinfoRepo := pathFileInfo{
			path: p,
			info: f,
		}

		// local pathInfo
		lcwd, _ := os.Getwd()
		pfinfoLocal := pathFileInfo{
			path: filepath.Join(lcwd, filepath.Base(args[0])),
		}

		// download recursively
		if f.IsDir() {
			return getRepoDir(pfinfoRepo, pfinfoLocal, true)
		}

		// download single file
		return getRepoFile(pfinfoRepo, pfinfoLocal, true)
	},
}

var rmCmd = &cobra.Command{
	Use:   "rm [file|directory]",
	Short: "remove a file or a directory from the repository",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {

		p := getCleanRepoPath(args[0])

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
	Use:   "mkdir [directory]",
	Short: "create new directory in the repository",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return cli.MkdirAll(getCleanRepoPath(args[0]), 0664)
	},
}

// getCleanRepoPath resolves provided path into a clean absolute path taking into account
// the `cwd`.
func getCleanRepoPath(p string) string {
	if strings.HasPrefix(p, "/") {
		return path.Clean(p)
	}
	return path.Join(cwd, p)
}

// putRepoFile uploads a single local file to the repository.
func putRepoFile(pfinfoLocal, pfinfoRepo pathFileInfo, showProgress bool) error {

	// open pathLocal
	reader, err := os.Open(pfinfoLocal.path)
	if err != nil {
		return fmt.Errorf("cannot open local file: %s", err)
	}
	defer reader.Close()

	// progress bar
	barDesc := prettifyProgressbarDesc(pfinfoLocal.info.Name())
	bar := pb.DefaultBytesSilent(pfinfoLocal.info.Size(), barDesc)
	if showProgress {
		bar = pb.DefaultBytes(pfinfoLocal.info.Size(), barDesc)
	}

	// read pathRepo and write to pathLocal, the mode is not actually useful (!?)
	err = cli.WriteStream(pfinfoRepo.path, reader, pfinfoLocal.info.Mode())
	if err != nil {
		return fmt.Errorf("cannot write %s to the repository: %s", pfinfoRepo.path, err)
	}

	// file size check after upload
	f, err := cli.Stat(pfinfoRepo.path)
	if err != nil {
		return fmt.Errorf("cannot stat %s at the repository: %s", pfinfoRepo.path, err)
	}

	if f.Size() != pfinfoLocal.info.Size() {
		return fmt.Errorf("file size %s mis-match: %d != %d", pfinfoRepo.path, f.Size(), pfinfoLocal.info.Size())
	}

	// TODO: this jumps from 0% to 100% ... not ideal but there is no way with to get upload progression with the webdav client library
	bar.Add64(f.Size())

	return nil
}

// getRepoDir downloads a directory from the repository recusively.
func getRepoDir(pfinfoRepo, pfinfoLocal pathFileInfo, showProgress bool) error {

	// read the entire content of the dir
	files, err := cli.ReadDir(pfinfoRepo.path)
	if err != nil {
		return err
	}

	// channel for getting files concurrently.
	fchan := make(chan pathFileInfo)

	// initalize concurrent workers
	var wg sync.WaitGroup
	nworkers := 4
	// counter for file deletion error by worker
	cntErrFiles := make([]int, nworkers)
	for i := 0; i < nworkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for finfo := range fchan {

				// local file path
				_pfinfoLocal := pathFileInfo{
					path: filepath.Join(pfinfoLocal.path, path.Base(finfo.path)),
				}

				log.Debugf("getting %s to %s ...", finfo.path, _pfinfoLocal.path)
				if err := getRepoFile(finfo, _pfinfoLocal, showProgress); err != nil {
					log.Errorf("fail getting file: %s", finfo.path)
					cntErrFiles[id]++
				}
			}
		}(i)
	}

	// counter for overall getting errors
	cntErr := 0
	// loop over content
	for _, finfo := range files {
		p := path.Join(pfinfoRepo.path, finfo.Name())
		if finfo.IsDir() {

			// repo dir pathInfo
			_pfinfoRepo := pathFileInfo{
				path: p,
				info: finfo,
			}
			// local dir pathInfo
			_pfinfoLocal := pathFileInfo{
				path: filepath.Join(pfinfoLocal.path, finfo.Name()),
			}

			if err := os.MkdirAll(_pfinfoLocal.path, _pfinfoRepo.info.Mode()); err != nil {
				log.Errorf("fail creating local dir %s: %s", _pfinfoLocal.path, err)
				cntErr++
				continue
			}

			if err := getRepoDir(_pfinfoRepo, _pfinfoLocal, showProgress); err != nil {
				log.Errorf("fail get subdir %s: %s", p, err)
				cntErr++
			}
		} else {
			fchan <- pathFileInfo{
				path: p,
				info: finfo,
			}
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
		return fmt.Errorf("%d files/subdirs not downloaded", cntErr)
	}

	// remove the directory itself
	log.Debugf("getting %s ...", pfinfoRepo.path)

	return nil
}

// getRepoFile downloads a single file from the repository to a local file.
func getRepoFile(pfinfoRepo, pfinfoLocal pathFileInfo, showProgress bool) error {

	// open pathLocal
	fileLocal, err := os.OpenFile(pfinfoLocal.path, os.O_WRONLY|os.O_CREATE, pfinfoRepo.info.Mode())
	if err != nil {
		return fmt.Errorf("cannot create/write local file: %s", err)
	}
	defer fileLocal.Close()

	// progress bar
	barDesc := prettifyProgressbarDesc(pfinfoRepo.info.Name())
	bar := pb.DefaultBytesSilent(pfinfoRepo.info.Size(), barDesc)
	if showProgress {
		bar = pb.DefaultBytes(pfinfoRepo.info.Size(), barDesc)
	}

	// multiwriter: destination local file, and progress bar
	writer := io.MultiWriter(fileLocal, bar)

	// read pathRepo and write to pathLocal
	reader, err := cli.ReadStream(pfinfoRepo.path)
	if err != nil {
		return fmt.Errorf("cannot open file in repository: %s", err)
	}
	defer reader.Close()

	buffer := make([]byte, 4*1024*1024) // 4MiB buffer

	for {
		// read content to buffer
		rlen, rerr := reader.Read(buffer)
		log.Debugf("read %d", rlen)
		if rerr != nil && rerr != io.EOF {
			return fmt.Errorf("failure reading data from %s: %s", pfinfoRepo.path, rerr)
		}
		wlen, werr := writer.Write(buffer[:rlen])
		if werr != nil {
			return fmt.Errorf("failure writing data to %s: %s", pfinfoLocal.path, err)
		}
		log.Debugf("write %d", wlen)

		if rerr == io.EOF {
			break
		}
	}

	return nil
}

// rmRepoDir removes the directory `path` from the repository recursively.
func rmRepoDir(repoPath string, recursive bool) error {

	if !filepath.IsAbs(repoPath) {
		return fmt.Errorf("not an absolute path: %s", repoPath)
	}

	// read the entire content of the dir
	files, err := cli.ReadDir(repoPath)
	if err != nil {
		return err
	}

	if len(files) > 0 && !recursive {
		return fmt.Errorf("directory not empty: %s", repoPath)
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
		p := path.Join(repoPath, f.Name())
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
	log.Debugf("removing %s ...", repoPath)
	return cli.Remove(repoPath)
}

// prettifyProgressbarDesc returns a shortened description string up to 15 UTF-8 characters.
func prettifyProgressbarDesc(origin string) string {

	maxLen := 33

	if utf8.RuneCountInString(origin) <= maxLen {
		return fmt.Sprintf("%-33s", origin)
	}

	chars := []rune(origin)

	return fmt.Sprintf("%-15s...%-15s", string(chars[:15]), string(chars[len(origin)-15:]))
}
