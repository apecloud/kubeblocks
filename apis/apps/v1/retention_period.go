/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package v1

import (
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// RetentionPeriod represents a duration in the format "1y2mo3w4d5h6m", where
// y=year, mo=month, w=week, d=day, h=hour, m=minute.
type RetentionPeriod string

// ToDuration converts the RetentionPeriod to time.Duration.
func (r RetentionPeriod) ToDuration() (time.Duration, error) {
	if len(r.String()) == 0 {
		return time.Duration(0), nil
	}

	minutes, err := r.toMinutes()
	if err != nil {
		return time.Duration(0), err
	}
	return time.Minute * time.Duration(minutes), nil
}

func (r RetentionPeriod) String() string {
	return string(r)
}

func (r RetentionPeriod) toMinutes() (int, error) {
	d, err := r.parseDuration()
	if err != nil {
		return 0, err
	}
	minutes := d.Minutes
	minutes += d.Hours * 60
	minutes += d.Days * 24 * 60
	minutes += d.Weeks * 7 * 24 * 60
	minutes += d.Months * 30 * 24 * 60
	minutes += d.Years * 365 * 24 * 60
	return minutes, nil
}

type duration struct {
	Minutes int
	Hours   int
	Days    int
	Weeks   int
	Months  int
	Years   int
}

var errInvalidDuration = errors.New("invalid duration provided")

// parseDuration parses a duration from a string. The format is `6y5m234d37h`
func (r RetentionPeriod) parseDuration() (duration, error) {
	var (
		d   duration
		num int
		err error
	)

	s := strings.TrimSpace(r.String())
	for s != "" {
		num, s, err = r.nextNumber(s)
		if err != nil {
			return duration{}, err
		}

		if len(s) == 0 {
			return duration{}, errInvalidDuration
		}

		if len(s) > 1 && s[0] == 'm' && s[1] == 'o' {
			d.Months = num
			s = s[2:]
			continue
		}

		switch s[0] {
		case 'y':
			d.Years = num
		case 'w':
			d.Weeks = num
		case 'd':
			d.Days = num
		case 'h':
			d.Hours = num
		case 'm':
			d.Minutes = num
		default:
			return duration{}, errInvalidDuration
		}
		s = s[1:]
	}
	return d, nil
}

func (r RetentionPeriod) nextNumber(input string) (num int, rest string, err error) {
	if len(input) == 0 {
		return 0, "", nil
	}

	var n string

	if input[0] == '-' {
		return 0, input, errInvalidDuration
	}

	for i, s := range input {
		if !unicode.IsNumber(s) {
			rest = input[i:]
			break
		}

		n += string(s)
	}

	if len(n) == 0 {
		return 0, input, errInvalidDuration
	}

	num, err = strconv.Atoi(n)
	if err != nil {
		return 0, input, err
	}

	return num, rest, nil
}
