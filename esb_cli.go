// Package main implements a CLI interface to download esbnetworks.ie smart meter's power consumption data.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/lorentz83/esb_ie/esblib"
)

var (
	// TODO: it would be nice to read the password from env variable for security reasons.
	user     = flag.String("user", "", "the esbnetworks.ie user name")
	password = flag.String("password", "", "the esbnetworks.ie password")
	mprn     = flag.String("mprn", "", "the mprn number")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "A command line tool to download smart meter's power consumption from esbnetworks.ie.\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	e, err := esblib.NewClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot create client: %v\n", err)
		os.Exit(1)
	}

	if err := e.Login(*user, *password); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot login: %v\n", err)
		os.Exit(1)
	}

	data, err := e.DownloadPowerConsumption(*mprn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot download power consumption data: %v\n", err)
		os.Exit(1)
	}

	if _, err := io.Copy(os.Stdout, bytes.NewReader(data)); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot write power consumption data: %v\n", err)
		os.Exit(1)
	}
	// The file doesn't have a newline at the end.
	fmt.Println()

}
