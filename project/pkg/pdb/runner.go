package pdb

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sync"

	"github.com/Donders-Institute/tg-toolset-golang/pkg/config"
	ufp "github.com/Donders-Institute/tg-toolset-golang/pkg/filepath"
	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/acl"
	"github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Runner implements high-level functions for interacting with the project data.
type Runner struct {
	// ConfigFile is the path to the YAML configuration file in which the project
	// database server connection information is specified.
	ConfigFile string
	// Nthreads defines number of workers interact with the project database
	// concurrently to speed up the operation.
	Nthreads int
}

// getPdbConfig loads the configuration file, and returns the database settings
// for making the connection to the project database.
func (r Runner) getPdbConfig() (mysql.Config, error) {

	var dbConfig mysql.Config

	// load configuration
	cfg, err := filepath.Abs(r.ConfigFile)
	if err != nil {
		return dbConfig, fmt.Errorf("cannot resolve config path: %s", r.ConfigFile)
	}

	if _, err := os.Stat(cfg); err != nil {
		return dbConfig, fmt.Errorf("cannot load config: %s", cfg)
	}

	viper.SetConfigFile(cfg)
	if err := viper.ReadInConfig(); err != nil {
		return dbConfig, fmt.Errorf("Error reading config file, %s", err)
	}
	var conf config.Configuration
	err = viper.Unmarshal(&conf)
	if err != nil {
		return dbConfig, fmt.Errorf("unable to decode into struct, %v", err)
	}
	log.Debugf("loaded configuration: %+v", conf)

	dbConfig = mysql.Config{
		Net:                  "tcp",
		Addr:                 fmt.Sprintf("%s:%d", conf.PDB.HostSQL, conf.PDB.PortSQL),
		DBName:               conf.PDB.DatabaseSQL,
		User:                 conf.PDB.UserSQL,
		Passwd:               conf.PDB.PassSQL,
		AllowNativePasswords: true,
		ParseTime:            true,
	}

	return dbConfig, nil
}

// UpdateRolesWithStorage synchronize roles of all projects under the `projectRootPath`
// to the project database.
// Roles of subdirectories not taken into account.
func (r Runner) UpdateRolesWithStorage(projectRootPath string) error {

	// loads configuration for making connection to the project database.
	dbConfig, err := r.getPdbConfig()
	if err != nil {
		return err
	}
	log.Debugf("db configuration: %+v", dbConfig)

	// channel of passing project's absolute path
	chanPrj := make(chan os.FileInfo)

	// go routine populating the absolute paths of all projects found under *optsBase.
	go func() {
		defer close(chanPrj)

		// loop over subdirectories within the *optsBase
		projects, err := ioutil.ReadDir(projectRootPath)
		if err != nil {
			log.Fatal(err)
		}
		for _, info := range projects {
			chanPrj <- info
		}
		return
	}()

	// initialize the database connection.
	db, err := sql.Open("mysql", dbConfig.FormatDSN())
	if err != nil {
		return fmt.Errorf("Fail connecting SQL database: %+v", err)
	}
	defer db.Close()

	// start parallel workers within a wait group.
	var wg sync.WaitGroup
	wg.Add(r.Nthreads)
	for i := 0; i < r.Nthreads; i++ {
		go func() {
			defer wg.Done()
			for fi := range chanPrj {
				if fpm, err := ufp.ResolveAndCheckPath(projectRootPath, fi); err != nil {
					log.Warnf("skip for project %s: %s", fi.Name(), err)
				} else {
					if err := updateProjectACL(db, fi.Name(), fpm); err != nil {
						log.Errorf("cannot update roles for project %s: %s", fi.Name(), err)
					}
				}
			}
		}()
	}

	// wait for workers to complete
	wg.Wait()

	return nil
}

// ProvisionOrUpdateStorage creates storage space and update ACLs for projects with
// pending access-role setup.
// The input argument for project ID is optional.  If the value of it is
// a project number, the provision or update is only applied for the specific project.
func (r Runner) ProvisionOrUpdateStorage(pids ...string) error {
	// loads configuration for making connection to the project database.
	dbConfig, err := r.getPdbConfig()
	if err != nil {
		return err
	}
	log.Debugf("db configuration: %+v", dbConfig)

	// initialize the database connection.
	db, err := sql.Open("mysql", dbConfig.FormatDSN())
	if err != nil {
		return fmt.Errorf("Fail connecting SQL database: %+v", err)
	}
	defer db.Close()

	actionMap, err := SelectPendingRoleMap(db)
	if err != nil {
		return fmt.Errorf("Fail getting pending access-role settings: %+v", err)
	}

	// data structure to be passed on the channel.
	type paction struct {
		pid     string
		actions []RoleAction
	}

	chanPrj := make(chan paction)
	go func() {
		defer close(chanPrj)
		// filling up the channel
		if len(pids) == 0 {
			for pid, actions := range actionMap {
				chanPrj <- paction{pid: pid, actions: actions}
			}
		} else {
			for _, pid := range pids {
				if actions, ok := actionMap[pid]; ok {
					chanPrj <- paction{pid: pid, actions: actions}
				}
			}
		}
	}()

	// start parallel workers to perform action in a wait group.
	var wg sync.WaitGroup
	wg.Add(r.Nthreads)
	for i := 0; i < r.Nthreads; i++ {
		go func() {
			defer wg.Done()
			for paction := range chanPrj {
				fmt.Printf("project: %s, actions: %+v\n", paction.pid, paction.actions)
			}
		}()
	}

	// wait for workers to complete
	wg.Wait()

	return nil
}

// updateProjectACL performs actions on retrieving ACLs from the filesystem,
// and updating ACLs in the project database.
func updateProjectACL(db *sql.DB, pid string, fpm *ufp.FilePathMode) error {

	// get the roles from the givne project path
	roler := acl.GetRoler(*fpm)
	if roler == nil {
		return fmt.Errorf("roler not found: %+v", *fpm)
	}
	log.Debug(fmt.Sprintf("path: %s %s", fpm.Path, reflect.TypeOf(roler)))
	roles, err := roler.GetRoles(*fpm)
	if err != nil {
		return fmt.Errorf("cannot retrieve roles: %s, reason: %+v", fpm.Path, err)
	}

	if err := UpdateProjectRoles(db, pid, roles); err != nil {
		return fmt.Errorf("failure updating project database: %s, reason: %+v", fpm.Path, err)
	}

	return nil
}
