package repocli

import (
	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
	pb "github.com/schollz/progressbar/v3"
)

type Progress struct {
	bar       *pb.ProgressBar
	total     chan int
	processed chan struct{}
	stop      chan struct{}
}

// Start a progress bar.  It returns two channels.
//
// The first one is to increase the bar's max by the value pushed to the channel.
// The second is to signal the progress bar to increase the processed items by one.
func (p *Progress) Start(silent bool) (chan int, chan struct{}) {

	p.total = make(chan int)
	p.processed = make(chan struct{})
	p.stop = make(chan struct{})

	go func() {

		defer func() {
			close(p.total)
			close(p.processed)
			close(p.stop)
		}()

		if silent {
			p.bar = pb.DefaultSilent(1)
		} else {
			p.bar = pb.Default(1)
		}
		bmax := 0
	ploop:
		for {
			log.Debugf("test")
			select {
			case it := <-p.total:
				bmax = bmax + it
				p.bar.ChangeMax(bmax)
			case <-p.processed:
				p.bar.Add(1)
			case <-p.stop:
				log.Debugf("stopped")
				break ploop
			}
		}
	}()

	return p.total, p.processed
}

// Stop the progress bar
func (p *Progress) Stop() {
	log.Debugf("want to stop")
	p.stop <- struct{}{}
}
