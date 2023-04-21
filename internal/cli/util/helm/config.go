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
