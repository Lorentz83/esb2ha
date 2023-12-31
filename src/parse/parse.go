// Package parse implements a parser for the ESB "Harmonised Downloadable File (HDF)".
package parse

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"strconv"
	"time"

	"github.com/lorentz83/esb2ha/ha"
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
// Sometime ESB data has holes. This function breaks the result in blocks
// which have correct half an hour increments.
// Results are ordered by timestamp at both levels.
//
// Timestamps are assumed in Europe/Dublin timezone.
// Some heuristic is done to fix the timezone during the change from
// Daylight Saving Time to Winter Time: during this shift the same
// hour happens twice in the same day.
// Without the timezone information in the source file we have to
// guess relying on the fact that timestamps are sorted.
func HDF(hdf io.Reader) ([]Result, error) {
	var res Result
	r := csv.NewReader(hdf)

	if err := validateHeader(r); err != nil {
		return nil, err
	}

	// Read data.
	for i := 1; ; i++ {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		line, err := parseLine(i, record)
		if err != nil {
			return nil, err
		}

		if i == 1 {
			res.MPRN, res.MeterSerialNumber, res.ReadTypes = line.MPRN, line.SerialNumber, wantReadType
		} else {
			if res.MPRN != line.MPRN {
				return nil, fmt.Errorf("invalid format: multiple MPRN found (%q and %q)", res.MPRN, line.MPRN)
			}
			if res.MeterSerialNumber != line.SerialNumber {
				return nil, fmt.Errorf("invalid format: multiple meter serial numbers found (%q and %q)", res.MeterSerialNumber, line.SerialNumber)
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

	//return []Result{res}, nil
	// We need to check also fixTimezone, so validation has to be the last step.
	return splitTimes(res)
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
		if n, _ := r.EndTime.Zone(); n != "GMT" {
			// Only Winter Time can be misclassified.
			// During winter Ireland is on GMT.
			// During summer Ireland is on IST == Irish Standard (Summer) Time.
			continue
		}
		ct := r.EndTime
		// If the data is in the middle of the file, only one of the following
		// checks is required.
		// Doing both checks we can fix also the time switch at the beginning or end of the file.
		switch {
		case i+2 < max: // If there is an entry one hour later.
			// And adding one hour to this entry makes it one our earlier of the next hour.
			fix := ct.Add(-time.Hour)
			if res.Reads[i+2].EndTime.Equal(ct) {
				r.EndTime = fix
				res.Reads[i] = r
			}
		case i-2 >= 0: // If there is an entry one hour earlier.
			fix := res.Reads[i-2].EndTime.Add(time.Hour)

			_, ftz := fix.Zone()
			_, rtz := ct.Zone()

			// If the fix changes timezone.
			if ftz != rtz {
				r.EndTime = fix
				res.Reads[i] = r
			}
		default:
			// I really hope we'll never have less than 3 entries.
			// But better have some log in case it happens.
			log.Println("Too little data to attempt fix timezone")
		}
	}
}

// splitTimes splits the result in chunks with half an hour increments
// to workaround ESB missing data.
func splitTimes(res Result) ([]Result, error) {
	var lastTs time.Time

	// shallow copy
	shallowCopyResult := func() Result {
		r := res
		r.Reads = nil
		return r
	}

	rr := []Result{shallowCopyResult()}
	idx := 0

	for _, r := range res.Reads {
		ts := r.EndTime
		if err := isHalfSharp(ts); err != nil {
			return nil, err
		}
		if !lastTs.IsZero() {
			switch m := ts.Sub(lastTs).Minutes(); {
			case m == 30:
			// Expected case, nothing to do.
			case m <= 0:
				return rr, fmt.Errorf("data is not sorted by time: last %v, current %v", lastTs, ts)
			default:
				// We have a gap, let's add a new block of results.
				rr = append(rr, shallowCopyResult())
				idx++
			}
		}
		rr[idx].Reads = append(rr[idx].Reads, r)
		lastTs = ts
	}
	return rr, nil
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

// isRound returns if the time is a hour round (no minutes or seconds).
func isRound(t time.Time) bool {
	return t.Round(time.Hour).Equal(t)
}

// Translate translates ESB data into Home Assistant statistics.
//
// ESB exports kW every half an hour, while Home Assistant wants
// kWh every hour.
//
// Also ESB reports the timestamp at the end of the record period
// while Home Assistant wants the start time.
//
// The input must be valid according to ESB().
func Translate(raw Result) (ha.Statistics, error) {
	ret := ha.Statistics{
		Metadata: ha.StatisticMetadata{
			HasSum:            true,
			UnitOfMeasurement: "kWh",
		},
	}

	reads := raw.Reads
	if len(reads) == 0 {
		return ret, errors.New("not enough data")
	}
	if len(reads) > 0 && isRound(reads[0].EndTime) {
		// We want to start from a half an hour.
		reads = reads[1:]
	}

	var (
		tsValidator = reads[0].EndTime.Add(-30 * time.Minute)
		sum         float64 // Accumulator
		value       float64 // The current hour value
	)

	// My physic knowledge is a little rusted. Graph looks good
	// in Home Assistant, but please let me know if there is any
	// mistake here.
	// kW is power while kWh is energy.
	// Like kW is speed, while kWh is distance.
	// So kW times hours = kWh
	for i, r := range reads {
		// Let's be sure that we get 30 minutes increments as expected.
		if min := r.EndTime.Sub(tsValidator).Minutes(); min != 30 {
			return ret, fmt.Errorf("value %d: entries should be recorded at 30 minutes increment, got %v (%v -> %v)", i, min, tsValidator, r.EndTime)
		}
		tsValidator = r.EndTime
		//log.Printf("ts %v %v", i, tsValidator)

		value += r.Value / 2.0 // Only half an hour reading.

		// The graph in Home Assistant aligns better to the graph on
		// esbnetworks.ie if we keep the start time on the center of
		// 2 values.
		if i%2 == 0 && i > 0 {
			sum += value
			ret.Stats = append(ret.Stats, ha.StatisticValue{
				Start: reads[i-1].EndTime,
				State: value,
				Sum:   sum,
			})
			value = 0
		}
	}

	return ret, nil
}
