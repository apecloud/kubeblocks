/*
Copyright © 2022 The OpenCli Authors

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

package provider

import (
	"strings"

	"helm.sh/helm/v3/pkg/repo"

	"jihulab.com/infracreate/dbaas-system/opencli/pkg/utils/helm"
)

type Provider interface {
	GetRepos() []repo.Entry
	GetBaseCharts(ns string) []helm.InstallOpts
	GetDBCharts(ns string, dbname string) []helm.InstallOpts
}

func NewProvider(e string, v string) Provider {
	switch buildEngineType(e) {
	case BitnamiMySQL:
		return &BitnamiMysql{
			serverVersion: v,
		}
	case MySQLOperator:
		return &MysqlOperator{
			serverVersion: v,
		}
	default:
		return nil
	}
}

func buildEngineType(e string) EngineType {
	if strings.Contains(e, "bitnami") && strings.Contains(e, "mysql") {
		return BitnamiMySQL
	}

	if strings.Contains(e, "mysql") {
		return MySQLOperator
	}

	return UnknownEngine
}
