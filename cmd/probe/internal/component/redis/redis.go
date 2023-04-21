/*
Copyright (C) 2022 ApeCloud Co., Ltd

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
