// Package parse implements a parser for the ESB "Harmonised Downloadable File (HDF)".
package parse

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"strconv"
	"time"
)

const wantReadType = "Active Import Interval (kW)"

var (
	headerFormat      = []string{"MPRN", "Meter Serial Number", "Read Value", "Read Type", "Read Date and End Time"}
	irelandTimezone   *time.Location
	irelandWinterTime *time.Location
)

func init() {
	location, err := time.LoadLocation("Europe/Dublin")
	if err != nil {
		panic(err)
	}
	irelandTimezone = location

	irelandWinterTime = time.FixedZone("GMT", 0)
}

type line struct {
	MPRN         string
	SerialNumber string
	Value        float64
	EndTime      time.Time
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

// HDF parses a HDF file and returns the result in ascending timestamps.
//
// Timestamps are assumed in Europe/Dublin timezone.
// Some heuristic is done to fix the timezone during the change from
// Daylight Saving Time to Winter Time: during this shift the same
// hour happens twice in the same day.
// Without the timezone information in the source file we have to
// guess relying on the fact that timestamps are sorted.
func HDF(hdf io.Reader) (Result, error) {
	var res Result
	r := csv.NewReader(hdf)

	if err := validateHeader(r); err != nil {
		return res, err
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

		line, err := parseLine(i, record)
		if err != nil {
			return res, err
		}

		if i == 1 {
			res.MPRN, res.MeterSerialNumber, res.ReadTypes = line.MPRN, line.SerialNumber, wantReadType
		} else {
			if res.MPRN != line.MPRN {
				return res, fmt.Errorf("invalid format: multiple MPRN found (%q and %q)", res.MPRN, line.MPRN)
			}
			if res.MeterSerialNumber != line.SerialNumber {
				return res, fmt.Errorf("invalid format: multiple meter serial numbers found (%q and %q)", res.MeterSerialNumber, line.SerialNumber)
			}
		}

		res.Reads = append(res.Reads, Read{
			Value:   line.Value,
			EndTime: line.EndTime,
		})
	}

	// Reverse to have ascending timestamp order.
	for i, j := 0, len(res.Reads)-1; i < j; i++ {
		res.Reads[i], res.Reads[j] = res.Reads[j], res.Reads[i]
		j--
	}

	fixTimezone(&res)

	// We need to check also fixTimezone, so validation has to be the last step.
	if err := validateTimes(res); err != nil {
		return res, err
	}

	return res, nil
}

// fixTimezone attempts to fix the timezone when moving from summer to winter time.
//
// Because there is no timezone information in the data file, when transitioning
// off Daylight Saving Time the same hour happens twice.
//
// This function implements a heuristic to fix this issue doing the following assumptions:
//   - There is an entry every 30 minutes
//   - There are at least 3 entries in the result
func fixTimezone(res *Result) {
	max := len(res.Reads)
	for i := 0; i < max; i++ {
		r := res.Reads[i]
		if n, _ := r.EndTime.Zone(); n != "IST" {
			// IST == Irish Standard (Summer) Time.
			// Only Summer Time can be misclassified.
			continue
		}
		ct := r.EndTime
		// If the data is in the middle of the file, only one of the following
		// checks is required.
		// Doing both checks we can fix also the time switch at the beginning or end of the file.
		switch {
		case i-2 >= 0: // Is the prev prev is exactly the same time.
			res.Reads[i-2].EndTime.Equal(ct)
			r.EndTime = ct.Add(time.Hour)
			res.Reads[i] = r
		case i+2 < max: // Can be followed by winter.
			// If adding one hour to this entry makes it one our earlier of the next next.
			fix := ct.Add(time.Hour)
			if res.Reads[i+2].EndTime.Sub(fix).Hours() == 1 {
				r.EndTime = fix
				res.Reads[i] = r
			}
		default:
			// It is not really true, but this code is already complex enough
			// and I don't expect we'll ever have less than 3 entries.
			log.Println("Too little data to attempt fix timezone")
		}
	}
}

// validateTimes validates the the values are recorded every half an hour.
func validateTimes(res Result) error {
	var lastTs time.Time
	for _, r := range res.Reads {
		ts := r.EndTime
		if err := isHalfSharp(ts); err != nil {
			return err
		}
		if !lastTs.IsZero() {
			if m := ts.Sub(lastTs).Minutes(); m != 30 {
				return fmt.Errorf("time not in 30 minutes increments (%v, %v) %v", lastTs, ts, m)
			}
		}
		lastTs = ts
	}
	return nil
}

// isHalfSharp checks that the timestamp is at the hour or half an hour sharp.
func isHalfSharp(ts time.Time) error {
	m := ts.Minute()
	if ts.Second() != 0 || ts.Nanosecond() != 0 || (m != 0 && m != 30) {
		return fmt.Errorf("timestamp %v is not aligned with 30 minutes", ts)
	}
	return nil
}

func parseLine(lineNumber int, record []string) (line, error) {
	var (
		res        = line{MPRN: record[0], SerialNumber: record[1]}
		recordType = record[3]
		sval       = record[2]
		sts        = record[4]
		err        error
	)
	if recordType != wantReadType {
		return res, fmt.Errorf("invalid format: on line %d got read type %q, want %q", lineNumber, recordType, wantReadType)
	}
	res.Value, err = strconv.ParseFloat(sval, 64)
	if err != nil {
		return res, fmt.Errorf("invalid format: cannot parse line %d: %w", lineNumber, err)
	}
	res.EndTime, err = time.ParseInLocation("02-01-2006 15:04", sts, irelandTimezone)
	if err != nil {
		return res, fmt.Errorf("invalid format: cannot parse line %d: %w", lineNumber, err)
	}
	return res, nil
}

func validateHeader(r *csv.Reader) error {
	h, err := r.Read()
	if err != nil {
		return fmt.Errorf("invalid format: cannot read header: %w", err)
	}
	wantLen := len(headerFormat)
	if got := len(h); got != wantLen {
		return fmt.Errorf("invalid format: header is %d long, want %d", got, wantLen)
	}
	for i, want := range headerFormat {
		if got := h[i]; got != want {
			return fmt.Errorf("invalid format: record %d in header is %q, want %q", i, got, want)
		}
	}
	return nil
}
