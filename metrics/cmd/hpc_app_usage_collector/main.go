// hpc_app_usage_collector is a daemon for monitoring the HPC software usage by
// collecting data instrumented in the environment modules on the HPC cluster.
//
// This program contains two parts:
//
// - _Collector_ service listens on incoming data sent from the `module load` command.
//   The data is as simple as the string containing the module name and the version.
//
// - _Metrics_ service provides the `/metrics` HTTP endpoint for Prometheus server to
//   scrapes collected metrics.  Data collected by the _Collector_ is transformed into
//   Prometheus Gauge metrics every given period of time (default: 10 seconds).
//
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

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

	// reset existing counter to 0
	for k := range cs.counter {
		cs.counter[k] = 0
	}
}

var (
	optsVerbose                *bool
	optsNthreads               *int
	optsCollectPeriod          *int
	optsListenAddressCollector *string
	optsListenAddressMetrics   *string
	cs                         counterStore

	// metrics
	appUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hpc_stat_app_usage",
			Help: "How many times an application module has been loaded.",
		},
		[]string{"module", "version"},
	)
)

func init() {

	// CLI options
	optsVerbose = flag.Bool("v", false, "print debug messages")
	optsNthreads = flag.Int("n", runtime.NumCPU()-1, "set `number` of threads for collector")
	optsCollectPeriod = flag.Int("p", 10, "set `period` in seconds within which the usage is counted")
	optsListenAddressCollector = flag.String("c", ":9999", "set listener `address` for collecting data from module load")
	optsListenAddressMetrics = flag.String("m", ":9998", "set listener `address` for exporting metrics to prometheus")

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

	// register metrics
	prometheus.MustRegister(appUsage)

	// unregister metrics that are by default added to the
	// `prometheus.DefaultRegisterer`
	prometheus.Unregister(prometheus.NewGoCollector())
	prometheus.Unregister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
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

	// periodically update metrics with collected usage count
	ticker := time.NewTicker(time.Duration(*optsCollectPeriod) * time.Second)
	go updateMetrics(ticker)

	// launch HTTP server with /metrics endpoint
	go serveMetrics(ctx, cancel)

	// blocking the service
	<-ctx.Done()
}

// serveMetrics starts a HTTP server with an endpoint `/metrics` to export
// metrics to the Prometheus server.
func serveMetrics(ctx context.Context, cancel context.CancelFunc) {

	// Expose the registered metrics via HTTP.
	http.Handle("/metrics", promhttp.HandlerFor(
		prometheus.DefaultGatherer,
		promhttp.HandlerOpts{
			EnableOpenMetrics: false,
		},
	))

	// http server
	srv := &http.Server{
		Addr:    *optsListenAddressMetrics,
		Handler: nil, // using default
	}

	// start the http server.
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Errorf("cannot start Metrics service: %s", err)
		cancel()
	}
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

// updateMetrics updates the prometheus metrics periodically with
// data collected in the in-memory `counterStore` since the last update.
func updateMetrics(ticker *time.Ticker) {
	for {
		select {
		case <-ticker.C:
			for k, v := range cs.counter {

				d := strings.Split(k, "/")
				module := strings.Join(d[:len(d)-1], "/")
				version := d[len(d)-1]

				log.Debugf("[module: %s, version: %s] %d", module, version, v)

				m, err := appUsage.GetMetricWith(
					prometheus.Labels{
						"module":  module,
						"version": version,
					},
				)

				if err != nil {
					log.Errorf("[module: %s, version: %s] cannot update metrics: %s", module, version, err)
				}

				m.Set(float64(v))
			}

			// reset counter
			cs.reset()
		}
	}
}
