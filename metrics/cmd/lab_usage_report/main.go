package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Donders-Institute/tg-toolset-golang/pkg/config"
	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/pdb"
)

// MetricOpenTSDB is the generic data structure for OpenTSDB metric.
// an example data below:
// ```
// {"timestamp": 1679482800, "metric": "lab.usage", "value": 1.0, "tags": {"status": "CONFIRMED", "group": "Technical_Group", "bill": "MEG", "lab": "MEG_Lab", "project": "3055020.03", "source": "3055020"}}
// ```
type MetricOpenTSDB struct {
	Metric    string            `json:"metric"`
	Timestamp int64             `json:"timestamp"`
	Value     float64           `json:"value"`
	Tags      map[string]string `json:"tags"`
}

var (
	optsDateFrom        *string
	optsDateTo          *string
	optsVerbose         *bool
	optsDryrun          *bool
	optsConfig          *string
	optsOpenTSDBPushURL *string
)

func init() {

	yesterday := time.Now().Add(-24 * time.Hour).Format(time.RFC3339[:10])

	// CLI options
	optsDateFrom = flag.String("f", yesterday, "set the from `date` of the date range")
	optsDateTo = flag.String("t", yesterday, "set the to `date` of the date range")
	optsVerbose = flag.Bool("v", false, "print debug messages")
	optsDryrun = flag.Bool("d", false, "perform dryrun without push metrics to the OpenTSDB server")
	optsConfig = flag.String("c", "config.yml", "set the `path` of the configuration file")
	optsOpenTSDBPushURL = flag.String("l", "http://opentsdb:4242/api/put", "set OpenTSDB `endpoint` for pushing metrics")

	flag.Usage = usage

	flag.Parse()

	// configuration for logger
	cfg := log.Configuration{
		EnableConsole:     true,
		ConsoleJSONFormat: false,
		ConsoleLevel:      log.Info,
	}

	if *optsVerbose {
		cfg.ConsoleLevel = log.Debug
	}

	// initialize logger
	log.NewLogger(cfg, log.InstanceLogrusLogger)
}

func usage() {
	fmt.Printf("\npush lab usage metrics to OpenTSDB. By default it pushes the usage of yesterday.\n")
	fmt.Printf("\nUSAGE: %s [OPTIONS]\n", os.Args[0])
	fmt.Printf("\nOPTIONS:\n")
	flag.PrintDefaults()
}

func main() {
	// get booking events of today
	// compose and push events to `lab.usage` metrics, using the event start time as the timestamp
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

	bookings, err := ipdb.GetLabBookingsForReport(pdb.ALL, *optsDateFrom, *optsDateTo)
	if err != nil {
		log.Errorf("cannot retrieve labbookings, reason: %+v", err)
		os.Exit(100)
	}

	dpoints := make([]MetricOpenTSDB, 0)
	for _, booking := range bookings {
		dpoints = append(dpoints, MetricOpenTSDB{
			Metric:    "lab.usage",
			Timestamp: booking.StartTime.Unix(),
			Value:     math.Round(booking.EndTime.Sub(booking.StartTime).Hours()*100) / 100,
			Tags: map[string]string{
				"status":  booking.Status,
				"project": booking.Project,
				"lab":     labelize(booking.Lab),
				"bill":    labelize(booking.Modality),
				"group":   labelize(booking.Group),
				"source":  booking.FundingSource,
			},
		})
	}

	// TODO: derive `lab.free` metrics

	if *optsDryrun {
		for _, p := range dpoints {
			if b, err := json.Marshal(p); err == nil {
				log.Infof(string(b))
			}
		}
	} else {
		if err := pushMetric(dpoints); err != nil {
			log.Errorf("fail to push lab usage metric: %s", err)
		}
	}
}

func labelize(t string) string {

	s := t

	s = strings.ReplaceAll(s, `  `, ` `)
	s = strings.ReplaceAll(s, ` `, `_`)
	s = strings.ReplaceAll(s, `(`, `/`)
	s = strings.ReplaceAll(s, `)`, `/`)
	s = strings.ReplaceAll(s, `,`, `.`)
	s = strings.ReplaceAll(s, `&`, `and`)

	return s
}

func pushMetric(dpoints []MetricOpenTSDB) error {
	reqBody, err := json.Marshal(dpoints)
	if err != nil {
		return err
	}

	log.Infof("%s", reqBody)

	// POST data to OpenTSDB endpoint
	resp, err := http.Post(*optsOpenTSDBPushURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	log.Debugf(string(body))
	return nil
}
