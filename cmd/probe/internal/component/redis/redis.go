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
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	ClusterType = "cluster"
	NodeType    = "node"
)

func ParseClientFromProperties(properties map[string]string, defaultSettings *Settings) (client redis.UniversalClient, settings *Settings, err error) {
	if defaultSettings == nil {
		settings = &Settings{}
	} else {
		settings = defaultSettings
	}
	err = settings.Decode(properties)
	if err != nil {
		return nil, nil, fmt.Errorf("redis client configuration error: %w", err)
	}
	if settings.Failover {
		return newFailoverClient(settings), settings, nil
	}

	return newClient(settings), settings, nil
}

func newFailoverClient(s *Settings) redis.UniversalClient {
	if s == nil {
		return nil
	}
	opts := &redis.FailoverOptions{
		DB:              s.DB,
		MasterName:      s.SentinelMasterName,
		SentinelAddrs:   []string{s.Host},
		Password:        s.Password,
		Username:        s.Username,
		MaxRetries:      s.RedisMaxRetries,
		MaxRetryBackoff: time.Duration(s.RedisMaxRetryInterval),
		MinRetryBackoff: time.Duration(s.RedisMinRetryInterval),
		DialTimeout:     time.Duration(s.DialTimeout),
		ReadTimeout:     time.Duration(s.ReadTimeout),
		WriteTimeout:    time.Duration(s.WriteTimeout),
		PoolSize:        s.PoolSize,
		MinIdleConns:    s.MinIdleConns,
		PoolTimeout:     time.Duration(s.PoolTimeout),
	}

	/* #nosec */
	if s.EnableTLS {
		opts.TLSConfig = &tls.Config{
			InsecureSkipVerify: s.EnableTLS,
		}
	}

	if s.RedisType == ClusterType {
		opts.SentinelAddrs = strings.Split(s.Host, ",")

		return redis.NewFailoverClusterClient(opts)
	}

	return redis.NewFailoverClient(opts)
}

func newClient(s *Settings) redis.UniversalClient {
	if s == nil {
		return nil
	}
	if s.RedisType == ClusterType {
		options := &redis.ClusterOptions{
			Addrs:           strings.Split(s.Host, ","),
			Password:        s.Password,
			Username:        s.Username,
			MaxRetries:      s.RedisMaxRetries,
			MaxRetryBackoff: time.Duration(s.RedisMaxRetryInterval),
			MinRetryBackoff: time.Duration(s.RedisMinRetryInterval),
			DialTimeout:     time.Duration(s.DialTimeout),
			ReadTimeout:     time.Duration(s.ReadTimeout),
			WriteTimeout:    time.Duration(s.WriteTimeout),
			PoolSize:        s.PoolSize,
			MinIdleConns:    s.MinIdleConns,
			PoolTimeout:     time.Duration(s.PoolTimeout),
		}
		/* #nosec */
		if s.EnableTLS {
			options.TLSConfig = &tls.Config{
				InsecureSkipVerify: s.EnableTLS,
			}
		}

		return redis.NewClusterClient(options)
	}

	options := &redis.Options{
		Addr:            s.Host,
		Password:        s.Password,
		Username:        s.Username,
		DB:              s.DB,
		MaxRetries:      s.RedisMaxRetries,
		MaxRetryBackoff: time.Duration(s.RedisMaxRetryInterval),
		MinRetryBackoff: time.Duration(s.RedisMinRetryInterval),
		DialTimeout:     time.Duration(s.DialTimeout),
		ReadTimeout:     time.Duration(s.ReadTimeout),
		WriteTimeout:    time.Duration(s.WriteTimeout),
		PoolSize:        s.PoolSize,
		MinIdleConns:    s.MinIdleConns,
		PoolTimeout:     time.Duration(s.PoolTimeout),
	}

	/* #nosec */
	if s.EnableTLS {
		options.TLSConfig = &tls.Config{
			InsecureSkipVerify: s.EnableTLS,
		}
	}

	return redis.NewClient(options)
}
