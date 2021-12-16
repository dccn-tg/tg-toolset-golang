package repocli

import (
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
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

// dataTransferInput is a data structure for the input of file get and put.
type dataTransferInput struct {
	src pathFileInfo
	dst pathFileInfo
}

// Op is the operation options
type Op int

const (
	// Put
	Put Op = iota
	// Get
	Get
)

// current working directory
var cwd string = "/"
var dataDir string
var recursive bool
var overwrite bool

func init() {

	if err := initDataDir(); err != nil {
		log.Fatalf("%s", err)
	}

	rmCmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "remove directory recursively")
	mvCmd.Flags().BoolVarP(&overwrite, "overwrite", "f", false, "overwrite the destination file")

	rootCmd.AddCommand(lsCmd, putCmd, getCmd, rmCmd, mvCmd, mkdirCmd)
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

// // command to change directory in the repository.
// // At the moment, this command makes no sense as the current directory is not persistent.
// var cdCmd = &cobra.Command{
// 	Use:   "cd <repo_directory>",
// 	Short: "Change directory in the repository",
// 	Long:  ``,
// 	Args:  cobra.ExactArgs(1),
// 	RunE: func(cmd *cobra.Command, args []string) error {

// 		p := getCleanRepoPath(args[0])

// 		// stat the path to check if the path is a valid directory
// 		if f, err := cli.Stat(p); err != nil || !f.IsDir() {
// 			return fmt.Errorf("invalid directory: %s", p)
// 		}

// 		// set cwd to the new path
// 		cwd = p
// 		return nil
// 	},
// }

// command to list a file or the content of a directory in the repository.
var lsCmd = &cobra.Command{
	Use:   "ls <repo_file|repo_directory>",
	Short: "List a file or the content of a directory in the repository",
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
	Use:   "put <local_file|local_directory> <repo_file|repo_directory>",
	Short: "Upload a file or a directory at local into a repository directory",
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

		// a file or a directory
		pfinfoLocal := pathFileInfo{
			path: lfpath,
			info: lfinfo,
		}

		p := getCleanRepoPath(args[1])
		f, rerr := cli.Stat(p)

		if lfinfo.IsDir() {

			if rerr == nil && !f.IsDir() {
				return fmt.Errorf("destination not a directory: %s", args[1])
			}

			// the source does not have a tailing path separator.  The whole local directory is
			// created within the specifified directory
			cpath := []rune(args[0])
			if cpath[len(cpath)-1] != os.PathSeparator {
				p = path.Join(p, lfinfo.Name())
			}

			log.Debugf("upload content of %s into %s", pfinfoLocal.path, p)

			// repo pathInfo
			pfinfoRepo := pathFileInfo{
				path: p,
			}

			// create top-level directory in advance
			cli.MkdirAll(pfinfoRepo.path, pfinfoLocal.info.Mode())

			// walk through repo directories
			ichan := make(chan dataTransferInput)
			go walkLocalDirForPut(pfinfoLocal, pfinfoRepo, ichan, true)

			// perform data transfer with 4 concurrent workers
			cntOk, cntErr := transferFiles(Put, ichan, 4)

			// log statistics
			log.Infof("no. succeeded: %d, no. failed: %d", cntOk, cntErr)

			return nil
		} else {

			// path exists in collection, and it is a directory
			// the uploaded file will be put into the directory with the same filename.
			if rerr == nil && f.IsDir() {
				p = path.Join(p, lfinfo.Name())
			}

			// construct filepath at repository
			pfinfoRepo := pathFileInfo{
				path: p,
			}
			return putRepoFile(pfinfoLocal, pfinfoRepo, true)
		}
	},
}

