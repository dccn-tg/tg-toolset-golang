package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"

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

	// reset counter to an empty map
	cs.counter = make(map[string]int)
}

var (
	optsVerbose        *bool
	optsNthreads       *int
	optsPushGapSeconds *int
	optsPushGatewayURL *string
	cs                 counterStore
)

func init() {

	// CLI options
	optsVerbose = flag.Bool("v", false, "print debug messages")
	optsNthreads = flag.Int("n", runtime.NumCPU(), "set number of concurrent processing threads")
	optsPushGapSeconds = flag.Int("t", 10, "set duration in seconds between two metrics push")
	optsPushGatewayURL = flag.String("p", "http://docker.dccn.nl:9091", "set duration in seconds between two metrics push")

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

	// push collected metrics for every given duration in seconds.
	ticker := time.NewTicker(time.Duration(*optsPushGapSeconds) * time.Second)
	stopped := make(chan struct{})
	go pushMetrics(ctx, ticker, stopped)

	// handle interrupt signals, e.g. Crtl-C
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		log.Infof("Received an interrupt, stopping services...")
		cancel()
	}()

	// launch UDP server
	if err := serveUDP(ctx); err != nil {
		log.Errorf("%s", err)
	}

	// only leave the main program when the push metrics is fully stopped.
	<-stopped
}

// serveUDP starts a UDP server with concurrent listeners.
func serveUDP(ctx context.Context) error {
	// setup UDP server and connector
	addr, err := net.ResolveUDPAddr("udp", ":9999")
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	// launch concurrent UDP listeners
	errors := make(chan error)
	for i := 0; i < *optsNthreads; i++ {
		go listen(conn, errors)
	}

	for {
		select {
		case <-ctx.Done():
			log.Infof("UDP service stopped")
			return nil
		case err := <-errors:
			return err
		}
	}
}

// listen is a function to handle incoming UDP connection.  It is
// intended for a go routine for the concurrency.
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

// pushMetrics sends once a while the metrics to a remote
// Prometheus push gateway.
func pushMetrics(ctx context.Context, ticker *time.Ticker, stopped chan struct{}) {

	for {
		select {
		case <-ctx.Done():
			if err := pushAppUsage(); err != nil {
				log.Errorf("cannot push app usage: %s", err)
			}
			close(stopped)
			return
		case <-ticker.C:
			if err := pushAppUsage(); err != nil {
				log.Errorf("cannot push app usage: %s", err)
			}
			// reset counter
			cs.reset()
		}
	}
}

// convert counter data to app usage metrics and push them to the push gateway.
func pushAppUsage() error {

	appUsage := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hpc_stat_module_load_counter",
			Help: "How many times an application module has been loaded.",
		},
		[]string{"module", "version"},
	)

	pusher := push.New(*optsPushGatewayURL, "hpc_app_usage")

	for k, v := range cs.counter {

		d := strings.Split(k, "/")

		log.Debugf("[module: %s, version: %s] %d", strings.Join(d[:len(d)-1], "/"), d[len(d)-1], v)

		// add to counter with module and version labels
		appUsage.WithLabelValues(
			strings.Join(d[:len(d)-1], "/"),
			d[len(d)-1],
		).Add(float64(v))
	}

	return pusher.Collector(appUsage).Push()
}
