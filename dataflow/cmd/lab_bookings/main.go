package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Donders-Institute/tg-toolset-golang/pkg/config"
	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/pdb"
	log "github.com/sirupsen/logrus"
)

var (
	optsDate    *string
	optsConfig  *string
	optsLabMod  pdb.Lab
	optsVerbose *bool
)

func init() {
	optsDate = flag.String("d", time.Now().Format(time.RFC3339[:10]), "set the `date` of the bookings")
	optsConfig = flag.String("c", "config.yml", "set the `path` of the configuration file")
	flag.Var(&optsLabMod, "l", "set the `modality` for the bookings")
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
	fmt.Printf("\nGetting bookings of a modality on a given date.\n")
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

	conf, err := config.LoadConfig(cfg)
	if err != nil {
		log.Fatalf("cannot load configuration file: %s", err)
	}

	pdb, err := pdb.New(conf.PDB)
	if err != nil {
		log.Fatalf("cannot connect to the project database: %s", err)
	}

	bookings, err := pdb.GetLabBookings(optsLabMod, *optsDate)
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
		fmt.Printf("%02d:%02d:%02d|%s|%9s-%s|%9s|%s\n",
			lb.StartTime.Hour(), lb.StartTime.Minute(), lb.StartTime.Second(),
			lb.Project, lb.Subject,
			lb.Session, lb.Modality, name)
	}
	os.Exit(0)
}
