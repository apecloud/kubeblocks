/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package cluster

import (
	"embed"
	"fmt"
	"io"
)

// embedConfig is the interface for the go embed chart
type embedConfig struct {
	chartFS embed.FS
	// chart file name, include the extension
	name string
	// chart alias, this alias will be used as the command alias
	alias string
}

var _ chartLoader = &embedConfig{}

func (e *embedConfig) register(subcmd ClusterType) error {
	if _, ok := ClusterTypeCharts[subcmd]; ok {
		return fmt.Errorf("cluster type %s already registered", subcmd)
	}
	ClusterTypeCharts[subcmd] = e
	return nil
}

func (e *embedConfig) getAlias() string {
	return e.alias
}

func (e *embedConfig) loadChart() (io.ReadCloser, error) {
	return e.chartFS.Open(fmt.Sprintf("charts/%s", e.name))
}

func (e *embedConfig) getChartFileName() string {
	return e.name
}

var (
	// run `make generate` to generate this embed file
	//go:embed charts/apecloud-mysql-cluster.tgz
	mysqlChart embed.FS
	//go:embed charts/postgresql-cluster.tgz
	postgresqlChart embed.FS
	//go:embed charts/kafka-cluster.tgz
	kafkaChart embed.FS
	//go:embed charts/redis-cluster.tgz
	redisChart embed.FS
	//go:embed charts/mongodb-cluster.tgz
	mongodbChart embed.FS
	//go:embed charts/llm-cluster.tgz
	llmChart embed.FS
	//go:embed charts/xinference-cluster.tgz
	xinferenceChart embed.FS
)

func IsbuiltinCharts(chart string) bool {
	return chart == "mysql" || chart == "postgresql" || chart == "kafka" || chart == "redis" || chart == "mongodb"
}

// internal_chart registers embed chart

func init() {

	mysql := &embedConfig{
		chartFS: mysqlChart,
		name:    "apecloud-mysql-cluster.tgz",
		alias:   "",
	}
	if err := mysql.register("mysql"); err != nil {
		fmt.Println(err.Error())
	}

	postgresql := &embedConfig{
		chartFS: postgresqlChart,
		name:    "postgresql-cluster.tgz",
		alias:   "",
	}
	if err := postgresql.register("postgresql"); err != nil {
		fmt.Println(err.Error())
	}

	kafka := &embedConfig{
		chartFS: kafkaChart,
		name:    "kafka-cluster.tgz",
		alias:   "",
	}
	if err := kafka.register("kafka"); err != nil {
		fmt.Println(err.Error())
	}

	redis := &embedConfig{
		chartFS: redisChart,
		name:    "redis-cluster.tgz",
		alias:   "",
	}
	if err := redis.register("redis"); err != nil {
		fmt.Println(err.Error())
	}

	mongodb := &embedConfig{
		chartFS: mongodbChart,
		name:    "mongodb-cluster.tgz",
		alias:   "",
	}
	if err := mongodb.register("mongodb"); err != nil {
		fmt.Println(err.Error())
	}

	llm := &embedConfig{
		chartFS: llmChart,
		name:    "llm-cluster.tgz",
		alias:   "",
	}
	if err := llm.register("llm"); err != nil {
		fmt.Println(err.Error())
	}

	xinference := &embedConfig{
		chartFS: xinferenceChart,
		name:    "xinference-cluster.tgz",
		alias:   "",
	}
	if err := xinference.register("xinference"); err != nil {
		fmt.Println(err.Error())
	}
}
