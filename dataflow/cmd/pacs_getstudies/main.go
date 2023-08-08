package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dccn-tg/tg-toolset-golang/dataflow/pkg/orthanc"
	"github.com/dccn-tg/tg-toolset-golang/pkg/config"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var (
	// optsPacsHost     *string
	// optsPacsPort     *int
	// optsPacsUsername *string
	// optsPacsPassword *string
	optsPacsConfig  *string
	optsOlderThan   *int
	optsYoungerThan *int
	optsNthreads    *int
	optsVerbose     *bool
)

func init() {
	optsYoungerThan = flag.Int("y", 2, "get the studies `younger than` the given hours.")
	optsOlderThan = flag.Int("o", 1, "get the studies `older than` the given hours.")
	optsPacsConfig = flag.String("c", "config.yml", "set the configuration path for connecting to the PACS server.")
	// optsPacsHost = flag.String("h", "pacs.dccn.nl", "set the PACS server hostname, overwriting value from the -c option.")
	// optsPacsPort = flag.Int("p", 8042, "set the PACS server network port, overwriting the value from the -c option.")
	// optsPacsUsername = flag.String("u", "", "set the PACS server connection user, overwriting the value from the -c option.")
	// optsPacsPassword = flag.String("s", "", "set the PACS server connection password, overwriting the value from the -c option.")

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
	fmt.Printf("\nGetting DICOM studies from the PACS server.\n")
	fmt.Printf("\nUSAGE: %s [OPTIONS]\n", os.Args[0])
	fmt.Printf("\nOPTIONS:\n")
	flag.PrintDefaults()
	fmt.Printf("\n")
}

func main() {

	// check validity of the age options
	if *optsYoungerThan <= *optsOlderThan {
		log.Fatalf("invalid -o and -y options.  value of -y should be larger than the value of -o.")
	}

	// load configuration
	cfg, err := filepath.Abs(*optsPacsConfig)
	if err != nil {
		log.Fatalf("cannot resolve config path: %s", *optsPacsConfig)
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

	o := orthanc.Orthanc{
		PrefixURL: conf.PACS.PrefixURL,
		Username:  conf.PACS.Username,
		Password:  conf.PACS.Password,
	}

	// get studies conducted in the last 24 hours
	tBeg := time.Now().Add(time.Hour * -1 * time.Duration(*optsYoungerThan))
	tEeg := time.Now().Add(time.Hour * -1 * time.Duration(*optsOlderThan))

	studies, err := o.GetStudies(tBeg, tEeg)
	if err != nil {
		log.Errorf("Fail getting serieses: %+v", err)
	}

	for _, s := range studies {
		ds := s.MainDicomTags.StudyDate
		ts := s.MainDicomTags.StudyTime
		dts := time.Date(ds.Year(), ds.Month(), ds.Day(), ts.Hour(), ts.Minute(), ts.Second(), 0, time.Now().Location())

		// verbosed output
		if *optsVerbose {
			log.Infof("study %s, date: %s, nseries: %d", s.ID, dts, len(s.Series))
			if bs, err := json.MarshalIndent(s, "", "\t"); err == nil {
				log.Debugf("\n----- Detail -----\n%s\n------------------\n", bs)
			}
			continue
		}

		// simple output for reusing it in another program.
		fmt.Printf("%s,%s\n", s.ID, dts)
	}

}
