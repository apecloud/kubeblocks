/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package configuration

import (
	"strconv"
	"strings"
	"time"
	"unicode"

	"cuelang.org/go/cue"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	k8sResourceAttr          = "k8sResource"
	attrQuantityValue        = "quantity"
	storeResourceAttr        = "storeResource"
	timeDurationResourceAttr = "timeDurationResource"
	// attrStorageValue         = "storage"
	// attrTimeDurationValue    = "timeDuration"
)

const (
	StoreUnit = 1024

	KByte = 1 * StoreUnit
	MByte = KByte * StoreUnit
	GByte = MByte * StoreUnit
	TByte = GByte * StoreUnit
	PByte = TByte * StoreUnit
	EByte = PByte * StoreUnit
	ZByte = EByte * StoreUnit
	YByte = ZByte * StoreUnit
)

const (
	Millisecond = time.Duration(1)
	Second      = 1000 * Millisecond
	Minute      = 60 * Second
	Hour        = 60 * Minute
	Day         = 24 * Hour
)

var bytesSizeTable = map[string]int64{
	"KB": KByte,
	"MB": MByte,
	"GB": GByte,
	"TB": TByte,
	"PB": PByte,
	"EB": EByte,
	// "ZB": ZByte,
	// "YB": YByte,
	"K": KByte,
	"M": MByte,
	"G": GByte,
	"T": TByte,
	"P": PByte,
	"E": EByte,
	// "Z":  ZByte,
	// "Y":  YByte,
}

var timeDurationTable = map[string]time.Duration{
	"ms":   Millisecond,
	"s":    Second,
	"min":  Minute,
	"m":    Minute,
	"h":    Hour,
	"hour": Hour,
	"d":    Day,
	"day":  Day,
}

func processCueIntegerExpansion(x cue.Value) (CueType, string) {
	attrs := x.Attributes(cue.FieldAttr)
	if len(attrs) == 0 {
		return IntType, ""
	}
	for _, attr := range attrs {
		switch attr.Name() {
		case k8sResourceAttr:
			return K8SQuantityType, ""
		case storeResourceAttr:
			return ClassicStorageType, attr.Contents()
		case timeDurationResourceAttr:
			return ClassicTimeDurationType, attr.Contents()
		}
	}
	return IntType, ""
}

func handleK8sQuantityType(s string) (int64, error) {
	quantity, err := resource.ParseQuantity(s)
	if err != nil {
		return 0, err
	}
	return quantity.Value(), nil
}

func parseDigitNumber(s string) int {
	lastDigit := 0
	for _, b := range s {
		if lastDigit == 0 && b == '-' {
			lastDigit++
			continue
		}
		if !unicode.IsDigit(b) {
			break
		}
		lastDigit++
	}
	return lastDigit
}

type cueExpandHandle func(string) (int64, error)

func handleCueExpandHelper(expand string, handle cueExpandHandle) cueExpandHandle {
	var baseUnit int64 = 0
	if expand != "" {
		baseUnit, _ = handle(expand)
	}
	return func(s string) (int64, error) {
		v, err := handle(s)
		if baseUnit > 0 {
			v /= baseUnit
		}
		return v, err
	}
}

func handleClassicStorageType(expand string) cueExpandHandle {
	return handleCueExpandHelper(expand, func(s string) (int64, error) {
		digitNumber := parseDigitNumber(s)
		if digitNumber == 0 {
			return 0, MakeError("failed to parse storage type[%s]", s)
		}
		iv, err := strconv.Atoi(s[:digitNumber])
		if err != nil {
			return 0, err
		}
		if digitNumber == len(s) {
			return int64(iv), nil
		}

		unit := strings.ToUpper(s[digitNumber:])
		if v, ok := bytesSizeTable[unit]; ok {
			return int64(iv) * v, nil
		}
		return 0, MakeError("failed to parse storage value[%s]", s)
	})
}

func handleClassicTimeDurationType(expand string) cueExpandHandle {
	return handleCueExpandHelper(expand, func(s string) (int64, error) {
		digitNumber := parseDigitNumber(s)
		if digitNumber == 0 {
			return 0, MakeError("failed to parse time duration type[%s]", s)
		}
		iv, err := strconv.Atoi(s[:digitNumber])
		if err != nil {
			return 0, err
		}
		if digitNumber == len(s) {
			return int64(iv), nil
		}

		unit := strings.ToLower(s[digitNumber:])
		if v, ok := timeDurationTable[unit]; ok {
			return int64(iv) * int64(v), nil
		}
		return 0, MakeError("failed to parse time duration value[%s]", s)
	})
}
