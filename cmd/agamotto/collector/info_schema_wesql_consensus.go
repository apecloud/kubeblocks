/*
Copyright ApeCloud Inc.

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

package collector

import (
	"context"
	"database/sql"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/mysqld_exporter/collector"
)

const (
	// Exporter namespace.
	namespace = "mysql"

	// Subsystem.
	informationSchema = "info_schema"

	// SQL Query
	wesqlConsensusQuery = `
	SELECT CURRENT_TERM,COMMIT_INDEX,LAST_LOG_TERM,LAST_LOG_INDEX,ROLE,LAST_APPLY_INDEX
	FROM information_schema.WESQL_CLUSTER_LOCAL
	LIMIT 1;
	`
)

// Metric descriptors.
var (
	wesqlConsensusCurrentTermDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "wesql_consensus_current_term"),
		"Log term for the current instance.",
		[]string{"instance_role"}, nil,
	)
	wesqlConsensusCommitIndexDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "wesql_consensus_commit_index"),
		"Committed index for the current instance.",
		[]string{"instance_role"}, nil,
	)
	wesqlConsensusLastLogTermDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "wesql_consensus_last_log_term"),
		"Last synced log term for the current instance.",
		[]string{"instance_role"}, nil,
	)
	wesqlConsensusLastLogIndexDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "wesql_consensus_last_log_index"),
		"Last synced log index for the current instance.",
		[]string{"instance_role"}, nil,
	)
	wesqlConsensusLastApplyIndexDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "wesql_consensus_last_apply_index"),
		"Last applied log index for the current instance.",
		[]string{"instance_role"}, nil,
	)
)

// ScrapeWesqlConsensus collect metrics from information_schema.WESQL_CLUSTER_LOCAL
type ScrapeWesqlConsensus struct{}

// Name of the Scraper. Should be unique.
func (ScrapeWesqlConsensus) Name() string {
	return informationSchema + ".wesql_consensus"
}

// Help describes the role of the Scraper.
func (ScrapeWesqlConsensus) Help() string {
	return "Collect metrics from information_schema.WESQL_CLUSTER_LOCAL"
}

// Version of MySQL from which scraper is available.
func (ScrapeWesqlConsensus) Version() float64 {
	return 8.0
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeWesqlConsensus) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {
	rows, err := db.QueryContext(ctx, wesqlConsensusQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	var (
		currentTerm, commitIndex, lastLogTerm, lastLogIndex, lastApplyIndex int64
		instanceRole                                                        string
	)

	for rows.Next() {
		if err := rows.Scan(
			&currentTerm, &commitIndex, &lastLogTerm, &lastLogIndex, &instanceRole, &lastApplyIndex,
		); err != nil {
			return err
		}

		ch <- prometheus.MustNewConstMetric(
			wesqlConsensusCurrentTermDesc, prometheus.GaugeValue, float64(currentTerm), instanceRole,
		)
		ch <- prometheus.MustNewConstMetric(
			wesqlConsensusCommitIndexDesc, prometheus.GaugeValue, float64(commitIndex), instanceRole,
		)
		ch <- prometheus.MustNewConstMetric(
			wesqlConsensusLastLogTermDesc, prometheus.GaugeValue, float64(lastLogTerm), instanceRole,
		)
		ch <- prometheus.MustNewConstMetric(
			wesqlConsensusLastLogIndexDesc, prometheus.GaugeValue, float64(lastLogIndex), instanceRole,
		)
		ch <- prometheus.MustNewConstMetric(
			wesqlConsensusLastApplyIndexDesc, prometheus.GaugeValue, float64(lastApplyIndex), instanceRole,
		)
		break
	}
	return nil
}

// check interface
var _ collector.Scraper = ScrapeWesqlConsensus{}
