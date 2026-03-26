package main

import (
	"errors"
	"testing"
)

func TestParseSeconds(t *testing.T) {
	testCases := []struct {
		testName string
		seconds  string
		want     int
		err      error
	}{
		{
			"happy path",
			"1",
			1,
			nil,
		},
		{
			"negative number",
			"-1",
			0,
			errors.New(`invalid seconds value "-1": must be between 0 and 59`),
		},
		{
			"invalid number",
			"60",
			0,
			errors.New(`invalid seconds value "60": must be between 0 and 59`),
		},
		{
			"invalid number 2",
			"asdf234",
			0,
			errors.New(`invalid seconds value "asdf234": must be between 0 and 59`),
		},
		{
			"zero",
			"0",
			0,
			nil,
		},
		{
			"upper bound",
			"59",
			59,
			nil,
		},
		{
			"empty string",
			"",
			0,
			errors.New(`invalid seconds value "": must be between 0 and 59`),
		},
		{
			"spaces are not trimmed",
			" 1 ",
			0,
			errors.New(`invalid seconds value " 1 ": must be between 0 and 59`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			got, err := parseSeconds(tc.seconds)
			if tc.err != nil && err == nil {
				t.Fatalf("got nil, want %q", tc.err.Error())
			}
			if tc.err != nil && err != nil && tc.err.Error() != err.Error() {
				t.Fatalf("got %q, want %q", err.Error(), tc.err.Error())
			}
			if got != tc.want {
				t.Fatalf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestParseMinutes(t *testing.T) {
	testCases := []struct {
		testName string
		minutes  string
		want     int
		err      error
	}{
		{
			"happy path",
			"1",
			1,
			nil,
		},
		{
			"negative number",
			"-1",
			0,
			errors.New(`invalid minutes value "-1": must be between 0 and 59`),
		},
		{
			"invalid number",
			"60",
			0,
			errors.New(`invalid minutes value "60": must be between 0 and 59`),
		},
		{
			"invalid number 2",
			"asdf234",
			0,
			errors.New(`invalid minutes value "asdf234": must be between 0 and 59`),
		},
		{
			"zero",
			"0",
			0,
			nil,
		},
		{
			"upper bound",
			"59",
			59,
			nil,
		},
		{
			"empty string",
			"",
			0,
			errors.New(`invalid minutes value "": must be between 0 and 59`),
		},
		{
			"spaces are not trimmed",
			" 1 ",
			0,
			errors.New(`invalid minutes value " 1 ": must be between 0 and 59`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			got, err := parseMinutes(tc.minutes)
			if tc.err != nil && err == nil {
				t.Fatalf("got nil, want %q", tc.err.Error())
			}
			if tc.err != nil && err != nil && tc.err.Error() != err.Error() {
				t.Fatalf("got %q, want %q", err.Error(), tc.err.Error())
			}
			if got != tc.want {
				t.Fatalf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestParseHours(t *testing.T) {
	testCases := []struct {
		testName string
		hours    string
		want     int
		err      error
	}{
		{
			"happy path",
			"1",
			1,
			nil,
		},
		{
			"negative number",
			"-1",
			0,
			errors.New(`invalid hours value "-1": must be between 0 and 11`),
		},
		{
			"invalid number",
			"60",
			0,
			errors.New(`invalid hours value "60": must be between 0 and 11`),
		},
		{
			"invalid number 2",
			"asdf234",
			0,
			errors.New(`invalid hours value "asdf234": must be between 0 and 11`),
		},
		{
			"zero",
			"0",
			0,
			nil,
		},
		{
			"upper bound",
			"11",
			11,
			nil,
		},
		{
			"out of range upper bound",
			"12",
			0,
			errors.New(`invalid hours value "12": must be between 0 and 11`),
		},
		{
			"empty string",
			"",
			0,
			errors.New(`invalid hours value "": must be between 0 and 11`),
		},
		{
			"spaces are not trimmed",
			" 1 ",
			0,
			errors.New(`invalid hours value " 1 ": must be between 0 and 11`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			got, err := parseHours(tc.hours)
			if tc.err != nil && err == nil {
				t.Fatalf("got nil, want %q", tc.err.Error())
			}
			if tc.err != nil && err != nil && tc.err.Error() != err.Error() {
				t.Fatalf("got %q, want %q", err.Error(), tc.err.Error())
			}
			if got != tc.want {
				t.Fatalf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestTimestampToSeconds(t *testing.T) {
	testCases := []struct {
		testName  string
		timestamp string
		want      int
		err       error
	}{
		{
			"test case 1",
			"start",
			StartSecond,
			nil,
		},
		{
			"test case 2",
			"end",
			EndSecond,
			nil,
		},
		{
			"test case 3",
			"0:1",
			1,
			nil,
		},
		{
			"test case 4",
			"1:1",
			61,
			nil,
		},
		{
			"test case 5",
			"0:01",
			1,
			nil,
		},
		{
			"test case 6",
			"00:01",
			1,
			nil,
		},
		{
			"test case 7",
			"1:01",
			61,
			nil,
		},
		{
			"test case 8",
			"01:0",
			60,
			nil,
		},
		{
			"test case 9",
			"00:00:00",
			0,
			nil,
		},
		{
			"test case 10",
			"00:00:01",
			1,
			nil,
		},
		{
			"test case 11",
			"00:01:01",
			61,
			nil,
		},
		{
			"test case 12",
			"01:01:01",
			3661,
			nil,
		},
		{
			"test case 13",
			"0:0:0",
			0,
			nil,
		},
		{
			"test case 14",
			"1:0:0",
			3600,
			nil,
		},
		{
			"test case 15",
			"00:1:0",
			60,
			nil,
		},
		{
			"test case 16",
			":",
			0,
			errors.New(`invalid seconds value "": must be between 0 and 59`),
		},
		{
			"test case 17",
			"::",
			0,
			errors.New(`invalid seconds value "": must be between 0 and 59`),
		},
		{
			"test case 18",
			"61:12",
			0,
			errors.New(`invalid minutes value "61": must be between 0 and 59`),
		},
		{
			"test case 19",
			"dulce",
			0,
			errors.New(`invalid timestamp "dulce": expected HH:MM:SS, MM:SS, start, or end`),
		},
		{
			"test case 20",
			"invalidhours:00:00",
			0,
			errors.New(`invalid hours value "invalidhours": must be between 0 and 11`),
		},
		{
			"test case 21",
			"00:invalidminutes:00",
			0,
			errors.New(`invalid minutes value "invalidminutes": must be between 0 and 59`),
		},
		{
			"test case 22",
			"00:00:invalidseconds",
			0,
			errors.New(`invalid seconds value "invalidseconds": must be between 0 and 59`),
		},
		{
			"test case 23",
			"00:00:00:00",
			0,
			errors.New(`invalid timestamp "00:00:00:00": expected HH:MM:SS, MM:SS, start, or end`),
		},
		{
			"mm:ss upper bound",
			"59:59",
			3599,
			nil,
		},
		{
			"hh:mm:ss upper bound",
			"11:59:59",
			43199,
			nil,
		},
		{
			"hours out of range",
			"12:00:00",
			0,
			errors.New(`invalid hours value "12": must be between 0 and 11`),
		},
		{
			"invalid minutes in mm:ss",
			"00:60",
			0,
			errors.New(`invalid seconds value "60": must be between 0 and 59`),
		},
		{
			"invalid seconds in hh:mm:ss",
			"00:00:60",
			0,
			errors.New(`invalid seconds value "60": must be between 0 and 59`),
		},
		{
			"invalid start token with suffix",
			"start:00",
			0,
			errors.New(`invalid minutes value "start": must be between 0 and 59`),
		},
		{
			"invalid end token with suffix",
			"end:00",
			0,
			errors.New(`invalid minutes value "end": must be between 0 and 59`),
		},
		{
			"empty string",
			"",
			0,
			errors.New(`invalid timestamp "": expected HH:MM:SS, MM:SS, start, or end`),
		},
		{
			"spaces are not trimmed",
			" start ",
			0,
			errors.New(`invalid timestamp " start ": expected HH:MM:SS, MM:SS, start, or end`),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			got, err := TimestampToSeconds(tc.timestamp)
			if tc.err != nil && err == nil {
				t.Fatalf("got nil, want %q", tc.err.Error())
			}
			if tc.err != nil && err != nil && tc.err.Error() != err.Error() {
				t.Fatalf("got %q, want %q", err.Error(), tc.err.Error())
			}
			if got != tc.want {
				t.Fatalf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestSecondsToTimestamp(t *testing.T) {
	testCases := []struct {
		testName string
		seconds  int
		want     string
	}{
		{
			"test case 1",
			0,
			"00:00",
		},
		{
			"test case 2",
			1,
			"00:01",
		},
		{
			"test case 3",
			60,
			"01:00",
		},
		{
			"test case 4",
			3661,
			"01:01:01",
		},
		{
			"upper bound mm:ss",
			59,
			"00:59",
		},
		{
			"largest mm:ss before hours",
			3599,
			"59:59",
		},
		{
			"hour boundary",
			3600,
			"01:00:00",
		},
		{
			"upper bound supported hh:mm:ss",
			43199,
			"11:59:59",
		},
		{
			"negative seconds",
			-1,
			"",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			got := SecondsToTimestamp(tc.seconds)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseTimestampRangeParts(t *testing.T) {
	testCases := []struct {
		testName        string
		parts           []string
		wantStartSecond int
		wantEndSecond   int
		err             error
	}{
		{
			"happy path mm:ss",
			[]string{"00:00", "00:05"},
			0,
			5,
			nil,
		},
		{
			"happy path with start",
			[]string{"start", "00:05"},
			StartSecond,
			5,
			nil,
		},
		{
			"happy path with end",
			[]string{"00:05", "end"},
			5,
			EndSecond,
			nil,
		},
		{
			"happy path start to end",
			[]string{"start", "end"},
			StartSecond,
			EndSecond,
			nil,
		},
		{
			"invalid parts length zero",
			[]string{},
			0,
			0,
			errors.New("invalid timestamp range []"),
		},
		{
			"invalid parts length one",
			[]string{"00:05"},
			0,
			0,
			errors.New("invalid timestamp range [00:05]"),
		},
		{
			"invalid parts length three",
			[]string{"00:05", "00:06", "00:07"},
			0,
			0,
			errors.New("invalid timestamp range [00:05 00:06 00:07]"),
		},
		{
			"invalid start timestamp",
			[]string{"invalid", "00:05"},
			0,
			0,
			errors.New(`invalid timestamp "invalid": expected HH:MM:SS, MM:SS, start, or end`),
		},
		{
			"invalid end timestamp",
			[]string{"00:05", "invalid"},
			0,
			0,
			errors.New(`invalid timestamp "invalid": expected HH:MM:SS, MM:SS, start, or end`),
		},
		{
			"same start and end",
			[]string{"00:05", "00:05"},
			0,
			0,
			errors.New("start timestamp must be before end timestamp"),
		},
		{
			"start after end",
			[]string{"00:06", "00:05"},
			0,
			0,
			errors.New("start timestamp must be before end timestamp"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			gotStartSecond, gotEndSecond, err := parseTimestampRangeParts(tc.parts)
			if tc.err != nil && err == nil {
				t.Fatalf("got nil, want %q", tc.err.Error())
			}
			if tc.err != nil && err != nil && tc.err.Error() != err.Error() {
				t.Fatalf("got %q, want %q", err.Error(), tc.err.Error())
			}
			if gotStartSecond != tc.wantStartSecond {
				t.Fatalf("got %d, want %d", gotStartSecond, tc.wantStartSecond)
			}
			if gotEndSecond != tc.wantEndSecond {
				t.Fatalf("got %d, want %d", gotEndSecond, tc.wantEndSecond)
			}
		})
	}
}

func TestTimestampRangeToSeconds(t *testing.T) {
	testCases := []struct {
		testName        string
		timestampRange  string
		wantStartSecond int
		wantEndSecond   int
		err             error
	}{
		{
			"test case 1",
			"00:00-00:05",
			0,
			5,
			nil,
		},
		{
			"start to timestamp",
			"start-00:05",
			0,
			5,
			nil,
		},
		{
			"timestamp to end",
			"00:05-end",
			5,
			EndSecond,
			nil,
		},
		{
			"start to end",
			"start-end",
			StartSecond,
			EndSecond,
			nil,
		},
		{
			"hh:mm:ss range",
			"01:00:00-01:00:05",
			3600,
			3605,
			nil,
		},
		{
			"same start and end",
			"00:05-00:05",
			0,
			0,
			errors.New("start timestamp must be before end timestamp"),
		},
		{
			"start after end",
			"00:06-00:05",
			0,
			0,
			errors.New("start timestamp must be before end timestamp"),
		},
		{
			"invalid start token",
			"end-00:05",
			0,
			0,
			errors.New(`invalid timestamp range "end-00:05"`),
		},
		{
			"invalid end token",
			"00:05-start",
			0,
			0,
			errors.New(`invalid timestamp range "00:05-start"`),
		},
		{
			"invalid start value",
			"invalid-00:05",
			0,
			0,
			errors.New(`invalid timestamp range "invalid-00:05"`),
		},
		{
			"invalid end value",
			"00:05-invalid",
			0,
			0,
			errors.New(`invalid timestamp range "00:05-invalid"`),
		},
		{
			"empty string",
			"",
			0,
			0,
			errors.New(`invalid timestamp range ""`),
		},
		{
			"missing separator",
			"00:05",
			0,
			0,
			errors.New(`invalid timestamp range "00:05"`),
		},
		{
			"too many separators",
			"00:05-00:06-00:07",
			0,
			0,
			errors.New(`invalid timestamp range "00:05-00:06-00:07"`),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			gotStartSecond, gotEndSecond, err := TimestampRangeToSeconds(tc.timestampRange)
			if tc.err != nil && err == nil {
				t.Fatalf("got nil, want %q", tc.err.Error())
			}
			if tc.err != nil && err != nil && tc.err.Error() != err.Error() {
				t.Fatalf("got %q, want %q", err.Error(), tc.err.Error())
			}
			if gotStartSecond != tc.wantStartSecond {
				t.Fatalf("got %d, want %d", gotStartSecond, tc.wantStartSecond)
			}
			if gotEndSecond != tc.wantEndSecond {
				t.Fatalf("got %d, want %d", gotEndSecond, tc.wantEndSecond)
			}
		})
	}
}
