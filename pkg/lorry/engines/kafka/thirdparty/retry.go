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

package thirdparty

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/cenkalti/backoff/v4"
	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/pkg/lorry/util/config"
)

// PolicyType denotes if the back off delay should be constant or exponential.
type PolicyType int

const (
	// PolicyConstant is a backoff policy that always returns the same backoff delay.
	PolicyConstant PolicyType = iota
	// PolicyExponential is a backoff implementation that increases the backoff period
	// for each retry attempt using a randomization function that grows exponentially.
	PolicyExponential
)

// Config encapsulates the back off policy configuration.
type Config struct {
	Policy PolicyType `mapstructure:"policy"`

	// Constant back off
	Duration time.Duration `mapstructure:"duration"`

	// Exponential back off
	InitialInterval     time.Duration `mapstructure:"initialInterval"`
	RandomizationFactor float32       `mapstructure:"randomizationFactor"`
	Multiplier          float32       `mapstructure:"multiplier"`
	MaxInterval         time.Duration `mapstructure:"maxInterval"`
	MaxElapsedTime      time.Duration `mapstructure:"maxElapsedTime"`

	// Additional options
	MaxRetries int64 `mapstructure:"maxRetries"`
}

// String implements fmt.Stringer and is used for debugging.
func (c Config) String() string {
	return fmt.Sprintf(
		"policy='%s' duration='%v' initialInterval='%v' randomizationFactor='%f' multiplier='%f' maxInterval='%v' maxElapsedTime='%v' maxRetries='%d'",
		c.Policy, c.Duration, c.InitialInterval, c.RandomizationFactor, c.Multiplier, c.MaxInterval, c.MaxElapsedTime, c.MaxRetries,
	)
}

// DefaultConfig represents the default configuration for a `Config`.
func DefaultConfig() Config {
	return Config{
		Policy:              PolicyConstant,
		Duration:            5 * time.Second,
		InitialInterval:     backoff.DefaultInitialInterval,
		RandomizationFactor: backoff.DefaultRandomizationFactor,
		Multiplier:          backoff.DefaultMultiplier,
		MaxInterval:         backoff.DefaultMaxInterval,
		MaxElapsedTime:      backoff.DefaultMaxElapsedTime,
		MaxRetries:          -1,
	}
}

// NewBackOff returns a BackOff instance for use with `NotifyRecover`
// or `backoff.RetryNotify` directly. The instance will not stop due to
// context cancellation. To support cancellation (recommended), use
// `NewBackOffWithContext`.
//
// Since the underlying backoff implementations are not always thread safe,
// `NewBackOff` or `NewBackOffWithContext` should be called each time
// `RetryNotifyRecover` or `backoff.RetryNotify` is used.
func (c *Config) NewBackOff() backoff.BackOff {
	var b backoff.BackOff
	switch c.Policy {
	case PolicyConstant:
		b = backoff.NewConstantBackOff(c.Duration)
	case PolicyExponential:
		eb := backoff.NewExponentialBackOff()
		eb.InitialInterval = c.InitialInterval
		eb.RandomizationFactor = float64(c.RandomizationFactor)
		eb.Multiplier = float64(c.Multiplier)
		eb.MaxInterval = c.MaxInterval
		eb.MaxElapsedTime = c.MaxElapsedTime
		b = eb
	}

	if c.MaxRetries >= 0 {
		b = backoff.WithMaxRetries(b, uint64(c.MaxRetries))
	}

	return b
}

// NewBackOffWithContext returns a BackOff instance for use with `RetryNotifyRecover`
// or `backoff.RetryNotify` directly. The provided context is used to cancel retries
// if it is canceled.
//
// Since the underlying backoff implementations are not always thread safe,
// `NewBackOff` or `NewBackOffWithContext` should be called each time
// `RetryNotifyRecover` or `backoff.RetryNotify` is used.
func (c *Config) NewBackOffWithContext(ctx context.Context) backoff.BackOff {
	b := c.NewBackOff()

	return backoff.WithContext(b, ctx)
}

// DecodeConfigWithPrefix decodes a Go struct into a `Config`.
func DecodeConfigWithPrefix(c *Config, input interface{}, prefix string) error {
	input, err := PrefixedBy(input, prefix)
	if err != nil {
		return err
	}

	return DecodeConfig(c, input)
}

