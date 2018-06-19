package cdb

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dccn.nl/config"
	"github.com/go-sql-driver/mysql"
	"github.com/spf13/viper"
)

var conf config.Configuration

func init() {
	// load configuration
	cfg := filepath.Join(os.Getenv("GOPATH"), "etc/config_test.yml")
	viper.SetConfigFile(cfg)
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Sprintf("Error reading config file, %s", err))
	}
	err := viper.Unmarshal(&conf)
	if err != nil {
		panic(fmt.Sprintf("unable to decode into struct, %s", err))
	}
}

func TestSelectLabBookings(t *testing.T) {
	config := mysql.Config{
		Net:                  "tcp",
		Addr:                 fmt.Sprintf("%s:%d", conf.CDB.HostSQL, conf.CDB.PortSQL),
		DBName:               conf.CDB.DatabaseSQL,
		User:                 conf.CDB.UserSQL,
		Passwd:               conf.CDB.PassSQL,
		AllowNativePasswords: true,
		ParseTime:            true,
	}

	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Errorf("Fail connecting SQL database: %+v", err)
	}
	defer db.Close()

	bookings, err := SelectLabBookings(db, MEG, time.Now().Format(time.RFC3339)[:10])

	if err != nil {
		t.Errorf("Fail getting bookings: %+v", err)
	}

	for _, b := range bookings {
		t.Logf("%s", b)
	}
}
