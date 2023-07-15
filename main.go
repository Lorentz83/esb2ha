package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/lorentz83/esb_ie/esblib"
)

var (
	user     = flag.String("user", "", "esbnetworks.ie user name")
	password = flag.String("password", "", "esbnetworks.ie password")
	mprn     = flag.String("mprn", "", "mprn number")
)

func main() {
	flag.Parse()

	e, err := esblib.NewClient(*user, *password, *mprn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot create client: %v\n", err)
		os.Exit(1)
	}

	if err := e.Login(); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot login: %v\n", err)
		os.Exit(1)
	}

	if err := e.Download(); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot download data: %v\n", err)
		os.Exit(1)
	}

}
