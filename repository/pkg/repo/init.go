// Package repo provides a library for managing the data collections
// stored in the Donders Repository.
package repo

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	log "github.com/dccn-tg/tg-toolset-golang/pkg/logger"
)

// EscapeSpecialCharsGenQuery addes "\" in front of the known special characters
// that cannot be passed to GenQuery directly.
func EscapeSpecialCharsGenQuery(p string) string {

	// note that the special characters need to be handcrafted one by one.
	// so far, the one noticed not being accepted by iRODS GenQuery is "`".
	for _, c := range []string{"`"} {
		p = strings.ReplaceAll(p, c, fmt.Sprintf("\\%s", c))
	}

	return p
}

// IcommandChanOut executes the given icommand `cmdStr` and return output
// line-by-line to the channel `out`.
//
// Set `closeOut` to `true` will cause the function to close the channel `out`
// when there is no more output from the icommand.
func IcommandChanOut(cmdStr string, out *chan string, closeOut bool) {

	log.Debugf("cmd: %s", cmdStr)

	// conditionally close the output channel before leaving the executor function.
	defer func() {
		if closeOut {
			close(*out)
		}
	}()

	cmd := exec.Command("bash", "-c", cmdStr)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Errorf("cannot pipe output: %s", err)
		return
	}

	if err = cmd.Start(); err != nil {
		log.Errorf("cannot start command: %s", err)
		return
	}

	outScanner := bufio.NewScanner(stdout)
	outScanner.Split(bufio.ScanLines)

	for outScanner.Scan() {
		// push to the channel `*out` only if the scanned text is not "CAT_NO_ROWS_FOUND".
		if l := outScanner.Text(); !strings.Contains(l, "CAT_NO_ROWS_FOUND") {
			*out <- l
		}
	}

	if err = outScanner.Err(); err != nil {
		log.Errorf("error reading output of command: %s", err)
	}

	// wait the cmd to finish and the IO pipes are closed.
	// write out error if the command execution is failed.
	if err = cmd.Wait(); err != nil {
		log.Errorf("%s fail: %s", cmdStr, err)
	}
}

// IcommandWriterOut executes the given icommand `cmdStr` and write stdout to the
// to provided writer `wout`.
func IcommandWriterOut(cmdStr string, wout *bufio.Writer) error {
	log.Debugf("cmd: %s", cmdStr)

	// execute command
	cmd := exec.Command("bash", "-c", cmdStr)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("cannot pipe output: %s", err)
	}

	if err = cmd.Start(); err != nil {
		return fmt.Errorf("cannot start command: %s", err)
	}

	outScanner := bufio.NewScanner(stdout)
	outScanner.Split(bufio.ScanLines)

	for outScanner.Scan() {
		// push to the channel `*out` only if the scanned text is not "CAT_NO_ROWS_FOUND".
		if l := outScanner.Text(); !strings.Contains(l, "CAT_NO_ROWS_FOUND") {
			fmt.Fprintln(wout, l)
		}
	}

	if err = outScanner.Err(); err != nil {
		return fmt.Errorf("error reading output of command: %s", err)
	}

	// wait the cmd to finish and the IO pipes are closed.
	// write out error if the command execution is failed.
	if err = cmd.Wait(); err != nil {
		return fmt.Errorf("%s fail: %s", cmdStr, err)
	}

	return nil
}

// IcommandFileOut executes the given icommand `cmdStr` and write stdout to the
// to provided `fstdout` filename. If the filename is empty (i.e. `""`)
// the output is discarded.
func IcommandFileOut(cmdStr, fstdout string) error {

	log.Debugf("cmd: %s", cmdStr)

	// simple command execution discarding output.
	if fstdout == "" {
		_, err := exec.Command("bash", "-c", cmdStr).Output()
		return err
	}

	// command execution saving stdout to file `fstdout`.
	fout, err := os.Create(fstdout)
	if err != nil {
		return err
	}
	defer fout.Close()

	// prepare writer
	wout := bufio.NewWriter(fout)
	defer wout.Flush()

	// execute command
	return IcommandWriterOut(cmdStr, wout)
}
