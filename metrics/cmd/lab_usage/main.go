package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
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
	optsConfig          *string
	optsOpenTSDBPushURL *string
)

func init() {

	// CLI options
	optsDateFrom = flag.String("f", time.Now().Format(time.RFC3339[:10]), "set the `from` of the date range")
	optsDateTo = flag.String("t", time.Now().Format(time.RFC3339[:10]), "set the `to` of the date range")
	optsVerbose = flag.Bool("v", false, "print debug messages")
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
	fmt.Printf("\npush lab usage metrics to OpenTSDB.\n")
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

	bookings, err := ipdb.GetLabBookings(pdb.ALL, *optsDateFrom)
	if err != nil {
		log.Errorf("cannot retrieve labbookings, reason: %+v", err)
		os.Exit(100)
	}

	dpoints := make([]MetricOpenTSDB, 0)
	for _, booking := range bookings {
		dpoints = append(dpoints, MetricOpenTSDB{
			Metric:    "lab.usage",
			Timestamp: booking.StartTime.Unix(),
			Value:     math.Round(booking.EndTime.Sub(booking.StartTime).Hours()*10) / 10,
			Tags: map[string]string{
				"status":  booking.Status,
				"project": booking.Project,
				"lab":     booking.Modality,
			},
		})
	}

	for _, p := range dpoints {
		if b, err := json.Marshal(p); err == nil {
			log.Infof(string(b))
		}
	}

	// TODO: derive `lab.free` metrics

}
