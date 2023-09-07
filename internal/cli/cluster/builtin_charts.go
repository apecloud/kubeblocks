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

var _ chartConfigInterface = &embedConfig{}

func (e *embedConfig) register(subcmd ClusterType) error {
	if _, ok := ClusterTypeCharts[subcmd]; ok {
		panic(fmt.Sprintf("cluster type %s already registered", subcmd))
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
	//run `make generate` to generate this embed file
	//go:embed charts/apecloud-mysql-cluster.tgz
	mysqlChart embed.FS
	//go:embed charts/postgresql-cluster.tgz
	postgresqlChart embed.FS
	//go:embed charts/kafka-cluster.tgz
	kafkaChart embed.FS
	//go:embed charts/redis-cluster.tgz
	redisChart embed.FS
	//go:embed charts/neon-cluster.tgz
	neonChart embed.FS
	////go:embed charts/mongodb-cluster.tgz
	//mongodbChart embed.FS
)

// internal_chart registers embed chart

func init() {

	mysql := &embedConfig{
		chartFS: mysqlChart,
		name:    "apecloud-mysql-cluster.tgz",
		alias:   "",
	}
	mysql.register("mysql")

	postgresql := &embedConfig{
		chartFS: postgresqlChart,
		name:    "postgresql-cluster.tgz",
		alias:   "",
	}
	postgresql.register("postgresql")

	kafka := &embedConfig{
		chartFS: kafkaChart,
		name:    "kafka-cluster.tgz",
		alias:   "",
	}
	kafka.register("kafka")

	redis := &embedConfig{
		chartFS: redisChart,
		name:    "redis-cluster.tgz",
		alias:   "",
	}
	redis.register("redis")

	neon := &embedConfig{
		chartFS: neonChart,
		name:    "neon-cluster.tgz",
		alias:   "",
	}
	neon.register("neon")

	//mongodb := &embedConfig{
	//	chartFS: mongodbChart,
	//	name:    "mongodb-cluster.tgz",
	//	alias:   "",
	//}
	//mongodb.register("mongodb")

}
