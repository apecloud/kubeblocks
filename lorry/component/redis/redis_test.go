/*
Copyright 2021 The Dapr Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
