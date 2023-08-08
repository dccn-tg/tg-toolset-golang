package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/dccn-tg/tg-toolset-golang/dataflow/pkg/orthanc"
	"github.com/dccn-tg/tg-toolset-golang/pkg/config"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var (
	// optsStreamerHost     *string
	// optsStreamerPort     *int
	// optsStreamerUsername *string
	// optsStreamerPassword *string
	optsStreamerConfig *string
	optsPacsObject     *string
	optsVerbose        *bool
)

func init() {
	optsStreamerConfig = flag.String("c", "config.yml", "set the configuration path for connecting to the streamer and the PACS servers.")
	// optsStreamerHost = flag.String("h", "pacs.dccn.nl", "set the streamer server hostname, overwriting value from the -c option.")
	// optsStreamerPort = flag.Int("p", 3001, "set the streamer server network port, overwriting the value from the -c option.")
	// optsStreamerUsername = flag.String("u", "", "set the streamer server connection user, overwriting the value from the -c option.")
	// optsStreamerPassword = flag.String("s", "", "set the streamer server connection password, overwriting the value from the -c option.")

	optsPacsObject = flag.String("o", "study", "indicate the PACS object `type` (study or series) of the provided object UUIDs.")
	optsVerbose = flag.Bool("v", false, "print debug messages")

	flag.Usage = usage

	flag.Parse()

	// check if provided object is supported
	if *optsPacsObject != "series" && *optsPacsObject != "study" {
		log.Fatalf("Unsupported PACS object: %s\n", *optsPacsObject)
	}

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
	fmt.Printf("\nUSAGE: %s [OPTIONS] <UUID_1> <UUID_2>...\n", os.Args[0])
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
