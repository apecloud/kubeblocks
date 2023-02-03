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

package collector

import (
	"context"
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestScrapeWesqlConsensus(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	mock.ExpectQuery(wesqlCheckQuery).WillReturnRows(sqlmock.NewRows([]string{""}).AddRow(1))

	columns := []string{"CURRENT_TERM", "COMMIT_INDEX", "LAST_LOG_TERM", "LAST_LOG_INDEX", "ROLE", "LAST_APPLY_INDEX"}
	rows := sqlmock.NewRows(columns).
		AddRow(1, 2, 3, 4, "Leader", 5)
	mock.ExpectQuery(wesqlConsensusQuery).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapeWesqlConsensus{}).Scrape(context.Background(), db, ch, log.NewNopLogger()); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	metricsExpected := []MetricResult{
		{labels: labelMap{"instance_role": "Leader"}, value: 1, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"instance_role": "Leader"}, value: 2, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"instance_role": "Leader"}, value: 3, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"instance_role": "Leader"}, value: 4, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"instance_role": "Leader"}, value: 5, metricType: dto.MetricType_GAUGE},
	}
	for _, expect := range metricsExpected {
		actual := readMetric(<-ch)
		if !reflect.DeepEqual(actual, expect) {
			t.Errorf("metric not equal, actual:%v, expected:%v", actual, expect)
		}
	}

	// Ensure all SQL queries were executed
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled exceptions: %s", err)
	}
}

func TestScrapeNonWesqlConsensus(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	mock.ExpectQuery(wesqlCheckQuery).WillReturnRows(sqlmock.NewRows([]string{""}))

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapeWesqlConsensus{}).Scrape(context.Background(), db, ch, log.NewNopLogger()); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	if _, ok := <-ch; ok {
		t.Errorf("non-wesql database should not report metrics")
	}

	// Ensure all SQL queries were executed
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled exceptions: %s", err)
	}
}
