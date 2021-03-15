package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"runtime"
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

	// reset counter to an empty map
	cs.counter = make(map[string]int)
}

var (
	optsVerbose        *bool
	optsNthreads       *int
	optsPushGapSeconds *int
	cs                 counterStore
)

func init() {

	// CLI options
	optsVerbose = flag.Bool("v", false, "print debug messages")
	optsNthreads = flag.Int("n", runtime.NumCPU(), "set number of concurrent processing threads")
	optsPushGapSeconds = flag.Int("t", 10, "set duration in seconds between two metrics push")

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
	stop := make(chan bool)
	stopped := make(chan bool)
	go pushMetrics(ticker, stop, stopped)

	// handle interrupt signals, e.g. Crtl-C
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		log.Infof("Received an interrupt, stopping services...")
		cancel()

		// notify pushMetrics to stop
		stop <- true
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
func pushMetrics(ticker *time.Ticker, stop chan bool, stopped chan bool) {
	for {
		select {
		case <-stop:
			// make last push before quit
			for k, v := range cs.counter {
				log.Infof("%s: %d", k, v)
			}
			close(stopped)
			return
		case <-ticker.C:
			for k, v := range cs.counter {
				log.Infof("%s: %d", k, v)
			}
			// reset counter
			cs.reset()
		}
	}
}