var getCmd = &cobra.Command{
	Use:   "get <repo_file|repo_directory> <local_file|local_directory>",
	Short: "Download a file or a directory from the repository to the current working directory at local",
	Long:  ``,
	Args:  cobra.ExactArgs(2),
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

		// get local path's FileInfo
		lp, err := filepath.Abs(args[1])
		if err != nil {
			return err
		}
		lfinfo, lerr := os.Stat(lp)

		// download recursively
		if f.IsDir() {

			// destination path exists but not a directory.
			if lerr == nil && !lfinfo.IsDir() {
				return fmt.Errorf("destination not a directory: %s", args[1])
			}

			// the source does not have a tailing path separator.  The whole local directory is
			// created within the specifified directory
			cpath := []rune(args[0])
			if cpath[len(cpath)-1] != '/' {
				lp = filepath.Join(lp, path.Base(p))
			}

			pfinfoLocal := pathFileInfo{
				path: lp,
			}

			log.Debugf("download content of %s into %s", pfinfoRepo.path, pfinfoLocal.path)

			if err := os.MkdirAll(lp, pfinfoRepo.info.Mode()); err != nil {
				return err
			}

			// walk through repo directories
			ichan := make(chan dataTransferInput)
			go walkRepoDirForGet(pfinfoRepo, pfinfoLocal, ichan, true)

			// perform data transfer with 4 concurrent workers
			cntOk, cntErr := transferFiles(Get, ichan, 4)

			// log statistics
			log.Infof("no. succeeded: %d, no. failed: %d", cntOk, cntErr)

			return nil

		} else {

			if lerr == nil && lfinfo.IsDir() {
				lp = filepath.Join(lp, path.Base(p))
			}

			pfinfoLocal := pathFileInfo{
				path: lp,
			}

			// download single file
			return getRepoFile(pfinfoRepo, pfinfoLocal, true)
		}

	},
}

var mvCmd = &cobra.Command{
	Use:   "mv <repo_file|repo_directory> <repo_file|repo_directory>",
	Short: "Move or rename a file or directory in the repository",
	Long:  ``,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {

		src := getCleanRepoPath(args[0])
		dst := getCleanRepoPath(args[1])

		// source must exists
		fsrc, err := cli.Stat(src)
		if err != nil {
			return err
		}

		// if the source (1st argument) is a file.
		if !fsrc.IsDir() {
			return mvRepoFile(pathFileInfo{
				path: src,
				info: fsrc,
			}, dst, overwrite)
		}

		return mvRepoDir(pathFileInfo{
			path: src,
			info: fsrc,
		}, dst, overwrite)
	},
}

var rmCmd = &cobra.Command{
	Use:   "rm <file_repo|directory_repo>",
	Short: "Remove a file or a directory from the repository",
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
	Use:   "mkdir <repo_directory>",
	Short: "Create a new directory in the repository",
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

// transferFiles performs transfer operation `Op` with source and destination provided through the
// channel `ichan`.
func transferFiles(op Op, ichan chan dataTransferInput, nworkers int) (cntOk, cntErr int) {

	// initalize concurrent workers
	var wg sync.WaitGroup
	for i := 0; i < nworkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for inputs := range ichan {
				switch op {
				case Put:
					if err := putRepoFile(inputs.src, inputs.dst, true); err != nil {
						cntErr += 1
						continue
					}
				case Get:
					if err := getRepoFile(inputs.src, inputs.dst, true); err != nil {
						cntErr += 1
						continue
					}
				default:
					// do nothing
					log.Errorf("unknown operation: %s", op)
					cntErr += 1
					continue
				}
				cntOk += 1
			}
		}()
	}

	// wait for workers to be released
	wg.Wait()

	return
}

// walkLocalDirForPut walks through a local directory and creates inputs for putting files from local to repo.
func walkLocalDirForPut(pfinfoLocal, pfinfoRepo pathFileInfo, ichan chan dataTransferInput, closeChanOnComplete bool) {
	// read the entire content of the dir
	files, err := ioutil.ReadDir(pfinfoLocal.path)
	if err != nil {
		log.Errorf("cannot read local dir %s: %s", pfinfoLocal.path, err)
		return
	}

	for _, finfo := range files {
		p := path.Join(pfinfoLocal.path, finfo.Name())

		// local dir pathInfo
		_pfinfoLocal := pathFileInfo{
			path: p,
			info: finfo,
		}
		// repo dir pathInfo
		_pfinfoRepo := pathFileInfo{
			path: path.Join(pfinfoRepo.path, finfo.Name()),
		}

		if finfo.IsDir() {
			// create sub directory in advance
			if err := cli.Mkdir(_pfinfoRepo.path, _pfinfoLocal.info.Mode()); err != nil {
				log.Errorf("cannot create repo dir %s: %s", _pfinfoRepo.path, err)
				continue
			}
			// walk into sub directory without closing the channel
			walkLocalDirForPut(_pfinfoLocal, _pfinfoRepo, ichan, false)
		} else {
			ichan <- dataTransferInput{
				src: _pfinfoLocal,
				dst: _pfinfoRepo,
			}
		}
	}

	if closeChanOnComplete {
		close(ichan)
	}
}

