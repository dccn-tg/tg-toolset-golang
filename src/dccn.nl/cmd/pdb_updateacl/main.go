package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sync"

	"dccn.nl/project"
	"dccn.nl/project/acl"
	ufp "dccn.nl/utility/filepath"
	"github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
)

var (
	optsBase     *string
	optsNthreads *int
	optsVerbose  *bool
)

func init() {
	optsBase = flag.String("d", "/project", "set the root path of project storage")
	optsNthreads = flag.Int("n", 2, "set number of concurrent processing threads")
	optsVerbose = flag.Bool("v", false, "print debug messages")

	flag.Usage = usage

	flag.Parse()

	// set logging
	log.SetOutput(os.Stderr)
	// set logging level
	llevel := log.InfoLevel
	if *optsVerbose {
		llevel = log.DebugLevel
	}
	log.SetLevel(llevel)
}

func usage() {
	fmt.Printf("\nUpdating projects' data-access roles into the project database.\n")
	fmt.Printf("\nUSAGE: %s [OPTIONS]\n", os.Args[0])
	fmt.Printf("\nOPTIONS:\n")
	flag.PrintDefaults()
	fmt.Printf("\n")
}

func main() {

	// channel of passing project's absolute path
	chanPrj := make(chan os.FileInfo)

	// go routine populating the absolute paths of all projects found under *optsBase.
	go func() {
		defer close(chanPrj)
		projects, err := ioutil.ReadDir(*optsBase)
		if err != nil {
			log.Fatal(err)
		}
		for _, info := range projects {
			chanPrj <- info
		}
	}()

	// loop over projects with parallel workers.
	// The number of workers is defined by the input option *optsNthreads
	config := mysql.Config{
		Net:    "tcp",
		Addr:   "mysql-intranet.dccn.nl:3306",
		DBName: "fcdc",
		User:   "acl",
		Passwd: "test",
	}

	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		log.Errorf("Fail connecting SQL database: %+v", err)
	}
	defer db.Close()

	// start parallel workers within a wait group.
	var wg sync.WaitGroup
	wg.Add(*optsNthreads)
	for i := 0; i < *optsNthreads; i++ {
		go func() {
			defer wg.Done()
			for fpm := range chanPrj {
				updateProjectAcl(db, fpm)
			}
		}()
	}

	// wait for workers to complete
	wg.Wait()
}

// updateProjectAcl performs actions on retrieving ACLs from the filesystem,
// and updating ACLs in the project database.
func updateProjectAcl(db *sql.DB, pinfo os.FileInfo) error {
	p, err := resolveAndCheckProjectPath(pinfo)
	if err != nil {
		return err
	}

	// take project id from the pinfo.Name()
	pid := pinfo.Name()

	// get the roles from the givne project path
	roler := acl.GetRoler(*p)
	if roler == nil {
		return errors.New(fmt.Sprintf("roler not found: %+v", *p))
	}
	log.Debug(fmt.Sprintf("path: %s %s", p.Path, reflect.TypeOf(roler)))
	roles, err := roler.GetRoles(*p)
	if err != nil {
		return errors.New(fmt.Sprintf("cannot retrieve roles: %s, reason: %+v", p.Path, err))
	}

	if err := pdb.UpdateProjectRoles(db, pid, roles); err != nil {
		return errors.New(fmt.Sprintf("failure updating project database: %s, reason: %+v", p.Path, err))
	}

	return nil
}

// resolveAndCheckProjectPath evaulates the project path information, resolves to its
// absolute pate (for symbolic links), and checks whether the absolute path is existing and
// accessible.
func resolveAndCheckProjectPath(pinfo os.FileInfo) (*ufp.FilePathMode, error) {
	p := filepath.Join(*optsBase, pinfo.Name())

	// resolve symlink
	if pinfo.Mode()&os.ModeSymlink != 0 {
		referent, err := os.Readlink(p)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("cannot resolve referent of symlink: %s, reason: %+v", p, err))
		}
		if []rune(referent)[0] != os.PathSeparator {
			p = filepath.Join(p, referent)
		}
	}

	// make the path absolute and clean
	p, _ = filepath.Abs(p)

	// check availability of the path
	stat, err := os.Stat(p)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("project path not found: %s, reason: %+v", p, err))
	}

	fpm := ufp.FilePathMode{
		Path: p,
		Mode: stat.Mode(),
	}

	return &fpm, nil
}
