// Package parse implements a parser for the ESB "Harmonised Downloadable File (HDF)".
package parse

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"time"
)

const wantReadType = "Active Import Interval (kW)"

var (
	header = []string{"MPRN", "Meter Serial Number", "Read Value", "Read Type", "Read Date and End Time"}
	tz     *time.Location
)

func init() {
	location, err := time.LoadLocation("Europe/Dublin")
	if err != nil {
		panic(err)
	}
	tz = location
}

// Result is the parsed HDF file.
type Result struct {
	MPRN              string
	MeterSerialNumber string
	ReadTypes         string
	Reads             []Read
}

// Read is a single line of the HDF.
type Read struct {
	Value   float64
	EndTime time.Time
}

// HDF parses a HDF file.
//
// It may return partial results in case of error.
// Timestamps are assumed in Europe/Dublin timezone.
func HDF(hdf io.Reader) (Result, error) {
	wantLen := len(header)

	var res Result
	r := csv.NewReader(hdf)

	// Validate header.
	h, err := r.Read()
	if err != nil {
		return res, fmt.Errorf("invalid format: cannot read header: %w", err)
	}
	if got := len(h); got != wantLen {
		return res, fmt.Errorf("invalid format: header is %d long, want %d", got, wantLen)
	}
	for i, want := range header {
		if got := h[i]; got != want {
			return res, fmt.Errorf("invalid format: record %d in header is %q, want %q", i, got, want)
		}
	}

	// Read data.
	for i := 1; ; i++ {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return res, err
		}
		if got := len(record); got != wantLen {
			return res, fmt.Errorf("invalid format: got line with %d records, want %d", got, wantLen)
		}
		mprn, sn, sval, rt, sts := record[0], record[1], record[2], record[3], record[4]

		if rt != wantReadType {
			return res, fmt.Errorf("invalid format: got read type %q, want %q", rt, wantReadType)
		}

		if i == 1 {
			res.MPRN, res.MeterSerialNumber, res.ReadTypes = mprn, sn, rt
		} else {
			if res.MPRN != mprn {
				return res, fmt.Errorf("invalid format: multiple MPRN found")
			}
			if res.MeterSerialNumber != sn {
				return res, fmt.Errorf("invalid format: multiple meter serial numbers found")
			}
		}

		val, err := strconv.ParseFloat(sval, 64)
		if err != nil {
			return res, fmt.Errorf("invalid format: cannot parse line %d: %w", i, err)
		}
		ts, err := time.ParseInLocation("02-01-2006 15:04", sts, tz)
		if err != nil {
			return res, fmt.Errorf("invalid format: cannot parse line %d: %w", i, err)
		}
		res.Reads = append(res.Reads, Read{
			Value:   val,
			EndTime: ts,
		})
	}

	return res, nil
}
