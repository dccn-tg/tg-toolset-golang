package pdb

import (
	"database/sql"
	"testing"

	"github.com/go-sql-driver/mysql"
)

func TestSelectPendingRoleMap(t *testing.T) {

	config := mysql.Config{
		Net:                  "tcp",
		Addr:                 "mysql-intranet.dccn.nl:3306",
		DBName:               "fcdc",
		User:                 "acl",
		Passwd:               "test",
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
		Addr:                 "mysql-intranet.dccn.nl:3306",
		DBName:               "fcdc",
		User:                 "acl",
		Passwd:               "test",
		AllowNativePasswords: true,
		ParseTime:            true,
	}
	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Errorf("Fail connecting SQL database: %+v", err)
	}
	defer db.Close()

	// a known user in the Project database
	knownUser := PdbUser{
		Id:         "honlee",
		Firstname:  "Hurng-Chun",
		Middlename: "",
		Lastname:   "Lee",
		Email:      "h.lee@donders.ru.nl",
	}

	u, err := SelectPdbUser(db, "honlee")

	if err != nil {
		t.Errorf("Fail finding user: %+v", err)
	}

	if *u != knownUser {
		t.Errorf("Expect user %+v, found %+v", knownUser, *u)
	}
}
