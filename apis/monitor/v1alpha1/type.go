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
// +kubebuilder:validation:Enum={S3SinkType,LokiSinkType}
type LogsSinkType string

const (
	S3SinkType   LogsSinkType = "apeclouds3"
	LokiSinkType LogsSinkType = "loki"
)

// MetricsSinkType defines the type for LogsExporterSink
// +enum
// +kubebuilder:validation:Enum={S3SinkType,LokiSinkType}
type MetricsSinkType string

const (
	prometheusSinkType LogsSinkType = "proemtheus"
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
