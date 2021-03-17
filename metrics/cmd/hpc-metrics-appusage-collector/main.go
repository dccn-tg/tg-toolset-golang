// hpc-metrics-appusage-collector is a daemon for monitoring the HPC software usage by
// collecting data instrumented in the environment modules on the HPC cluster.
//
// This program contains two parts:
//
// - _Collector_ service listens on incoming data sent from the `module load` command.
//   The data is as simple as the string containing the module name and the version.
//
// - _Metrics pusher_ sends POST request to a OpenTSDB endpoint specified by `-l` option
//   every given period of time (default: 10 seconds).
//
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
)

// counterStore is a in-memory temporary data storage for holding
// module counters from the incoming UDP messages between two
// metrics pushes.
type counterStore struct {
	mutex   sync.Mutex
	counter map[string]int
}

func (cs *counterStore) add(module string) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	// initiate counter for the first time
	if _, ok := cs.counter[module]; !ok {
		cs.counter[module] = 0
	}

	// increase counter
	cs.counter[module]++
}

func (cs *counterStore) reset() {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	// reset by new map
	cs.counter = make(map[string]int)

	// // reset by setting existing counter to 0
	// for k := range cs.counter {
	// 	cs.counter[k] = 0
	// }
}

// MetricOpenTSDB is the generic data structure for OpenTSDB metric.
type MetricOpenTSDB struct {
	Metric    string            `json:"metric"`
	Timestamp int64             `json:"timestamp"`
	Value     int               `json:"value"`
	Tags      map[string]string `json:"tags"`
}

var (
	optsVerbose                *bool
	optsNthreads               *int
	optsCollectPeriod          *int
	optsOpenTSDBPushURL        *string
	optsListenAddressCollector *string
	cs                         counterStore
)

func init() {

	// CLI options
	optsVerbose = flag.Bool("v", false, "print debug messages")
	optsNthreads = flag.Int("n", runtime.NumCPU()-1, "set `number` of threads for collector")
	optsCollectPeriod = flag.Int("p", 10, "set `period` in seconds within which the usage is counted")
	optsOpenTSDBPushURL = flag.String("l", "http://opentsdb:4242/api/put", "set OpenTSDB `endpoint` for pushing metrics")
	optsListenAddressCollector = flag.String("c", ":9999", "set listener `address` for collecting data from module load")

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

	// initialize counter
	cs.counter = make(map[string]int)
}

func usage() {
	fmt.Printf("\nstart hpc app usage collector.\n")
	fmt.Printf("\nUSAGE: %s [OPTIONS]\n", os.Args[0])
	fmt.Printf("\nOPTIONS:\n")
	flag.PrintDefaults()
}

func main() {

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	// handle interrupt signals, e.g. Crtl-C
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		log.Infof("Received an interrupt, stopping services...")
		cancel()
	}()

	// launch Collector server
	go serveCollector(ctx, cancel)

	// periodically push metrics with collected usage count
	ticker := time.NewTicker(time.Duration(*optsCollectPeriod) * time.Second)
	go pushMetrics(ctx, ticker)

	// blocking the service
	<-ctx.Done()
}

// serveCollector starts a UDP server with concurrent listeners.
func serveCollector(ctx context.Context, cancel context.CancelFunc) {
	// setup UDP server and connector
	addr, err := net.ResolveUDPAddr("udp", *optsListenAddressCollector)
	if err != nil {
		log.Errorf("cannot resolve Collector address: %s", err)
		cancel()
		return
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Errorf("cannot start Collector service: %s", err)
		cancel()
		return
	}
	defer conn.Close()

	// launch concurrent UDP listeners
	errors := make(chan error)
	for i := 0; i < *optsNthreads; i++ {
		go listen(conn, errors)
	}

	log.Infof("Collector service started")
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errors:
			log.Errorf("%s", err)
		}
	}
}

// listen is a function to handle incoming UDP connection.  It is
// intended to run a go routine for the concurrency.
func listen(connection *net.UDPConn, errors chan error) {
	buf := make([]byte, 1024)
	for {
		n, addr, err := connection.ReadFromUDP(buf)

		if err != nil {
			errors <- err
			break
		}

		log.Debugf("from %s: %s", addr, string(buf[:n]))

		// add to counter
		cs.add(string(buf[:n]))
	}
}

// pushMetrics makes POST call to OpenTSDB's `/api/put` endpoint.
func pushMetrics(ctx context.Context, ticker *time.Ticker) {
	for {
		select {
		case <-ctx.Done():
			return
		case t := <-ticker.C:

			dpoints := make([]MetricOpenTSDB, 0)

			for k, v := range cs.counter {
				d := strings.Split(k, "/")
				module := strings.Join(d[:len(d)-1], "/")
				version := d[len(d)-1]

				log.Debugf("[module: %s, version: %s] %d", module, version, v)

				dpoints = append(dpoints, MetricOpenTSDB{
					Metric:    "hpc.appusage",
					Timestamp: t.Unix(),
					Value:     v,
					Tags:      map[string]string{"module": module, "version": version},
				})
			}

			reqBody, err := json.Marshal(dpoints)
			if err != nil {
				log.Errorf("%s", err)
				continue
			}

			log.Debugf("%s", reqBody)

			// POST data to OpenTSDB endpoint
			resp, err := http.Post(*optsOpenTSDBPushURL, "application/json", bytes.NewBuffer(reqBody))
			if err != nil {
				log.Errorf("%s", err)
				continue
			}
			defer resp.Body.Close()

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Errorf("%s", err)
				continue
			}

			log.Debugf(string(body))

			// reset counter
			cs.reset()
		}
	}
}
