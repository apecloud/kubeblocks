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
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/mitchellh/mapstructure"

	"github.com/cenkalti/backoff/v4"
	"github.com/pkg/errors"
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

	return Decode(input, c)
}
func Decode(input interface{}, output interface{}) error {
	decoder, err := mapstructure.NewDecoder(
		&mapstructure.DecoderConfig{ //nolint: exhaustruct
			Result:     output,
			DecodeHook: decodeString,
		})
	if err != nil {
		return err
	}

	return decoder.Decode(input)
}

var (
	typeDuration      = reflect.TypeOf(time.Duration(5))             //nolint: gochecknoglobals
	typeTime          = reflect.TypeOf(time.Time{})                  //nolint: gochecknoglobals
	typeStringDecoder = reflect.TypeOf((*StringDecoder)(nil)).Elem() //nolint: gochecknoglobals
)

type StringDecoder interface {
	DecodeString(value string) error
}

//nolint:cyclop
func decodeString(f reflect.Type, t reflect.Type, data any) (any, error) {
	if t.Kind() == reflect.String && f.Kind() != reflect.String {
		return fmt.Sprintf("%v", data), nil
	}
	if f.Kind() == reflect.Ptr {
		f = f.Elem()
		data = reflect.ValueOf(data).Elem().Interface()
	}
	if f.Kind() != reflect.String {
		return data, nil
	}

	dataString, ok := data.(string)
	if !ok {
		return nil, errors.Errorf("expected string: got %s", reflect.TypeOf(data))
	}

	var result any
	var decoder StringDecoder

	if t.Implements(typeStringDecoder) {
		result = reflect.New(t.Elem()).Interface()
		decoder = result.(StringDecoder)
	} else if reflect.PtrTo(t).Implements(typeStringDecoder) {
		result = reflect.New(t).Interface()
		decoder = result.(StringDecoder)
	}

	if decoder != nil {
		if err := decoder.DecodeString(dataString); err != nil {
			if t.Kind() == reflect.Ptr {
				t = t.Elem()
			}

			return nil, errors.Errorf("invalid %s %q: %v", t.Name(), dataString, err)
		}

		return result, nil
	}

	switch t {
	case typeDuration:
		// Check for simple integer values and treat them
		// as milliseconds
		if val, err := strconv.Atoi(dataString); err == nil {
			return time.Duration(val) * time.Millisecond, nil
		}

		// Convert it by parsing
		d, err := time.ParseDuration(dataString)

		return d, invalidError(err, "duration", dataString)
	case typeTime:
		// Convert it by parsing
		t, err := time.Parse(time.RFC3339Nano, dataString)
		if err == nil {
			return t, nil
		}
		t, err = time.Parse(time.RFC3339, dataString)

		return t, invalidError(err, "time", dataString)
	}

	switch t.Kind() {
	case reflect.Uint:
		val, err := strconv.ParseUint(dataString, 10, 32)

		return uint(val), invalidError(err, "uint", dataString)
	case reflect.Uint64:
		val, err := strconv.ParseUint(dataString, 10, 64)

		return val, invalidError(err, "uint64", dataString)
	case reflect.Uint32:
		val, err := strconv.ParseUint(dataString, 10, 32)

		return uint32(val), invalidError(err, "uint32", dataString)
	case reflect.Uint16:
		val, err := strconv.ParseUint(dataString, 10, 16)

		return uint16(val), invalidError(err, "uint16", dataString)
	case reflect.Uint8:
		val, err := strconv.ParseUint(dataString, 10, 8)

		return uint8(val), invalidError(err, "uint8", dataString)

	case reflect.Int:
		val, err := strconv.Atoi(dataString)

		return val, invalidError(err, "int", dataString)
	case reflect.Int64:
		val, err := strconv.ParseInt(dataString, 10, 64)

		return val, invalidError(err, "int64", dataString)
	case reflect.Int32:
		val, err := strconv.ParseInt(dataString, 10, 32)

		return int32(val), invalidError(err, "int32", dataString)
	case reflect.Int16:
		val, err := strconv.ParseInt(dataString, 10, 16)

		return int16(val), invalidError(err, "int16", dataString)
	case reflect.Int8:
		val, err := strconv.ParseInt(dataString, 10, 8)

		return int8(val), invalidError(err, "int8", dataString)

	case reflect.Float32:
		val, err := strconv.ParseFloat(dataString, 32)

		return float32(val), invalidError(err, "float32", dataString)
	case reflect.Float64:
		val, err := strconv.ParseFloat(dataString, 64)

		return val, invalidError(err, "float64", dataString)

	case reflect.Bool:
		val, err := strconv.ParseBool(dataString)

		return val, invalidError(err, "bool", dataString)

	default:
		return data, nil
	}
}
func invalidError(err error, msg, value string) error {
	if err == nil {
		return nil
	}

	return errors.Errorf("invalid %s %q", msg, value)
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
