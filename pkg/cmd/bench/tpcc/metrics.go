package tpcc

import "github.com/prometheus/client_golang/prometheus"

var (
	elapsedVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "tpc",
			Subsystem: "tpcc",
			Name:      "elapsed",
			Help:      "The real elapsed time per interval",
		}, []string{"op"},
	)
	sumVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "tpc",
			Subsystem: "tpcc",
			Name:      "sum",
			Help:      "The total latency per interval",
		}, []string{"op"},
	)
	countVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "tpc",
			Subsystem: "tpcc",
			Name:      "count",
			Help:      "The total count of transactions",
		}, []string{"op"},
	)
	opsVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "tpc",
			Subsystem: "tpcc",
			Name:      "ops",
			Help:      "The number of op per second",
		}, []string{"op"},
	)
	avgVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "tpc",
			Subsystem: "tpcc",
			Name:      "avg",
			Help:      "The avarge latency",
		}, []string{"op"},
	)
	p50Vec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "tpc",
			Subsystem: "tpcc",
			Name:      "p50",
			Help:      "P50 latency",
		}, []string{"op"},
	)
	p90Vec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "tpc",
			Subsystem: "tpcc",
			Name:      "p90",
			Help:      "P90 latency",
		}, []string{"op"},
	)
	p95Vec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "tpc",
			Subsystem: "tpcc",
			Name:      "p95",
			Help:      "P95 latency",
		}, []string{"op"},
	)
	p99Vec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "tpc",
			Subsystem: "tpcc",
			Name:      "p99",
			Help:      "P99 latency",
		}, []string{"op"},
	)
	p999Vec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "tpc",
			Subsystem: "tpcc",
			Name:      "p999",
			Help:      "p999 latency",
		}, []string{"op"},
	)
	maxVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "tpc",
			Subsystem: "tpcc",
			Name:      "max",
			Help:      "Max latency",
		}, []string{"op"},
	)
)

func init() {
	prometheus.MustRegister(elapsedVec, sumVec, countVec, opsVec, avgVec, p50Vec, p90Vec, p95Vec, p99Vec, p999Vec, maxVec)
}
