package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/google/subcommands"
	"github.com/lorentz83/esb2ha/esblib"
	"github.com/lorentz83/esb2ha/ha"
	"github.com/lorentz83/esb2ha/parse"
)

func init() {
	subcommands.Register(&downloadCmd{}, "")
	subcommands.Register(&uploadCmd{}, "")
	subcommands.Register(&pipeCmd{}, "")
	subcommands.Register(subcommands.HelpCommand(), "")
	baseExplain := subcommands.DefaultCommander.Explain
	subcommands.DefaultCommander.Explain = func(w io.Writer) {
		fmt.Fprint(w, "Download electricity usage data from ESB and (optionally) upload it to Home Assistant.\n\n")
		baseExplain(w)
		fmt.Fprintln(w, "Copyright 2023 Lorenzo Bossi.\nThis tool is free software, fore more information https://github.com/Lorentz83/esb2ha/")
	}
}

func main() {
	flag.Parse()
	s := subcommands.Execute(context.Background())
	os.Exit(int(s))
}

// flagsFromEnv sets the unset flags from environment variables with the same name.
func flagsFromEnv(f *flag.FlagSet) {
	f.VisitAll(func(f *flag.Flag) {
		// If the flag is not provided.
		if f.Value.String() == "" {
			// Set it with the value of the environment variable with the same name.
			_ = f.Value.Set(os.Getenv(f.Name)) // In case of error we move on, the error will be raised later.
		}
	})
}

// ensureFlagsAreSet checks if there are environment variables for the unset flag
// and returns an error for the missing flags.
func ensureFlagsAreSet(f *flag.FlagSet) error {
	flagsFromEnv(f)
	var missing []string
	f.VisitAll(func(f *flag.Flag) {
		if f.Value.String() == "" {
			missing = append(missing, f.Name)
		}
	})
	if len(missing) > 0 {
		return fmt.Errorf("the following flags are missing: %s", strings.Join(missing, ", "))
	}
	return nil
}

type downloadCmd struct {
	user, password, mprn string
}

func (downloadCmd) Name() string { return "download" }

func (downloadCmd) Synopsis() string {
	return "download the electricity usage data from esbnetworks.ie"
}

func (downloadCmd) Usage() string {
	return `download <flags>

All the flags are required, but can be provided as environment variables as well.
The file is printed on standard error.

`
}

func (c *downloadCmd) SetFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.user, "esb_user", "", "the user name on esbnetworks.ie")
	fs.StringVar(&c.password, "esb_password", "", "the user name on esbnetworks.ie")
	fs.StringVar(&c.mprn, "mprn", "", "the mprn number on the electricity bill")
}

func (c *downloadCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	if err := ensureFlagsAreSet(f); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err.Error())
		return subcommands.ExitUsageError
	}

	data, err := c.download()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s", err)
		return subcommands.ExitFailure
	}

	if _, err := io.Copy(os.Stdout, bytes.NewReader(data)); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Cannot write power consumption data: %v\n", err)
		return subcommands.ExitFailure
	}

	// The file doesn't have a newline at the end.
	fmt.Fprintln(os.Stdout)

	return subcommands.ExitSuccess
}

func (c *downloadCmd) download() ([]byte, error) {
	e, err := esblib.NewClient()
	if err != nil {
		return nil, fmt.Errorf("cannot connect to ESB website: %w", err)
	}

	if err := e.Login(c.user, c.password); err != nil {
		return nil, fmt.Errorf("cannot login: %w", err)
	}

	data, err := e.DownloadPowerConsumption(c.mprn)
	if err != nil {
		return nil, fmt.Errorf("cannot download power consumption data: %w", err)
	}
	return data, nil
}

type uploadCmd struct {
	server, token, sensor string
}

func (uploadCmd) Name() string { return "upload" }

func (uploadCmd) Synopsis() string {
	return "upload the electricity usage data to Home Assistant"
}

func (uploadCmd) Usage() string {
	return `upload <flags>
	
All the flags are required, but can be provided as environment variables as well.
The CSV file is read from standard input.

`
}

func (c *uploadCmd) SetFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.server, "ha_server", "", "Home Assistant server name or IP and optionally the port")
	fs.StringVar(&c.token, "ha_token", "", "Home Assistant admin authentication token")
	fs.StringVar(&c.sensor, "ha_sensor", "", "Home Assistant sensor ID used to record power usage")
}

func (c *uploadCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	if err := ensureFlagsAreSet(f); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err.Error())
		return subcommands.ExitUsageError
	}

	fmt.Println("Reading from stdin...")

	return c.parseAndUpload(ctx, os.Stdin)
}

func (c *uploadCmd) parseAndUpload(ctx context.Context, data io.Reader) subcommands.ExitStatus {
	parsed, err := parse.HDF(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		return subcommands.ExitFailure
	}
	if len(parsed) == 0 {
		fmt.Fprintf(os.Stderr, "ERROR: nothing to upload\n")
		return subcommands.ExitFailure
	}

	ret := subcommands.ExitSuccess
	for _, chunk := range parsed {
		fmt.Println("Uploading data...")
		stat, err := c.upload(ctx, chunk)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			ret = subcommands.ExitFailure
		} else {
			fmt.Printf("Sent %d data points\n", len(stat.Stats))
		}
	}
	return ret
}

func (c *uploadCmd) upload(ctx context.Context, data parse.Result) (ha.Statistics, error) {
	stat, err := parse.Translate(data)
	if err != nil {
		return stat, fmt.Errorf("cannot parse data: %w", err)
	}

	stat.Metadata.StatisticID = c.sensor

	conn, err := ha.NewConnection(ctx, c.server, c.token)
	if err != nil {
		return stat, fmt.Errorf("cannot connect to Home Assistant: %w", err)
	}

	if err := conn.SendStatistics(ctx, stat); err != nil {
		return stat, fmt.Errorf("cannot send statistics to Home Assistant: %w", err)
	}
	return stat, nil
}

type pipeCmd struct {
	ha  uploadCmd
	esb downloadCmd
}

func (pipeCmd) Name() string { return "pipe" }

func (pipeCmd) Synopsis() string {
	return "download from esb and upload to Home Assistant the electricity usage data"
}

func (pipeCmd) Usage() string {
	return `pipe  <flags>

All the flags are required, but can be provided as environment variables as well.
It is the equivalent of piping download and upload.

`
}

func (c *pipeCmd) SetFlags(fs *flag.FlagSet) {
	c.ha.SetFlags(fs)
	c.esb.SetFlags(fs)
}

func (c *pipeCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	if err := ensureFlagsAreSet(f); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err.Error())
		return subcommands.ExitUsageError
	}

	fmt.Println("Downloading data...")
	data, err := c.esb.download()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		return subcommands.ExitFailure
	}

	return c.ha.parseAndUpload(ctx, bytes.NewReader(data))
}
