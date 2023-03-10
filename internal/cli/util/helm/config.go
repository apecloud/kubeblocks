/*
Copyright ApeCloud, Inc.

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

package helm

import (
	"os"

	"helm.sh/helm/v3/pkg/action"
)

type Config struct {
	namespace   string
	kubeConfig  string
	debug       bool
	kubeContext string
	logFn       action.DebugLog
	fake        bool
}

func NewConfig(namespace string, kubeConfig string, ctx string, debug bool) *Config {
	cfg := &Config{
		namespace:   namespace,
		debug:       debug,
		kubeConfig:  kubeConfig,
		kubeContext: ctx,
	}

	if debug {
		cfg.logFn = GetVerboseLog(os.Stdout)
	} else {
		cfg.logFn = GetQuiteLog()
	}
	return cfg
}

func NewFakeConfig(namespace string) *Config {
	cfg := NewConfig(namespace, "", "", false)
	cfg.fake = true
	return cfg
}

func (o *Config) SetNamespace(namespace string) {
	o.namespace = namespace
}

func (o *Config) Namespace() string {
	return o.namespace
}