// walkRepoDirForGet walks through a repo directory and creates inputs for getting files from repo to local.
func walkRepoDirForGet(pfinfoRepo, pfinfoLocal pathFileInfo, ichan chan dataTransferInput, closeChanOnComplete bool) {

	// read the entire content of the dir
	files, err := cli.ReadDir(pfinfoRepo.path)
	if err != nil {
		log.Errorf("cannot read repo dir: %s", err)
		return
	}

	// loop over content
	for _, finfo := range files {
		p := path.Join(pfinfoRepo.path, finfo.Name())

		// repo dir pathInfo
		_pfinfoRepo := pathFileInfo{
			path: p,
			info: finfo,
		}
		// local dir pathInfo
		_pfinfoLocal := pathFileInfo{
			path: filepath.Join(pfinfoLocal.path, finfo.Name()),
		}

		if finfo.IsDir() {
			// create sub directory in advance
			if err := os.Mkdir(_pfinfoLocal.path, _pfinfoRepo.info.Mode()); err != nil {
				log.Errorf("cannot create local dir %s: %s", _pfinfoLocal.path, err)
				continue
			}
			// walk into sub directory without closing the channel
			walkRepoDirForGet(_pfinfoRepo, _pfinfoLocal, ichan, false)
		} else {
			ichan <- dataTransferInput{
				src: _pfinfoRepo,
				dst: _pfinfoLocal,
			}
		}
	}

	if closeChanOnComplete {
		close(ichan)
	}
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
		if rerr != nil && rerr != io.EOF {
			return fmt.Errorf("failure reading data from %s: %s", pfinfoRepo.path, rerr)
		}
		wlen, werr := writer.Write(buffer[:rlen])
		if werr != nil || rlen != wlen {
			return fmt.Errorf("failure writing data to %s: %s", pfinfoLocal.path, err)
		}

		if rerr == io.EOF {
			break
		}
	}

	return nil
}

// mvRepoDir moves directory from `src` to `dst` recursively.
//
// If the `dst` exists and is a directory, the entire `src` is moved into the `dst`.
//
// If the `dst` does not exist, the `src` is renamed to `dst`.
func mvRepoDir(src pathFileInfo, dst string, overwrite bool) error {

	// read the entire content of the source directory
	files, err := cli.ReadDir(src.path)
	if err != nil {
		return err
	}

	if finfo, err := cli.Stat(dst); err == nil && finfo.IsDir() {
		dst = path.Join(dst, path.Base(src.path))
	}

	// make attempt to create all parent directories of the destination.
	cli.MkdirAll(dst, 0664)

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
				if err := mvRepoFile(finfo, path.Join(dst, finfo.info.Name()), overwrite); err != nil {
					log.Errorf("%s", err)
					cntErrFiles[id]++
				}
			}
		}(i)
	}

	// counter for overall getting errors
	cntErr := 0
	// loop over content
	for _, finfo := range files {
		psrc := path.Join(src.path, finfo.Name())
		pdst := path.Join(dst, path.Base(finfo.Name()))
		if finfo.IsDir() {

			// repo dir pathInfo
			_pfinfoSrc := pathFileInfo{
				path: psrc,
				info: finfo,
			}

			log.Debugf("moving dir %s to %s", psrc, pdst)
			if err := mvRepoDir(_pfinfoSrc, pdst, overwrite); err != nil {
				log.Errorf("fail moving %s to %s: %s", psrc, pdst, err)
				cntErr++
			}
		} else {
			fchan <- pathFileInfo{
				path: psrc,
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
		return fmt.Errorf("%d files/subdirs not moved", cntErr)
	}

	if src.info.IsDir() {
		// remove the moved directory
		if err := cli.Remove(src.path); err != nil {
			log.Errorf("fail removing %s: %s", src.path, err)
		}
	}

	return nil
}

// mvRepoFile moves file `src` to `dst`.
//
// If `dst` exists and is a directory, the `src` is moved into the `dst`.
//
// If `dst` exists and is a file, it returns error unless the `overwrite` flag is set to `true`.
//
// If `dst` doesn't exist, the `src` is renamed to `dst`.
func mvRepoFile(src pathFileInfo, dst string, overwrite bool) error {
	// if the source (1st argument) is a file.
	if src.info.IsDir() {
		return fmt.Errorf("%s not a file", src.path)
	}

	// if the destination specified is an existing directory,
	// the `src` is moved into the `dst` with the same file name.
	if fdst, err := cli.Stat(dst); err == nil && fdst.IsDir() {
		dst = getCleanRepoPath(path.Join(dst, path.Base(src.path)))
	}

	log.Infof("moving %s to %s", src.path, dst)
	return cli.Rename(src.path, dst, overwrite)
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
