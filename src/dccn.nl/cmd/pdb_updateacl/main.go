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

	"dccn.nl/config"
	"dccn.nl/project"
	"dccn.nl/project/acl"
	ufp "dccn.nl/utility/filepath"
	"github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var (
	optsBase     *string
	optsConfig   *string
	optsNthreads *int
	optsVerbose  *bool
)

func init() {
	optsBase = flag.String("d", "/project", "set the root path of project storage")
	optsConfig = flag.String("c", "config.yml", "set the path of the configuration file")
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
	fmt.Printf("\nUpdating data-access roles of all projects (or a specific project) into the project database.\n")
	fmt.Printf("\nUSAGE: %s [OPTIONS] [projectId]\n", os.Args[0])
	fmt.Printf("\nOPTIONS:\n")
	flag.PrintDefaults()
	fmt.Printf("\n")
}

func main() {

	// load configuration
	cfg, err := filepath.Abs(*optsConfig)
	if err != nil {
		log.Fatalf("cannot resolve config path: %s", *optsConfig)
	}

	if _, err := os.Stat(cfg); err != nil {
		log.Fatalf("cannot load config: %s", cfg)
	}

	viper.SetConfigFile(cfg)
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file, %s", err)
	}
	var conf config.Configuration
	err = viper.Unmarshal(&conf)
	if err != nil {
		log.Fatalf("unable to decode into struct, %v", err)
	}
	log.Debugf("loaded configuration: %+v", conf)

	// channel of passing project's absolute path
	chanPrj := make(chan os.FileInfo)

	args := flag.Args()

	// go routine populating the absolute paths of all projects found under *optsBase.
	go func() {
		defer close(chanPrj)

		// loop over subdirectories within the *optsBase
		if len(args) < 1 {
			projects, err := ioutil.ReadDir(*optsBase)
			if err != nil {
				log.Fatal(err)
			}
			for _, info := range projects {
				chanPrj <- info
			}
			return
		}

		// resolve each args into os.FileInfo
		for _, pid := range args {
			info, err := os.Lstat(filepath.Join(*optsBase, pid))
			if err != nil {
				log.Fatal(err)
			}

			chanPrj <- info
		}
	}()

	// loop over projects with parallel workers.
	// The number of workers is defined by the input option *optsNthreads
	dbConfig := mysql.Config{
		Net:                  "tcp",
		Addr:                 fmt.Sprintf("%s:%d", conf.PDB.HostSQL, conf.PDB.PortSQL),
		DBName:               conf.PDB.DatabaseSQL,
		User:                 conf.PDB.UserSQL,
		Passwd:               conf.PDB.PassSQL,
		AllowNativePasswords: true,
		ParseTime:            true,
	}

	log.Debugf("db configuration: %+v", dbConfig)

	db, err := sql.Open("mysql", dbConfig.FormatDSN())
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
		} else {
			p = referent
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
