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

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"

	"gopkg.in/yaml.v2"

	"github.com/dapr/cli/pkg/print"
)

// Options represents the application configuration parameters.
type Options struct {
	HTTPPort           int    `env:"DAPR_HTTP_PORT" arg:"dapr-http-port"`
	GRPCPort           int    `env:"DAPR_GRPC_PORT" arg:"dapr-grpc-port"`
	ConfigFile         string `arg:"config"`
	Protocol           string `arg:"app-protocol"`
	Arguments          []string
	LogLevel           string `arg:"log-level"`
	ComponentsPath     string `arg:"components-path"`
	MetricsPort        int    `env:"DAPR_METRICS_PORT" arg:"metrics-port"`
	MaxRequestBodySize int    `arg:"dapr-http-max-request-size"`
	HTTPReadBufferSize int    `arg:"dapr-http-read-buffer-size"`
	UnixDomainSocket   string `arg:"unix-domain-socket"`
	InternalGRPCPort   int    `arg:"dapr-internal-grpc-port"`
	EnableAppHealth    bool   `arg:"enable-app-health-check"`
	AppHealthPath      string `arg:"app-health-check-path"`
	AppHealthInterval  int    `arg:"app-health-probe-interval" ifneq:"0"`
	AppHealthTimeout   int    `arg:"app-health-probe-timeout" ifneq:"0"`
	AppHealthThreshold int    `arg:"app-health-threshold" ifneq:"0"`
	EnableAPILogging   bool   `arg:"enable-api-logging"`
}

func (options *Options) validate() error {
	if options.MaxRequestBodySize < 0 {
		options.MaxRequestBodySize = -1
	}

	if options.HTTPReadBufferSize < 0 {
		options.HTTPReadBufferSize = -1
	}

	return nil
}

func (options *Options) getArgs() []string {
	args := []string{}
	schema := reflect.ValueOf(*options)
	for i := 0; i < schema.NumField(); i++ {
		valueField := schema.Field(i).Interface()
		typeField := schema.Type().Field(i)
		key := typeField.Tag.Get("arg")
		if len(key) == 0 {
			continue
		}
		key = "--" + key

		ifneq, hasIfneq := typeField.Tag.Lookup("ifneq")

		switch valueField.(type) {
		case bool:
			if valueField == true {
				args = append(args, key)
			}
		default:
			value := fmt.Sprintf("%v", reflect.ValueOf(valueField))
			if len(value) != 0 && (!hasIfneq || value != ifneq) {
				args = append(args, key, value)
			}
		}
	}
	if options.ConfigFile != "" {
		sentryAddress := mtlsEndpoint(options.ConfigFile)
		if sentryAddress != "" {
			// mTLS is enabled locally, set it up.
			args = append(args, "--enable-mtls", "--sentry-address", sentryAddress)
		}
	}

	if print.IsJSONLogEnabled() {
		args = append(args, "--log-as-json")
	}

	return args
}

func (options *Options) getEnv() []string {
	env := []string{}
	schema := reflect.ValueOf(*options)
	for i := 0; i < schema.NumField(); i++ {
		valueField := schema.Field(i).Interface()
		typeField := schema.Type().Field(i)
		key := typeField.Tag.Get("env")
		if len(key) == 0 {
			continue
		}
		if value, ok := valueField.(int); ok && value <= 0 {
			// ignore unset numeric variables.
			continue
		}

		value := fmt.Sprintf("%v", reflect.ValueOf(valueField))
		env = append(env, fmt.Sprintf("%s=%v", key, value))
	}
	return env
}

// RunOutput represents the run output.
type RunOutput struct {
	DaprCMD      *exec.Cmd
	DaprErr      error
	DaprHTTPPort int
	DaprGRPCPort int
	AppID        string
	AppCMD       *exec.Cmd
	AppErr       error
}

func getDaprCommand(options *Options) (*exec.Cmd, error) {
	daprCMD := binaryFilePath(defaultDaprBinPath(), "daprd")
	args := options.getArgs()
	cmd := exec.Command(daprCMD, args...)
	return cmd, nil
}

func mtlsEndpoint(configFile string) string {
	if configFile == "" {
		return ""
	}

	b, err := os.ReadFile(configFile)
	if err != nil {
		return ""
	}

	var options mtlsConfig
	err = yaml.Unmarshal(b, &options)
	if err != nil {
		return ""
	}

	if options.Spec.MTLS.Enabled {
		return sentryDefaultAddress
	}
	return ""
}

func getAppCommand(options *Options) *exec.Cmd {
	argCount := len(options.Arguments)

	if argCount == 0 {
		return nil
	}
	command := options.Arguments[0]

	args := []string{}
	if argCount > 1 {
		args = options.Arguments[1:]
	}

	cmd := exec.Command(command, args...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, options.getEnv()...)

	return cmd
}

func Run(options *Options) (*RunOutput, error) {
	//nolint
	err := options.validate()
	if err != nil {
		return nil, err
	}

	daprCMD, err := getDaprCommand(options)
	if err != nil {
		return nil, err
	}

	//nolint
	var appCMD *exec.Cmd = getAppCommand(options)
	return &RunOutput{
		DaprCMD:      daprCMD,
		DaprErr:      nil,
		AppCMD:       appCMD,
		AppErr:       nil,
		AppID:        options.AppID,
		DaprHTTPPort: options.HTTPPort,
		DaprGRPCPort: options.GRPCPort,
	}, nil
}
