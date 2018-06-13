package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"

	ustr "dccn.nl/utility/strings"
	log "github.com/sirupsen/logrus"
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
	fmt.Printf("\nDecrypting base64 encrypted password with a provided encryption key.\n")
	fmt.Printf("\nUSAGE: %s [OPTIONS] <encryptedpass>\n", os.Args[0])
	fmt.Printf("\nOPTIONS:\n")
	flag.PrintDefaults()
	fmt.Printf("\n")
}

func main() {

	key := []byte(*optsKey)

	encryptedPass, err := hex.DecodeString(flag.Args()[0])
	if err != nil {
		panic(fmt.Sprintf("cannot decode key, reason: %+v", err))
	}
	pass, err := ustr.Decrypt(encryptedPass, key)
	if err != nil {
		panic(fmt.Sprintf("cannot decrypt password, reason: %+v", err))
	}
	fmt.Printf("pass:%s\n", string(pass))
}
