package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"dccn.nl/config"
	"dccn.nl/project/cdb"
	"github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var (
	optsDate    *string
	optsConfig  *string
	optsVerbose *bool
)

func init() {
	optsDate = flag.String("d", time.Now().Format(time.RFC3339[:10]), "set the root path of project storage")
	optsConfig = flag.String("c", "config.yml", "set the path of the configuration file")
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
	fmt.Printf("\nGetting bookings of the MEG lab on a given date.\n")
	fmt.Printf("\nUSAGE: %s [OPTIONS]\n", os.Args[0])
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

	// connect to calendar booking database
	dbConfig := mysql.Config{
		Net:                  "tcp",
		Addr:                 fmt.Sprintf("%s:%d", conf.CDB.HostSQL, conf.CDB.PortSQL),
		DBName:               conf.CDB.DatabaseSQL,
		User:                 conf.CDB.UserSQL,
		Passwd:               conf.CDB.PassSQL,
		AllowNativePasswords: true,
		ParseTime:            true,
	}

	log.Debugf("db configuration: %+v", dbConfig)

	db, err := sql.Open("mysql", dbConfig.FormatDSN())
	if err != nil {
		log.Errorf("Fail connecting SQL database: %+v", err)
	}
	defer db.Close()

	bookings, err := cdb.SelectLabBookings(db, cdb.MEG, *optsDate)
	if err != nil {
		log.Errorf("cannot retrieve labbookings, reason: %+v", err)
		os.Exit(100)
	}
	for _, lb := range bookings {
		var name string
		if lb.Operator.Middlename != "" {
			name = fmt.Sprintf("%s %s %s", lb.Operator.Firstname, lb.Operator.Middlename, lb.Operator.Lastname)
		} else {
			name = fmt.Sprintf("%s %s", lb.Operator.Firstname, lb.Operator.Lastname)
		}
		fmt.Printf("%02d:%02d:%02d|%s|%9s-%s|%s\n",
			lb.StartTime.Hour(), lb.StartTime.Minute(), lb.StartTime.Second(),
			lb.Project, lb.Subject,
			lb.Session, name)
	}
	os.Exit(0)
}
