/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package redis

import (
	"fmt"
	"strconv"
	"time"
)

const (
	maxRetries             = "maxRetries"
	maxRetryBackoff        = "maxRetryBackoff"
	ttlInSeconds           = "ttlInSeconds"
	queryIndexes           = "queryIndexes"
	defaultBase            = 10
	defaultBitSize         = 0
	defaultMaxRetries      = 3
	defaultMaxRetryBackoff = time.Second * 2
)

type Metadata struct {
	MaxRetries      int
	MaxRetryBackoff time.Duration
	TTLInSeconds    *int
	QueryIndexes    string
}

func ParseRedisMetadata(properties map[string]string) (Metadata, error) {
	m := Metadata{}

	m.MaxRetries = defaultMaxRetries
	if val, ok := properties[maxRetries]; ok && val != "" {
		parsedVal, err := strconv.ParseInt(val, defaultBase, defaultBitSize)
		if err != nil {
			return m, fmt.Errorf("redis store error: can't parse maxRetries field: %s", err)
		}
		m.MaxRetries = int(parsedVal)
	}

	m.MaxRetryBackoff = defaultMaxRetryBackoff
	if val, ok := properties[maxRetryBackoff]; ok && val != "" {
		parsedVal, err := strconv.ParseInt(val, defaultBase, defaultBitSize)
		if err != nil {
			return m, fmt.Errorf("redis store error: can't parse maxRetryBackoff field: %s", err)
		}
		m.MaxRetryBackoff = time.Duration(parsedVal)
	}

	if val, ok := properties[ttlInSeconds]; ok && val != "" {
		parsedVal, err := strconv.ParseInt(val, defaultBase, defaultBitSize)
		if err != nil {
			return m, fmt.Errorf("redis store error: can't parse ttlInSeconds field: %s", err)
		}
		intVal := int(parsedVal)
		m.TTLInSeconds = &intVal
	} else {
		m.TTLInSeconds = nil
	}

	if val, ok := properties[queryIndexes]; ok && val != "" {
		m.QueryIndexes = val
	}
	return m, nil
}
