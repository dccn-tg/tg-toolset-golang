package main

import (
	"context"
	"net"
	"runtime"
	"sync"
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
	cs counterStore
)

func init() {

}

func main() {

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	// push collected metrics in the backgroun every 10 seconds.
	ticker := time.NewTicker(10 * time.Second)
	stopPusher := make(chan bool)
	go pushMetrics(ticker, stopPusher)
	// stopping metrics pushing and exits the program.
	defer func() {
		stopPusher <- true
		cancel()
	}()

	// launch UDP server
	if err := serveUDP(ctx); err != nil {
		log.Fatalf("%s", err)
	}
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
	for i := 0; i < runtime.NumCPU(); i++ {
		go listen(conn, errors)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-errors:
			return err
		}
	}
}

// listen is a function to handle incoming UDP connection.  It is
// intended for a go routine for the concurrency.
func listen(connection *net.UDPConn, errors chan error) {
	buffer := make([]byte, 1024)
	n, remoteAddr, err := 0, new(net.UDPAddr), error(nil)
	for err == nil {
		n, remoteAddr, err = connection.ReadFromUDP(buffer)

		log.Debugf("from", remoteAddr, "-", buffer[:n])

		// add to counter
		cs.add(string(buffer[:n]))
	}
	errors <- err
}

// pushMetrics sends once a while the metrics to a remote
// Prometheus push gateway.
func pushMetrics(ticker *time.Ticker, quit chan bool) {
	for {
		select {
		case <-quit:
			// make last push before quit
			for k, v := range cs.counter {
				log.Infof("[before exit] %s: %n", k, v)
			}
			return
		case t := <-ticker.C:
			for k, v := range cs.counter {
				log.Infof("[%s] %s: %n", t, k, v)
			}
		}
	}
}
