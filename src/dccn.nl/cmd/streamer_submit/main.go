package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"io/ioutil"
	"path/filepath"
	"time"

	"dccn.nl/config"
	"dccn.nl/dataflow"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var (
	// optsStreamerHost     *string
	// optsStreamerPort     *int
	// optsStreamerUsername *string
	// optsStreamerPassword *string
	optsStreamerConfig   *string
	optsPacsObject       *string
	optsNthreads         *int
	optsVerbose          *bool
)

func init() {
	optsStreamerConfig = flag.String("c", "config.yml", "set the configuration path for connecting to the streamer server.")
	// optsStreamerHost = flag.String("h", "pacs.dccn.nl", "set the streamer server hostname, overwriting value from the -c option.")
	// optsStreamerPort = flag.Int("p", 3001, "set the streamer server network port, overwriting the value from the -c option.")
	// optsStreamerUsername = flag.String("u", "", "set the streamer server connection user, overwriting the value from the -c option.")
	// optsStreamerPassword = flag.String("s", "", "set the streamer server connection password, overwriting the value from the -c option.")

	optsPacsObject = flag.String("o", "study", "indicates the PACS object to be submitted to the streamer")
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
	fmt.Printf("\nSubmitting a data streamer job for streaming a study or series from PACS to the project storage and (if applicable) the Donders Repository\n")
	fmt.Printf("\nUSAGE: %s [OPTIONS] <object_uuid>\n", os.Args[0])
	fmt.Printf("\nOPTIONS:\n")
	flag.PrintDefaults()
	fmt.Printf("\n")
}

func newHTTPSClient() (client *http.Client) {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout: 5 * time.Second,
	}

	client = &http.Client{
		Timeout:   10 * time.Second,
		Transport: transport,
	}

	return
}

func main() {

	if len(flag.Args()) < 1 {
		log.Fatal("require an PACS object UUID")
	}

	// load configuration
	cfg, err := filepath.Abs(*optsStreamerConfig)
	if err != nil {
		log.Fatalf("cannot resolve config path: %s", *optsStreamerConfig)
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

	var serieses []string
	if *optsPacsObject == "series" {
		// append all provided servies IDs
		serieses = append(serieses, flag.Args()...)
	}
	if *optsPacsObject == "study" {
		// retrieve serieses of the study
		o := orthanc.Orthanc{
			PrefixURL: conf.PACS.PrefixURL,
			Username:  conf.PACS.Username,
			Password:  conf.PACS.Password,
		}
		for _, id := range flag.Args() {
			log.Debugf("getting study %s...\n", id)
			study, err := o.GetStudy(id)
			if err != nil {
				log.Fatalf("Error getting study %s: %+v\n", id, err)
			}
			serieses = append(serieses, study.Series...)
		}
	}

	cli := newHTTPSClient()
	for _, sid := range serieses {
		log.Infof("submitting streamer job for series %s...\n", sid)
		req, err := http.NewRequest("POST", conf.Streamer.PrefixURL+"/mri/series/"+sid, nil)
		if err != nil {
			log.Error(err)
			continue
		}
		req.SetBasicAuth(conf.Streamer.Username, conf.Streamer.Password)
		res, err := cli.Do(req)
		if err != nil {
			log.Error(err)
			continue
		}
		if res.StatusCode != 200 {
			log.Error(err)
		}
		if httpBodyBytes, err := ioutil.ReadAll(res.Body); err == nil {
			log.Infof("%s\n", string(httpBodyBytes))
		}
	}
}
