/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package constant

const (
	ServiceKindPostgreSQL    = "postgresql"
	ServiceKindMongoDB       = "mongodb"
	ServiceKindClickHouse    = "clickhouse"
	ServiceKindZookeeper     = "zookeeper"
	ServiceKindElasticSearch = "elasticsearch"
)

// GetPostgreSQLAlias get postgresql alias
func GetPostgreSQLAlias() []string {
	return []string{
		"pg",
		"pgsql",
		"postgres",
		"postgresql",
	}
}

// GetMongoDBAlias get mongodb alias
func GetMongoDBAlias() []string {
	return []string{
		"mongo",
		"mongodb",
	}
}

// GetZookeeperAlias get zookeeper alias
func GetZookeeperAlias() []string {
	return []string{
		"zk",
		"zookeeper",
	}
}

// GetElasticSearchAlias get elasticsearch alias
func GetElasticSearchAlias() []string {
	return []string{
		"es",
		"elasticsearch",
	}
}

// GetClickHouseAlias get clickhouse alias
func GetClickHouseAlias() []string {
	return []string{
		"ch",
		"clickhouse",
	}
}
