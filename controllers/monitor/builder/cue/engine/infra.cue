// Copyright (C) 2022-2023 ApeCloud Co., Ltd
//
// This file is part of KubeBlocks project
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

parameters: {
	collection_interval: *"15s" | string
	initial_delay:       *"10s" | string
	timeout:             *"5s" | string
}

output: {
	collection_interval: parameters.collection_interval
	initial_delay:       parameters.initial_delay
	timeout:             parameters.timeout
	default_scrape_configs: {
		"apecloud-mysql/mysql/mysql/mysql": {
			enabled_metrics: true
			enabled_logs:    true
			metrics_collector: {
				collection_interval: "30s"
				enable_metrics: ["global_status", "global_variables", "slave_status"]
			}
			logs_collector: {
				errorlog:
					include: ["/data/mysql/log/mysqld-error.log"]
				slow:
					include: ["/data/mysql/log/mysqld-slowquery.log"]
			}
		}

		"apecloud-mysql/mysql/vttablet/prometheus": {
			enabled_metrics: true
			enabled_logs:    false
			metrics_collector:
				collection_interval: "30s"
		}

		"apecloud-mysql/vtgate/vtgate/prometheus": {
			enabled_metrics: true
			enabled_logs:    true
			logs_collector: {
				errorlog:
					include: ["/vtdataroot/vtgate.ERROR", "/vtdataroot/vtgate.WARNING", "/vtdataroot/vtgate.INFO"]
				queryLog:
					include: ["/vtdataroot/vtgate_querylog.txt"]
			}
			metrics_collector:
				collection_interval: "30s"
		}

		"apecloud-mysql/vtcontroller/vtconsensus/vtcontroller": {
			enabled_metrics: false
			enabled_logs:    true
			logs_collector:
				errorlog:
					include: ["/vtdataroot/vtcontroller/vtcontroller.ERROR", "/vtdataroot/vtcontroller/vtcontroller.WARNING", "/vtdataroot/vtcontroller/vtcontroller.INFO"]
		}

		"postgresql/postgresql/postgresql/postgresql": {
			enabled_metrics: true
			enabled_logs:    true
			metrics_collector:
				collection_interval: "30s"
			logs_collector:
				runninglog:
					include: ["/home/postgres/pgdata/pgroot/data/log/postgresql-*"]
		}

		"redis/redis/redis/redis": {
			enabled_metrics: true
			enabled_logs:    true
			metrics_collector:
				collection_interval: "30s"
			logs_collector:
				runninglog:
					include: ["/data/running.log"]
		}

		"mongodb/mongodb/mongodb/mongodb": {
			enabled_metrics: true
			enabled_logs:    true
			metrics_collector:
				collection_interval: "30s"
			logs_collector:
				runninglog:
					include: ["/data/mongodb/logs/mongodb.log*"]
		}

		"pulsar/pulsar-broker/broker/prometheus": {
			enabled_metrics: true
			enabled_logs:    false
			metrics_collector:
				collection_interval: "30s"
		}

		"pulsar/pulsar-proxy/proxy/prometheus": {
			enabled_metrics: true
			enabled_logs:    false
			metrics_collector:
				collection_interval: "30s"
		}

		"pulsar/bookies/bookies/prometheus": {
			enabled_metrics: true
			enabled_logs:    false
			metrics_collector:
				collection_interval: "30s"
		}

		"pulsar/bookies-recovery/bookies-recovery/prometheus": {
			enabled_metrics: true
			enabled_logs:    false
			metrics_collector:
				collection_interval: "30s"
		}

		"pulsar/zookeeper/zookeeper/prometheus": {
			enabled_metrics: true
			enabled_logs:    false
			metrics_collector:
				collection_interval: "30s"
		}

		"qdrant/qdrant/qdrant/prometheus": {
			enabled_metrics: true
			enabled_logs:    false
			metrics_collector:
				collection_interval: "30s"
		}
	}
}
