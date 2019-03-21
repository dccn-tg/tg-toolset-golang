package pdb

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Donders-Institute/tg-toolset-golang/pkg/config"
	"github.com/go-sql-driver/mysql"
	"github.com/spf13/viper"
)

var conf config.Configuration

func init() {
	// load configuration
	cfg := filepath.Join(os.Getenv("GOPATH"), "src/github.com/Donders-Institute/tg-toolset-golang/configs/config_test.yml")
	viper.SetConfigFile(cfg)
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Sprintf("Error reading config file, %s", err))
	}
	err := viper.Unmarshal(&conf)
	if err != nil {
		panic(fmt.Sprintf("unable to decode into struct, %s", err))
	}
}

func TestSelectPendingRoleMap(t *testing.T) {

	config := mysql.Config{
		Net:                  "tcp",
		Addr:                 fmt.Sprintf("%s:%d", conf.PDB.HostSQL, conf.PDB.PortSQL),
		DBName:               conf.PDB.DatabaseSQL,
		User:                 conf.PDB.UserSQL,
		Passwd:               conf.PDB.PassSQL,
		AllowNativePasswords: true,
		ParseTime:            true,
	}

	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Errorf("Fail connecting SQL database: %+v", err)
	}
	defer db.Close()

	projectRoleActionMap, err := SelectPendingRoleMap(db)

	if err != nil {
		t.Errorf("Fail getting pending role actions: %+v", err)
	}

	for p, roleActionMap := range projectRoleActionMap {
		t.Logf("%s: %+v", p, roleActionMap)
	}

}

func TestSelectPdbUser(t *testing.T) {

	config := mysql.Config{
		Net:                  "tcp",
		Addr:                 fmt.Sprintf("%s:%d", conf.PDB.HostSQL, conf.PDB.PortSQL),
		DBName:               conf.PDB.DatabaseSQL,
		User:                 conf.PDB.UserSQL,
		Passwd:               conf.PDB.PassSQL,
		AllowNativePasswords: true,
		ParseTime:            true,
	}
	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Errorf("Fail connecting SQL database: %+v", err)
	}
	defer db.Close()

	// a known user in the Project database
	knownUser := User{
		ID:         "honlee",
		Firstname:  "Hurng-Chun",
		Middlename: "",
		Lastname:   "Lee",
		Email:      "h.lee@donders.ru.nl",
	}

	u, err := SelectUser(db, "honlee")

	if err != nil {
		t.Errorf("Fail finding user: %+v", err)
	}

	if *u != knownUser {
		t.Errorf("Expect user %+v, found %+v", knownUser, *u)
	}
}
