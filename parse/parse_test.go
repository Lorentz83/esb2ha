package parse

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestHDF(t *testing.T) {
	const data = `MPRN,Meter Serial Number,Read Value,Read Type,Read Date and End Time
123,45,0.194000,Active Import Interval (kW),15-07-2023 23:30
123,45,0.157000,Active Import Interval (kW),15-01-2023 23:00`

	want := Result{
		MPRN:              "123",
		MeterSerialNumber: "45",
		ReadTypes:         "Active Import Interval (kW)",
		Reads: []Read{
			{
				Value:   0.194000,
				EndTime: time.Date(2023, 07, 15, 23, 30, 0, 0, time.FixedZone("IST", 60*60)), // Irish Summer Time.
			},
			{
				Value:   0.157000,
				EndTime: time.Date(2023, 01, 15, 23, 0, 0, 0, time.FixedZone("GMT", 0)), // Winter time in Dublin.
			},
		},
	}

	got, err := HDF(strings.NewReader(data))
	if err != nil {
		t.Errorf("HDF() unexpected error: %v", err)
	} else if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("HDF() unexpected diff %+v", diff)
	}
}

func TestHDF_Errors(t *testing.T) {

	tests := []struct {
		name string
		data string
	}{
		{
			"empty file",
			"",
		},
		{
			"invalid timestamp",
			`MPRN,Meter Serial Number,Read Value,Read Type,Read Date and End Time
			123,45,0.194000,Active Import Interval (kW),15-07-2023 99:99`,
		},
		{
			"invalid value",
			`MPRN,Meter Serial Number,Read Value,Read Type,Read Date and End Time
123,45,NO,Active Import Interval (kW),15-07-2023 23:30`,
		},
		{
			"long header",
			`MPRN,Meter Serial Number,Read Value,Read Type,Read Date and End Time,EXTRA
123,45,0.194000,Active Import Interval (kW),15-07-2023 23:30`,
		},
		{
			"invalid header",
			`MPRN,Meter Serial Number,Read Value,Read Type,DIFFERENT
123,45,0.194000,Active Import Interval (kW),15-07-2023 23:30`,
		},
		{
			"invalid type",
			`MPRN,Meter Serial Number,Read Value,Read Type,Read Date and End Time
123,45,0.194000,WRONG,15-07-2023 23:30`,
		},
		{
			"missing field",
			`MPRN,Meter Serial Number,Read Value,Read Type,Read Date and End Time
123,45,0.194000,Active Import Interval (kW)`,
		},
		{
			"Different MPRN",
			`MPRN,Meter Serial Number,Read Value,Read Type,Read Date and End Time
123,45,0.194000,Active Import Interval (kW),15-07-2023 23:30
321,45,0.157000,Active Import Interval (kW),15-01-2023 23:00`,
		},
		{
			"Different serial number",
			`MPRN,Meter Serial Number,Read Value,Read Type,Read Date and End Time
123,45,0.194000,Active Import Interval (kW),15-07-2023 23:30
123,00,0.157000,Active Import Interval (kW),15-01-2023 23:00`,
		},
		// 		{
		// 			"",
		// 			`MPRN,Meter Serial Number,Read Value,Read Type,Read Date and End Time
		// 123,45,0.194000,Active Import Interval (kW),15-07-2023 23:30
		// 123,45,0.157000,Active Import Interval (kW),15-01-2023 23:00`,
		// 		},
		// 		{
		// 			"",
		// 			`MPRN,Meter Serial Number,Read Value,Read Type,Read Date and End Time
		// 123,45,0.194000,Active Import Interval (kW),15-07-2023 23:30`,
		// 		},
	}

	for _, tt := range tests {
		got, err := HDF(strings.NewReader(tt.data))
		if err == nil {
			t.Errorf("HDF(%q) = %+v\nwant error", tt.name, got)
		}
	}
}
