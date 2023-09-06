package register

import "embed"

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

	mongodb := &embedConfig{
		chartFS: mongodbChart,
		name:    "mongodb-cluster.tgz",
		alias:   "",
	}
	mongodb.register("mongodb")

}
