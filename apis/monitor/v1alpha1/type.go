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

package v1alpha1

import "time"

// LogsSinkType defines the type for LogsExporterSink
// +enum
// +kubebuilder:validation:Enum={apeclouds3,loki}
type LogsSinkType string

const (
	S3SinkType   LogsSinkType = "apeclouds3"
	LokiSinkType LogsSinkType = "loki"
)

// MetricsSinkType defines the type for LogsExporterSink
// +enum
// +kubebuilder:validation:Enum={prometheus, prometheusremotewrite}
type MetricsSinkType string

const (
	PrometheusSinkType            MetricsSinkType = "prometheus"
	PrometheusRemoteWriteSinkType MetricsSinkType = "prometheusremotewrite"
)

type RetryPolicyOnFailure struct {
	// initialInterval the time to wait after the first failure before retrying.
	// +optional
	InitialInterval time.Duration `json:"initialInterval"`

	// randomizationFactor is a random factor used to calculate next backoffs
	// Randomized interval = RetryInterval * (1 ± RandomizationFactor)
	// +optional
	RandomizationFactor int64 `json:"randomizationFactor"`

	// multiplier is the value multiplied by the backoff interval bounds
	// +optional
	Multiplier int64 `json:"multiplier"`

	// maxInterval is the upper bound on backoff interval. Once this value is reached the delay between
	// consecutive retries will always be `MaxInterval`.
	// +optional
	MaxInterval time.Duration `json:"maxInterval"`

	// maxElapsedTime is the maximum amount of time (including retries) spent trying to send a request/batch.
	// Once this value is reached, the data is discarded.
	// +optional
	MaxElapsedTime time.Duration `json:"maxElapsedTime"`
}

type SinkQueueConfig struct {
	// numConsumers is the number of consumers from the queue.
	// +optional
	NumConsumers int `json:"numConsumers"`

	// queueSize is the maximum number of batches allowed in queue at a given time.
	// +optional
	QueueSize int `json:"queueSize"`
}

type (
	// Mode represents how the OTeld should be deployed (deployment vs. daemonset)
	// +kubebuilder:validation:Enum=daemonset;deployment;sidecar;statefulset
	Mode string
)

const (
	// ModeDaemonSet specifies that the OTeld should be deployed as a Kubernetes DaemonSet.
	ModeDaemonSet Mode = "daemonset"

	// ModeDeployment specifies that the OTeld should be deployed as a Kubernetes Deployment.
	ModeDeployment Mode = "deployment"

	// ModeSidecar specifies that the OTeld should be deployed as a sidecar to pods.
	ModeSidecar Mode = "sidecar"

	// ModeStatefulSet specifies that the OTeld should be deployed as a Kubernetes StatefulSet.
	ModeStatefulSet Mode = "statefulset"
)

type (
	// LogLevel represents the log level for the OTeld
	// +kubebuilder:validation:Enum=debug;info;warn;error
	LogLevel string
)

const (
	// LogLevelDebug specifies that the OTeld should log at debug level.
	LogLevelDebug LogLevel = "debug"

	// LogLevelInfo specifies that the OTeld should log at info level.
	LogLevelInfo LogLevel = "info"

	// LogLevelWarn specifies that the OTeld should log at warn level.
	LogLevelWarn LogLevel = "warn"

	// LogLevelError specifies that the OTeld should log at error level.
	LogLevelError LogLevel = "error"
)
