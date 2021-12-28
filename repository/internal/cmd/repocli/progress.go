package repocli

import (
	pb "github.com/schollz/progressbar/v3"
)

type Progress struct {
	bar       *pb.ProgressBar
	total     chan int
	processed chan struct{}
}

// Start a progress bar.  It returns two channels.
//
// The first one is to increase the bar's max by the value pushed to the channel.
// The second is to signal the progress bar to increase the processed items by one.
//
// This progress bar is intended for the case that the total steps to be processed
// is unknown at the beginning, and to be updated along the steps.  An example is
// the total number of files to be processed iteratively in a directory.
func (p *Progress) Start(silent bool) (chan int, chan struct{}) {

	p.total = make(chan int, 4)
	p.processed = make(chan struct{})

	go func() {

		if silent {
			p.bar = pb.DefaultSilent(1)
		} else {
			p.bar = pb.Default(1)
		}

		//defer p.bar.Finish()

		// set initial bar max to 1 to prevent the situation that
		// the number of processed steps is eventually equal to
		// the current total steps, which causes the bar to be
		// considered as "completed" (i.e. no further update).
		bmax := 1
	ploop:
		for {
			select {
			case it := <-p.total:
				bmax = bmax + it
				p.bar.ChangeMax(bmax)
			case _, more := <-p.processed:
				if more {
					p.bar.Add(1)
				} else {
					p.bar.ChangeMax(bmax - 1)
					break ploop
				}
			}
		}
	}()

	return p.total, p.processed
}

// Stop the progress bar and wait until the bar is finished/closed.
func (p *Progress) Stop() {

	// close the channels `processed` and `total` to stop updating the bar
	close(p.total)
	close(p.processed)

	// wait until the progressbar is really finished
	for {
		if p.bar.IsFinished() {
			break
		}
	}
}
