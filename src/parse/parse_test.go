package parse

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

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
123,45,0.194000,Active Import Interval (kW),15-01-2023 23:30
321,45,0.157000,Active Import Interval (kW),15-01-2023 23:00`,
		},
		{
			"Different serial number",
			`MPRN,Meter Serial Number,Read Value,Read Type,Read Date and End Time
123,45,0.194000,Active Import Interval (kW),15-01-2023 23:30
123,00,0.157000,Active Import Interval (kW),15-01-2023 23:00`,
		},
		{
			"wrong order",
			`MPRN,Meter Serial Number,Read Value,Read Type,Read Date and End Time
123,45,0.157000,Active Import Interval (kW),15-01-2023 23:00
123,45,0.194000,Active Import Interval (kW),15-01-2023 23:30`,
		},
		{
			"not aligned",
			`MPRN,Meter Serial Number,Read Value,Read Type,Read Date and End Time
123,45,0.194000,Active Import Interval (kW),15-01-2023 23:33`,
		},
		// 		{
		// 			"",
		// 			`MPRN,Meter Serial Number,Read Value,Read Type,Read Date and End Time
		// 123,45,0.194000,Active Import Interval (kW),15-01-2023 23:30
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

func TestHDF(t *testing.T) {
	tests := []struct {
		name string
		data string
		want []Result
	}{
		{
			name: "good data",
			data: `MPRN,Meter Serial Number,Read Value,Read Type,Read Date and End Time
123,45,0.194000,Active Import Interval (kW),15-01-2023 23:30
123,45,0.157000,Active Import Interval (kW),15-01-2023 23:00
123,45,0.111000,Active Import Interval (kW),15-01-2023 22:30`,
			want: []Result{{
				MPRN:              "123",
				MeterSerialNumber: "45",
				ReadTypes:         "Active Import Interval (kW)",
				Reads: []Read{
					{
						Value:   0.111000,
						EndTime: time.Date(2023, 01, 15, 22, 30, 0, 0, time.FixedZone("GMT", 0)),
					},
					{
						Value:   0.157000,
						EndTime: time.Date(2023, 01, 15, 23, 00, 0, 0, time.FixedZone("GMT", 0)),
					},
					{
						Value:   0.194000,
						EndTime: time.Date(2023, 01, 15, 23, 30, 0, 0, time.FixedZone("GMT", 0)),
					},
				},
			}},
		},
		{
			name: "holes in data",
			data: `MPRN,Meter Serial Number,Read Value,Read Type,Read Date and End Time
123,45,0.6,Active Import Interval (kW),20-01-2023 02:30
123,45,0.5,Active Import Interval (kW),20-01-2023 00:30
123,45,0.4,Active Import Interval (kW),20-01-2023 00:00
123,45,0.3,Active Import Interval (kW),15-01-2023 23:30
123,45,0.2,Active Import Interval (kW),15-01-2023 23:00
123,45,0.1,Active Import Interval (kW),15-01-2023 22:30`,
			want: []Result{
				{
					MPRN:              "123",
					MeterSerialNumber: "45",
					ReadTypes:         "Active Import Interval (kW)",
					Reads: []Read{
						{
							Value:   0.1,
							EndTime: time.Date(2023, 01, 15, 22, 30, 0, 0, time.FixedZone("GMT", 0)),
						},
						{
							Value:   0.2,
							EndTime: time.Date(2023, 01, 15, 23, 00, 0, 0, time.FixedZone("GMT", 0)),
						},
						{
							Value:   0.3,
							EndTime: time.Date(2023, 01, 15, 23, 30, 0, 0, time.FixedZone("GMT", 0)),
						},
					},
				},
				{
					MPRN:              "123",
					MeterSerialNumber: "45",
					ReadTypes:         "Active Import Interval (kW)",
					Reads: []Read{
						{
							Value:   0.4,
							EndTime: time.Date(2023, 01, 20, 0, 0, 0, 0, time.FixedZone("GMT", 0)),
						},
						{
							Value:   0.5,
							EndTime: time.Date(2023, 01, 20, 0, 30, 0, 0, time.FixedZone("GMT", 0)),
						},
					},
				},
				{
					MPRN:              "123",
					MeterSerialNumber: "45",
					ReadTypes:         "Active Import Interval (kW)",
					Reads: []Read{
						{
							Value:   0.6,
							EndTime: time.Date(2023, 01, 20, 2, 30, 0, 0, time.FixedZone("GMT", 0)),
						},
					},
				},
			},
		},
		{
			"Entering Daylight Saving time",
			`MPRN,Meter Serial Number,Read Value,Read Type,Read Date and End Time
123,45,0.1,Active Import Interval (kW),26-03-2023 02:30
123,45,0.1,Active Import Interval (kW),26-03-2023 02:00
123,45,0.1,Active Import Interval (kW),26-03-2023 00:30
123,45,0.1,Active Import Interval (kW),26-03-2023 00:00`,
			[]Result{
				{
					MPRN:              "123",
					MeterSerialNumber: "45",
					ReadTypes:         "Active Import Interval (kW)",
					Reads: []Read{
						{
							Value:   0.1,
							EndTime: time.Date(2023, 03, 26, 00, 00, 0, 0, time.FixedZone("GMT", 0)), // Irish Winter Time.
						},
						{
							Value:   0.1,
							EndTime: time.Date(2023, 03, 26, 00, 30, 0, 0, time.FixedZone("GMT", 0)), // Irish Winter Time.
						},
						{
							Value:   0.1,
							EndTime: time.Date(2023, 03, 26, 02, 00, 0, 0, time.FixedZone("IST", 60*60)), // Irish Summer Time.
						},
						{
							Value:   0.1,
							EndTime: time.Date(2023, 03, 26, 02, 30, 0, 0, time.FixedZone("IST", 60*60)), // Irish Summer Time.
						},
					},
				},
			},
		},
		{
			// TODO: verify this is what we get at the end of Summer Time.
			"Leaving Daylight Saving time",
			`MPRN,Meter Serial Number,Read Value,Read Type,Read Date and End Time
123,45,0.1,Active Import Interval (kW),29-10-2023 01:00
123,45,0.1,Active Import Interval (kW),29-10-2023 00:30
123,45,0.1,Active Import Interval (kW),29-10-2023 00:00
123,45,0.1,Active Import Interval (kW),29-10-2023 00:30`,
			[]Result{
				{
					MPRN:              "123",
					MeterSerialNumber: "45",
					ReadTypes:         "Active Import Interval (kW)",
					Reads: []Read{
						// This is actual Summer Time.
						{
							Value:   0.1,
							EndTime: time.Date(2023, 10, 29, 00, 30, 0, 0, time.FixedZone("IST", 60*60)),
						},
						// This is misclassified as Summer Time, we fix it as Winter Time.
						{
							Value:   0.1,
							EndTime: time.Date(2023, 10, 29, 00, 00, 0, 0, time.FixedZone("GMT", 0)),
						},
						{
							Value:   0.1,
							EndTime: time.Date(2023, 10, 29, 00, 30, 0, 0, time.FixedZone("GMT", 0)),
						},
						// Winter Time.
						{
							Value:   0.1,
							EndTime: time.Date(2023, 10, 29, 01, 00, 0, 0, time.FixedZone("GMT", 0)),
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		got, err := HDF(strings.NewReader(tt.data))
		if err != nil {
			t.Errorf("HDF(%q) unexpected error: %v", tt.name, err)
		} else if diff := cmp.Diff(tt.want, got); diff != "" {
			t.Errorf("HDF(%q) unexpected diff (+got -want): %v", tt.name, diff)
		}
	}
}
