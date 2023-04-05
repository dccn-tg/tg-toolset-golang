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

	// timzone location
	loc, _ := time.LoadLocation(pdb.Location)

	for _, booking := range bookings {

		// skip bookings not associated with a modality, e.g. MRISUPPORT
		if booking.Modality == "" {
			continue
		}

		tag := map[string]string{
			"status":  booking.Status,
			"project": booking.Project,
			"lab":     labelize(booking.Lab),
			"bill":    labelize(booking.Modality),
			"group":   labelize(booking.Group),
			"source":  booking.FundingSource,
		}

		if !dateEqual(booking.StartTime, booking.EndTime) {
			// overnights event should be split into multiple data points
			i := 0

			startDay := time.Date(
				booking.StartTime.Year(),
				booking.StartTime.Month(),
				booking.StartTime.Day(),
				0, 0, 0, 0,
				loc,
			)

			for day := startDay; !day.After(booking.EndTime); day = day.AddDate(0, 0, 1) {

				// starting date
				y := day.Year()
				m := day.Month()
				d := day.Day()

				var stime time.Time
				var etime time.Time

				switch {
				case dateEqual(day, booking.StartTime):
					// starting date
					stime = booking.StartTime
					etime = time.Date(y, m, d, 24, 0, 0, 0, loc)
				case dateEqual(day, booking.EndTime):
					// ending date
					stime = time.Date(y, m, d, 0, 0, 0, 0, loc)
					etime = booking.EndTime
				default:
					// full days
					stime = time.Date(y, m, d, 0, 0, 0, 0, loc)
					etime = time.Date(y, m, d, 24, 0, 0, 0, loc)
				}

				dpoints = append(dpoints, MetricOpenTSDB{
					Metric:    "lab.usage",
					Timestamp: stime.Unix(),
					Value:     math.Round(etime.Sub(stime).Hours()*100) / 100,
					Tags:      tag,
				})

				i += 1
			}
		} else {
			// handle the situation that start date is not the date provided by `-d` option.
			dpoints = append(dpoints, MetricOpenTSDB{
				Metric:    "lab.usage",
				Timestamp: booking.StartTime.Unix(),
				Value:     math.Round(booking.EndTime.Sub(booking.StartTime).Hours()*100) / 100,
				Tags:      tag,
			})
		}
	}

	// TODO: derive `lab.free` metrics

	if *optsDryrun {
		for _, p := range dpoints {
			if b, err := json.Marshal(p); err == nil {
				log.Infof(string(b))
			}
		}
	} else {
		// the chunksize is chosen to keep the POST data to OpenTSDB less than 4096 bytes,
		// which is the default value of `tsd.http.request.max_chunk`.
		chunckSize := 15
		for i := 0; i < len(dpoints); i += chunckSize {
			end := i + chunckSize
			if end > len(dpoints) {
				end = len(dpoints)
			}
			if err := pushMetric(dpoints[i:end]); err != nil {
				log.Errorf("fail to push lab usage metric: %s", err)
			} else {
				log.Infof("pushed %d data points", len(dpoints[i:end]))
			}
		}
	}
}

func dateEqual(date1, date2 time.Time) bool {
	y1, m1, d1 := date1.Date()
	y2, m2, d2 := date2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
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

	log.Debugf("POST request: %s", reqBody)

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

	log.Debugf("POST response: code %d, %s", resp.StatusCode, string(body))

	if resp.StatusCode >= 400 {
		return fmt.Errorf("status code >= 400: %d", resp.StatusCode)
	}

	return nil
}
