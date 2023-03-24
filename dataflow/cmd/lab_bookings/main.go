package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	optsJson    *bool
)

func init() {
	optsDate = flag.String("d", time.Now().Format(time.RFC3339[:10]), "set the `date` of the bookings")
	optsConfig = flag.String("c", "config.yml", "set the `path` of the configuration file")
	flag.Var(&optsLabMod, "l", "set the `modality` for the bookings")
	optsVerbose = flag.Bool("v", false, "print debug messages")
	optsJson = flag.Bool("j", false, "output in json format")

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

	ipdb, err := pdb.New(conf.PDB)
	if err != nil {
		log.Fatalf("cannot connect to the project database: %s", err)
	}

	bookings, err := ipdb.GetLabBookingsForWorklist(optsLabMod, *optsDate)
	if err != nil {
		log.Errorf("cannot retrieve labbookings, reason: %+v", err)
		os.Exit(100)
	}

	if *optsJson {
		if out, err := json.Marshal(bookings); err != nil {
			log.Fatalf("cannot format output in JSON: %s", err)
		} else {
			fmt.Println(string(out))
		}
	} else {
		// sort bookings by start time
		sort.Slice(bookings, func(i, j int) bool {
			return bookings[i].StartTime.Before(bookings[j].StartTime)
		})
		for _, lb := range bookings {
			var name string
			if lb.Operator.Middlename != "" {
				name = fmt.Sprintf("%s %s %s", lb.Operator.Firstname, lb.Operator.Middlename, lb.Operator.Lastname)
			} else {
				name = fmt.Sprintf("%s %s", lb.Operator.Firstname, lb.Operator.Lastname)
			}

			// handle the situation that start date is not the date provided by `-d` option.
			h := lb.StartTime.Hour()
			m := lb.StartTime.Minute()
			s := lb.StartTime.Second()
			dtime, _ := time.Parse("2006-01-02", *optsDate)
			if !dateEqual(lb.StartTime, dtime) {
				h = 0
				m = 0
				s = 0
			}

			fmt.Printf("%02d:%02d:%02d|%s|%9s-%1s|%10s|%s\n",
				h, m, s,
				lb.Project, lb.Subject,
				lb.Session, lb.Lab, name)
		}
	}

	os.Exit(0)
}

func dateEqual(date1, date2 time.Time) bool {
	y1, m1, d1 := date1.Date()
	y2, m2, d2 := date2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}