// DecodeConfig decodes a Go struct into a `Config`.
func DecodeConfig(c *Config, input interface{}) error {
	// Use the deefault config if `c` is empty/zero value.
	var emptyConfig Config
	if *c == emptyConfig {
		*c = DefaultConfig()
	}

	return config.Decode(input, c)
}

// NotifyRecover is a wrapper around backoff.RetryNotify that adds another callback for when an operation
// previously failed but has since recovered. The main purpose of this wrapper is to call `notify` only when
// the operations fails the first time and `recovered` when it finally succeeds. This can be helpful in limiting
// log messages to only the events that operators need to be alerted on.
func NotifyRecover(operation backoff.Operation, b backoff.BackOff, notify backoff.Notify, recovered func()) error {
	notified := atomic.Bool{}

	return backoff.RetryNotify(func() error {
		err := operation()

		if err == nil && notified.CompareAndSwap(true, false) {
			recovered()
		}

		return err
	}, b, func(err error, d time.Duration) {
		if notified.CompareAndSwap(false, true) {
			notify(err, d)
		}
	})
}

// NotifyRecoverWithData is a variant of NotifyRecover that also returns data in addition to an error.
func NotifyRecoverWithData[T any](operation backoff.OperationWithData[T], b backoff.BackOff, notify backoff.Notify, recovered func()) (T, error) {
	notified := atomic.Bool{}

	return backoff.RetryNotifyWithData(func() (T, error) {
		res, err := operation()

		if err == nil && notified.CompareAndSwap(true, false) {
			recovered()
		}

		return res, err
	}, b, func(err error, d time.Duration) {
		if notified.CompareAndSwap(false, true) {
			notify(err, d)
		}
	})
}

// DecodeString handles converting a string value to `p`.
func (p *PolicyType) DecodeString(value string) error {
	switch strings.ToLower(value) {
	case "constant":
		*p = PolicyConstant
	case "exponential":
		*p = PolicyExponential
	default:
		return errors.Errorf("unexpected back off policy type: %s", value)
	}
	return nil
}

// String implements fmt.Stringer and is used for debugging.
func (p PolicyType) String() string {
	switch p {
	case PolicyConstant:
		return "constant"
	case PolicyExponential:
		return "exponential"
	default:
		return ""
	}
}

func PrefixedBy(input interface{}, prefix string) (interface{}, error) {
	normalized, err := Normalize(input)
	if err != nil {
		// The only error that can come from normalize is if
		// input is a map[interface{}]interface{} and contains
		// a key that is not a string.
		return input, err
	}
	input = normalized

	if inputMap, ok := input.(map[string]interface{}); ok {
		converted := make(map[string]interface{}, len(inputMap))
		for k, v := range inputMap {
			if strings.HasPrefix(k, prefix) {
				key := uncapitalize(strings.TrimPrefix(k, prefix))
				converted[key] = v
			}
		}

		return converted, nil
	} else if inputMap, ok := input.(map[string]string); ok {
		converted := make(map[string]string, len(inputMap))
		for k, v := range inputMap {
			if strings.HasPrefix(k, prefix) {
				key := uncapitalize(strings.TrimPrefix(k, prefix))
				converted[key] = v
			}
		}

		return converted, nil
	}

	return input, nil
}

// uncapitalize initial capital letters in `str`.
func uncapitalize(str string) string {
	if len(str) == 0 {
		return str
	}

	vv := []rune(str) // Introduced later
	vv[0] = unicode.ToLower(vv[0])

	return string(vv)
}

//nolint:cyclop
func Normalize(i interface{}) (interface{}, error) {
	var err error
	switch x := i.(type) {
	case map[interface{}]interface{}:
		m2 := map[string]interface{}{}
		for k, v := range x {
			if strKey, ok := k.(string); ok {
				if m2[strKey], err = Normalize(v); err != nil {
					return nil, err
				}
			} else {
				return nil, fmt.Errorf("error parsing config field: %v", k)
			}
		}

		return m2, nil
	case map[string]interface{}:
		m2 := map[string]interface{}{}
		for k, v := range x {
			if m2[k], err = Normalize(v); err != nil {
				return nil, err
			}
		}

		return m2, nil
	case []interface{}:
		for i, v := range x {
			if x[i], err = Normalize(v); err != nil {
				return nil, err
			}
		}
	}

	return i, nil
}
