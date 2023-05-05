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

package kafka

import (
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strings"

	"github.com/Shopify/sarama"
)

const (
	// DefaultMaxBulkSubCount is the default max bulk count for kafka pubsub component
	// if the MaxBulkCountKey is not set in the metadata.
	DefaultMaxBulkSubCount = 80
	// DefaultMaxBulkSubAwaitDurationMs is the default max bulk await duration for kafka pubsub component
	// if the MaxBulkAwaitDurationKey is not set in the metadata.
	DefaultMaxBulkSubAwaitDurationMs = 10000
)

// asBase64String implements the `fmt.Stringer` interface in order to print
// `[]byte` as a base 64 encoded string.
// It is used above to log the message key. The call to `EncodeToString`
// only occurs for logs that are written based on the logging level.
type asBase64String []byte

func (s asBase64String) String() string {
	return base64.StdEncoding.EncodeToString(s)
}

func parseInitialOffset(value string) (initialOffset int64, err error) {
	switch strings.ToLower(value) {
	case "oldest":
		initialOffset = sarama.OffsetOldest
	case "newest":
		initialOffset = sarama.OffsetNewest
	default:
		return 0, fmt.Errorf("kafka error: invalid initialOffset: %s", value)
	}
	return initialOffset, err
}

// isValidPEM validates the provided input has PEM formatted block.
func isValidPEM(val string) bool {
	block, _ := pem.Decode([]byte(val))

	return block != nil
}

// TopicHandlerConfig is the map of topics and sruct containing handler and their config.
type TopicHandlerConfig map[string]SubscriptionHandlerConfig

// TopicList returns the list of topics
func (tbh TopicHandlerConfig) TopicList() []string {
	topics := make([]string, len(tbh))
	i := 0
	for topic := range tbh {
		topics[i] = topic
		i++
	}
	return topics
}
