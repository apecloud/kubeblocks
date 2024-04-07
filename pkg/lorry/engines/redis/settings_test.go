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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	host                  = "redisHost"
	password              = "redisPassword"
	username              = "redisUsername"
	db                    = "redisDB"
	redisType             = "redisType"
	redisMaxRetries       = "redisMaxRetries"
	redisMinRetryInterval = "redisMinRetryInterval"
	redisMaxRetryInterval = "redisMaxRetryInterval"
	dialTimeout           = "dialTimeout"
	readTimeout           = "readTimeout"
	writeTimeout          = "writeTimeout"
	poolSize              = "poolSize"
	minIdleConns          = "minIdleConns"
	poolTimeout           = "poolTimeout"
	idleTimeout           = "idleTimeout"
	idleCheckFrequency    = "idleCheckFrequency"
	maxConnAge            = "maxConnAge"
	enableTLS             = "enableTLS"
	failover              = "failover"
	sentinelMasterName    = "sentinelMasterName"
)

func getFakeProperties() map[string]string {
	return map[string]string{
		host:                  "fake.redis.com",
		password:              "fakePassword",
		username:              "fakeUsername",
		redisType:             "node",
		enableTLS:             "true",
		dialTimeout:           "5s",
		readTimeout:           "5s",
		writeTimeout:          "50000",
		poolSize:              "20",
		maxConnAge:            "200s",
		db:                    "1",
		redisMaxRetries:       "1",
		redisMinRetryInterval: "8ms",
		redisMaxRetryInterval: "1s",
		minIdleConns:          "1",
		poolTimeout:           "1s",
		idleTimeout:           "1s",
		idleCheckFrequency:    "1s",
		failover:              "true",
		sentinelMasterName:    "master",
	}
}

func TestParseRedisMetadata(t *testing.T) {
	t.Run("ClientMetadata is correct", func(t *testing.T) {
		fakeProperties := getFakeProperties()

		// act
		m := &Settings{}
		err := m.Decode(fakeProperties)

		// assert
		assert.NoError(t, err)
		assert.Equal(t, fakeProperties[host], m.Host)
		assert.Equal(t, fakeProperties[password], m.Password)
		assert.Equal(t, fakeProperties[username], m.Username)
		assert.Equal(t, fakeProperties[redisType], m.RedisType)
		assert.Equal(t, true, m.EnableTLS)
		assert.Equal(t, 5*time.Second, time.Duration(m.DialTimeout))
		assert.Equal(t, 5*time.Second, time.Duration(m.ReadTimeout))
		assert.Equal(t, 50000*time.Millisecond, time.Duration(m.WriteTimeout))
		assert.Equal(t, 20, m.PoolSize)
		assert.Equal(t, 200*time.Second, time.Duration(m.MaxConnAge))
		assert.Equal(t, 1, m.DB)
		assert.Equal(t, 1, m.RedisMaxRetries)
		assert.Equal(t, 8*time.Millisecond, time.Duration(m.RedisMinRetryInterval))
		assert.Equal(t, 1*time.Second, time.Duration(m.RedisMaxRetryInterval))
		assert.Equal(t, 1, m.MinIdleConns)
		assert.Equal(t, 1*time.Second, time.Duration(m.PoolTimeout))
		assert.Equal(t, 1*time.Second, time.Duration(m.IdleTimeout))
		assert.Equal(t, 1*time.Second, time.Duration(m.IdleCheckFrequency))
		assert.Equal(t, true, m.Failover)
		assert.Equal(t, "master", m.SentinelMasterName)
	})

	t.Run("host is not given", func(t *testing.T) {
		fakeProperties := getFakeProperties()

		fakeProperties[host] = ""

		// act
		m := &Settings{}
		err := m.Decode(fakeProperties)

		// assert
		// m.Decode dose not return error when host is ""
		assert.Error(t, errors.New("redis streams error: missing host address"), err)
		assert.Empty(t, m.Host)
	})

	t.Run("check values can be set as -1", func(t *testing.T) {
		fakeProperties := getFakeProperties()

		fakeProperties[readTimeout] = "-1"
		fakeProperties[idleTimeout] = "-1"
		fakeProperties[idleCheckFrequency] = "-1"
		fakeProperties[redisMaxRetryInterval] = "-1"
		fakeProperties[redisMinRetryInterval] = "-1"

		// act
		m := &Settings{}
		err := m.Decode(fakeProperties)
		// assert
		assert.NoError(t, err)
		assert.True(t, m.ReadTimeout == -1)
		assert.True(t, m.IdleTimeout == -1)
		assert.True(t, m.IdleCheckFrequency == -1)
		assert.True(t, m.RedisMaxRetryInterval == -1)
		assert.True(t, m.RedisMinRetryInterval == -1)
	})
}
