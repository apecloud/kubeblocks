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

type RetryConfig struct {
	// Enabled indicates whether to not retry sending batches in case of export failure.
	Enabled bool `json:"enabled"`

	// InitialInterval the time to wait after the first failure before retrying.
	InitialInterval time.Duration `json:"initial_interval"`

	// RandomizationFactor is a random factor used to calculate next backoffs
	// Randomized interval = RetryInterval * (1 ± RandomizationFactor)
	RandomizationFactor int64 `json:"randomization_factor"`

	// Multiplier is the value multiplied by the backoff interval bounds
	Multiplier int64 `json:"multiplier"`

	// MaxInterval is the upper bound on backoff interval. Once this value is reached the delay between
	// consecutive retries will always be `MaxInterval`.
	MaxInterval time.Duration `json:"max_interval"`

	// MaxElapsedTime is the maximum amount of time (including retries) spent trying to send a request/batch.
	// Once this value is reached, the data is discarded.
	MaxElapsedTime time.Duration `json:"max_elapsed_time"`
}

type QueueConfig struct {
	// Enabled indicates whether to not enqueue batches before sending to the consumerSender.
	Enabled bool `json:"enabled"`
	// NumConsumers is the number of consumers from the queue.
	NumConsumers int `json:"num_consumers"`
	// QueueSize is the maximum number of batches allowed in queue at a given time.
	QueueSize int `json:"queue_size"`
}
