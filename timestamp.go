package main

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const StartSecond = 0
const EndSecond = -1

// ErrInvalidTimestampRange reports that a timestamp range is malformed or
// semantically invalid.
var ErrInvalidTimestampRange = errors.New("invalid timestamp range")

// TimestampRangePattern matches the structure of timestamp ranges in
// MM:SS-MM:SS or HH:MM:SS-HH:MM:SS format, such as:
//
// 1:05-1:10
// 0:10-0:51
// 17:49-end
// 21:50-58:00
// 3:17:55-4:17:59
// 41:40-1:23:00
// 52:40-end
// start-1:10
// 2:30-end
//
// Detailed validation, such as checking numeric bounds, is performed
// separately when parsing the timestamps.
//
// The optional hour component is limited to two digits because, when this
// code was originally written, YouTube documentation indicated a maximum
// video length of 12 hours.
// https://support.google.com/youtube/answer/71673
var TimestampRangePattern = regexp.MustCompile(
	`^(start|([\d]{1,2}:)?[\d]{1,2}:[\d]{1,2})-(end|([\d]{1,2}:)?[\d]{1,2}:[\d]{1,2})$`,
)

// parseSeconds parses a seconds value and validates that it is between 0 and 59.
func parseSeconds(seconds string) (int, error) {
	invalidValueErr := fmt.Errorf("invalid seconds value %q: must be between 0 and 59", seconds)
	secondsInt, err := strconv.Atoi(seconds)
	if err != nil {
		return 0, invalidValueErr
	}
	if secondsInt < 0 || secondsInt > 59 {
		return 0, invalidValueErr
	}
	return secondsInt, nil
}

// parseMinutes parses a minutes value and validates that it is between 0 and 59.
func parseMinutes(minutes string) (int, error) {
	invalidValueErr := fmt.Errorf("invalid minutes value %q: must be between 0 and 59", minutes)
	minutesInt, err := strconv.Atoi(minutes)
	if err != nil {
		return 0, invalidValueErr
	}
	if minutesInt < 0 || minutesInt > 59 {
		return 0, invalidValueErr
	}
	return minutesInt, nil
}

// parseHours parses an hours value and validates that it is between 0 and 11.
func parseHours(hours string) (int, error) {
	invalidValueErr := fmt.Errorf("invalid hours value %q: must be between 0 and 11", hours)
	hoursInt, err := strconv.Atoi(hours)
	if err != nil {
		return 0, invalidValueErr
	}
	if hoursInt < 0 || hoursInt > 11 {
		return 0, invalidValueErr
	}
	return hoursInt, nil
}

// TimestampToSeconds parses a timestamp in MM:SS or HH:MM:SS format, or the
// keywords start and end, and returns its corresponding value in seconds.
func TimestampToSeconds(timestamp string) (int, error) {
	invalidTimestampErr := fmt.Errorf("invalid timestamp %q: expected HH:MM:SS, MM:SS, start, or end", timestamp)
	parts := strings.Split(timestamp, ":")
	partsNumber := len(parts)
	switch partsNumber {
	case 1:
		// Parse keywords "start" or "end"
		// The string "start" is converted into this array: ["start"]
		// The string "end" is converted into this array: ["end"]
		var totalSeconds int
		switch parts[0] {
		case "start":
			totalSeconds = StartSecond
		case "end":
			totalSeconds = EndSecond
		default:
			return 0, invalidTimestampErr
		}
		return totalSeconds, nil
	case 2:
		// Parse minutes and seconds (MM:SS), for example:
		// The string "00:10" is converted into this array: ["00", "10"]
		totalSeconds, err := parseSeconds(parts[1])
		if err != nil {
			return 0, err
		}
		minutes, err := parseMinutes(parts[0])
		if err != nil {
			return 0, err
		}
		totalSeconds += minutes * 60
		return totalSeconds, nil
	case 3:
		// Parse hours, minutes and seconds (HH:MM:SS), for example:
		// The string "01:05:10" is converted into this array: ["01", "05", "10"]
		totalSeconds, err := parseSeconds(parts[2])
		if err != nil {
			return 0, err
		}
		minutes, err := parseMinutes(parts[1])
		if err != nil {
			return 0, err
		}
		totalSeconds += minutes * 60
		hours, err := parseHours(parts[0])
		if err != nil {
			return 0, err
		}
		totalSeconds += hours * 60 * 60
		return totalSeconds, nil
	default:
		return 0, invalidTimestampErr
	}
}

// SecondsToTimestamp converts a number of seconds into a timestamp string in
// MM:SS format, or HH:MM:SS when the value is one hour or greater.
func SecondsToTimestamp(second int) string {
	if second < 0 {
		return ""
	}

	hours := second / 3600
	minutes := (second % 3600) / 60
	seconds := second % 60

	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

// parseTimestampRangeParts parses the start and end parts of a timestamp range,
// returns their values in seconds, and validates that the start time is before
// the end time.
func parseTimestampRangeParts(parts []string, timestampRange string) (int, int, error) {
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("%w: %q: must contain two timestamps", ErrInvalidTimestampRange, timestampRange)
	}

	startSecond, err := TimestampToSeconds(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("%w: %q: %w", ErrInvalidTimestampRange, timestampRange, err)
	}

	endSecond, err := TimestampToSeconds(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("%w: %q: %w", ErrInvalidTimestampRange, timestampRange, err)
	}

	// Skip the start >= end validation when endSecond is EndSecond, since
	// the actual end of the video is not known yet and will be resolved later.
	if endSecond != EndSecond && startSecond >= endSecond {
		return 0, 0, fmt.Errorf("%w: %q: start timestamp must be before end timestamp", ErrInvalidTimestampRange, timestampRange)
	}

	return startSecond, endSecond, nil
}

// TimestampRangeToSeconds parses a timestamp range whose start is either
// the keyword start or a timestamp in MM:SS or HH:MM:SS format, and whose
// end is either the keyword end or a timestamp in MM:SS or HH:MM:SS format.
// It returns the start and end values in seconds and validates that the
// start time is before the end time.
func TimestampRangeToSeconds(timestampRange string) (int, int, error) {
	if !TimestampRangePattern.MatchString(timestampRange) {
		return 0, 0, fmt.Errorf("%w: %q", ErrInvalidTimestampRange, timestampRange)
	}
	parts := strings.Split(timestampRange, "-")
	return parseTimestampRangeParts(parts, timestampRange)
}
