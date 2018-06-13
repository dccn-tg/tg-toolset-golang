package main

import (
	"flag"
	"fmt"
	"os"
	"syscall"

	ustr "dccn.nl/utility/strings"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	optsKey     *string
	optsVerbose *bool
)

func init() {
	optsKey = flag.String("k", "the-key-has-to-be-32-bytes-long!", "set the encryption key with min. 32 characters")
	optsVerbose = flag.Bool("v", false, "print debug messages")

	flag.Usage = usage

	flag.Parse()

	// set logging
	log.SetOutput(os.Stderr)
	// set logging level
	llevel := log.InfoLevel
	if *optsVerbose {
		llevel = log.DebugLevel
	}
	log.SetLevel(llevel)
}

func usage() {
	fmt.Printf("\nEncrypting password to base64 string with a provided encryption key.\n")
	fmt.Printf("\nUSAGE: %s [OPTIONS]\n", os.Args[0])
	fmt.Printf("\nOPTIONS:\n")
	flag.PrintDefaults()
	fmt.Printf("\n")
}

func main() {

	key := []byte(*optsKey)

	// CLI prompt to readin password from terminal
	fmt.Print("Enter Password: ")

	var bytePassword []byte
	var err error
	for {
		bytePassword, err = terminal.ReadPassword(int(syscall.Stdin))
		if err != nil {
			panic(fmt.Sprintf("cannot read password, reason: %+v", err))
		}
		log.Debugf("plain password: %s", string(bytePassword))
		if len(bytePassword) == 0 {
			log.Errorf("password cannot be empty string.  Try again.")
			continue
		}
		break
	}

	// encrypt value to base64
	cryptoText, err := ustr.Encrypt(bytePassword, key)
	if err != nil {
		panic(fmt.Sprintf("cannot encrypt password, reason: %+v", err))
	}
	fmt.Printf("\npass:%x\tkey:%s\n", cryptoText, *optsKey)
}
