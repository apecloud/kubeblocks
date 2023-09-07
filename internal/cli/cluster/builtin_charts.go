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
	//go:embed charts/neon-cluster.tgz
	neonChart embed.FS
	//go:embed charts/mongodb-cluster.tgz
	mongodbChart embed.FS
)

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
	if err := postgresql.register("mysql"); err != nil {
		fmt.Println(err.Error())
	}

	kafka := &embedConfig{
		chartFS: kafkaChart,
		name:    "kafka-cluster.tgz",
		alias:   "",
	}
	if err := kafka.register("mysql"); err != nil {
		fmt.Println(err.Error())
	}

	redis := &embedConfig{
		chartFS: redisChart,
		name:    "redis-cluster.tgz",
		alias:   "",
	}
	if err := redis.register("mysql"); err != nil {
		fmt.Println(err.Error())
	}

	neon := &embedConfig{
		chartFS: neonChart,
		name:    "neon-cluster.tgz",
		alias:   "",
	}
	if err := neon.register("mysql"); err != nil {
		fmt.Println(err.Error())
	}

	mongodb := &embedConfig{
		chartFS: mongodbChart,
		name:    "mongodb-cluster.tgz",
		alias:   "",
	}
	if err := mongodb.register("mysql"); err != nil {
		fmt.Println(err.Error())
	}

}
